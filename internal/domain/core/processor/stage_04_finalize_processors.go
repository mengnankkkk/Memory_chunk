package processor

import (
	"context"
	"strconv"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/components"
	"context-refiner/internal/domain/core/repository"
)

type PrefixCacheProcessor struct {
	counter core.TokenCounter
	store   repository.PrefixCacheRepository
	policy  core.PrefixCachePolicy
}

type AssembleProcessor struct {
	counter         core.TokenCounter
	promptComponent *components.PromptComponent
	ragNormalizer   *components.RAGNormalizer
}

func NewPrefixCacheProcessor(counter core.TokenCounter, store repository.PrefixCacheRepository, policy core.PrefixCachePolicy) *PrefixCacheProcessor {
	return &PrefixCacheProcessor{
		counter: counter,
		store:   store,
		policy:  policy,
	}
}

func NewAssembleProcessor(counter core.TokenCounter) *AssembleProcessor {
	return &AssembleProcessor{
		counter:         counter,
		promptComponent: components.NewPromptComponent(),
		ragNormalizer:   components.NewRAGNormalizer(),
	}
}

func (p *PrefixCacheProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "prefix_cache",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *AssembleProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "assemble",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *PrefixCacheProcessor) Process(ctx context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	identity := core.BuildPrefixCacheIdentity(updated, p.counter)
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]string)
	}
	updated.Metadata["normalization_version"] = identity.NormalizationVersion
	updated.Metadata["combined_prefix_hash"] = identity.CombinedPrefixHash
	updated.Metadata["system_prefix_hash"] = identity.SystemPrefixHash
	updated.Metadata["memory_prefix_hash"] = identity.MemoryPrefixHash
	updated.Metadata["rag_prefix_hash"] = identity.RAGPrefixHash
	updated.Metadata["system_prefix_tokens"] = strconv.Itoa(identity.SystemPrefixTokens)
	updated.Metadata["memory_prefix_tokens"] = strconv.Itoa(identity.MemoryPrefixTokens)
	updated.Metadata["rag_prefix_tokens"] = strconv.Itoa(identity.RAGPrefixTokens)
	namespace := p.buildNamespace(updated, identity)
	updated.Metadata["prefix_cache_namespace"] = namespace
	updated.Metadata["prefix_cache_tenant"] = p.resolveTenant(updated)
	updated.Metadata["prefix_cache_segment_count"] = strconv.Itoa(core.StableSegmentCount(identity))
	if identity.StablePrefixPrompt == "" {
		updated.Metadata["prefix_cache_lookup"] = "skipped"
		updated.Metadata["prefix_cache_admission"] = "skipped"
		updated.Metadata["prefix_cache_miss_reason"] = "empty"
		updated.Metadata["prefix_cache_segment_reason"] = "empty_prefix"
		applyPredictionMetadata(updated.Metadata, identity)
		return updated, core.ProcessResult{
			Details: map[string]string{
				"prefix_cache_lookup":      "skipped",
				"prefix_cache_miss_reason": "empty",
			},
		}, nil
	}
	if identity.StablePrefixTokens > 0 && identity.StablePrefixTokens < p.policy.MinStablePrefixTokens {
		updated.Metadata["prefix_cache_lookup"] = "skipped"
		updated.Metadata["prefix_cache_admission"] = "skipped"
		updated.Metadata["prefix_cache_miss_reason"] = "short_prefix"
		updated.Metadata["prefix_cache_segment_reason"] = dominantSegment(identity)
		applyPredictionMetadata(updated.Metadata, identity)
		return updated, core.ProcessResult{
			Details: map[string]string{
				"prefix_cache_lookup":      "skipped",
				"prefix_cache_miss_reason": "short_prefix",
			},
		}, nil
	}
	if core.StableSegmentCount(identity) < p.policy.MinSegmentCount {
		updated.Metadata["prefix_cache_lookup"] = "skipped"
		updated.Metadata["prefix_cache_admission"] = "skipped"
		updated.Metadata["prefix_cache_miss_reason"] = "low_value_prefix"
		updated.Metadata["prefix_cache_segment_reason"] = dominantSegment(identity)
		applyPredictionMetadata(updated.Metadata, identity)
		return updated, core.ProcessResult{
			Details: map[string]string{
				"prefix_cache_lookup":      "skipped",
				"prefix_cache_miss_reason": "low_value_prefix",
			},
		}, nil
	}
	updated.Metadata["prefix_cache_admission"] = "admitted"
	registration, err := p.store.RegisterPrefix(ctx, repository.PrefixCacheEntry{
		Key:                  identity.CombinedPrefixHash,
		SessionScope:         namespace + "|" + updated.SessionID,
		Namespace:            namespace,
		ModelID:              identity.ModelID,
		PrefixHash:           identity.CombinedPrefixHash,
		SystemPrefixHash:     identity.SystemPrefixHash,
		MemoryPrefixHash:     identity.MemoryPrefixHash,
		RAGPrefixHash:        identity.RAGPrefixHash,
		StablePrefixTokens:   identity.StablePrefixTokens,
		SystemPrefixTokens:   identity.SystemPrefixTokens,
		MemoryPrefixTokens:   identity.MemoryPrefixTokens,
		RAGPrefixTokens:      identity.RAGPrefixTokens,
		PromptLayoutVersion:  updated.Metadata["prompt_layout_version"],
		ArtifactKeyVersion:   updated.Metadata["artifact_key_version"],
		CacheOptimizationAim: updated.Metadata["cache_optimization_target"],
		NormalizationVersion: identity.NormalizationVersion,
	})
	if err != nil {
		return nil, core.ProcessResult{}, err
	}
	analysis := analyzePrefixRegistration(identity, registration)
	updated.Metadata["prefix_cache_key"] = registration.Entry.Key
	updated.Metadata["prefix_cache_hash"] = registration.Entry.PrefixHash
	updated.Metadata["prefix_cache_model"] = registration.Entry.ModelID
	updated.Metadata["prefix_cache_lookup"] = analysis.LookupResult
	updated.Metadata["prefix_cache_hit_count"] = strconv.FormatInt(registration.Entry.HitCount, 10)
	updated.Metadata["prefix_cache_miss_reason"] = analysis.MissReason
	updated.Metadata["prefix_cache_segment_reason"] = analysis.SegmentReason
	updated.Metadata["prefix_cache_previous_system_hash"] = analysis.PreviousSegments.System
	updated.Metadata["prefix_cache_previous_memory_hash"] = analysis.PreviousSegments.Memory
	updated.Metadata["prefix_cache_previous_rag_hash"] = analysis.PreviousSegments.RAG
	updated.Metadata["prefix_cache_system_changed"] = strconv.FormatBool(analysis.PreviousSegments.System != "" && analysis.PreviousSegments.System != identity.SystemPrefixHash)
	updated.Metadata["prefix_cache_memory_changed"] = strconv.FormatBool(analysis.PreviousSegments.Memory != "" && analysis.PreviousSegments.Memory != identity.MemoryPrefixHash)
	updated.Metadata["prefix_cache_rag_changed"] = strconv.FormatBool(analysis.PreviousSegments.RAG != "" && analysis.PreviousSegments.RAG != identity.RAGPrefixHash)
	updated.Metadata["prefix_cache_hot"] = strconv.FormatBool(registration.Entry.Hot)
	updated.Metadata["prefix_cache_hot_score"] = strconv.FormatFloat(registration.Entry.HotScore, 'f', 0, 64)
	updated.Metadata["prefix_cache_ttl_tier"] = registration.Entry.CacheTier
	updated.Metadata["prefix_cache_applied_ttl_seconds"] = strconv.FormatInt(registration.Entry.AppliedTTLSeconds, 10)
	applyPredictionMetadata(updated.Metadata, identity)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"prefix_cache_lookup":         analysis.LookupResult,
			"prefix_cache_miss_reason":    analysis.MissReason,
			"prefix_cache_segment_reason": analysis.SegmentReason,
		},
		Semantic: core.StepSemanticAudit{
			Retained:            appendNonEmpty(nil, "stable_prefix_hash", "stable_prefix_tokens", "model_id", "segment_prefix_hashes", "miss_reason"),
			Reasons:             appendNonEmpty(nil, "register_application_prefix_identity", "diagnose_prefix_cache_miss"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *AssembleProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	updated.OptimizedPrompt = p.promptComponent.AssemblePrompt(toPromptMessages(updated.Messages), toComponentChunks(updated.RAGChunks))
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]string)
	}
	systemMessages, memoryMessages, dynamicMessages := p.promptComponent.StablePromptSegments(toPromptMessages(updated.Messages))
	identity := core.BuildPrefixCacheIdentity(updated, p.counter)
	updated.Metadata["prompt_layout"] = "stable-context-first"
	updated.Metadata["stable_prefix_tokens"] = strconv.Itoa(identity.StablePrefixTokens)
	updated.Metadata["stable_rag_chunks"] = strconv.Itoa(len(p.ragNormalizer.StableChunks(toComponentChunks(updated.RAGChunks))))
	updated.Metadata["stable_messages"] = strconv.Itoa(len(systemMessages) + len(memoryMessages))
	updated.Metadata["stable_system_messages"] = strconv.Itoa(len(systemMessages))
	updated.Metadata["stable_memory_messages"] = strconv.Itoa(len(memoryMessages))
	updated.Metadata["dynamic_messages"] = strconv.Itoa(len(dynamicMessages))
	updated.Metadata["system_prefix_tokens"] = strconv.Itoa(identity.SystemPrefixTokens)
	updated.Metadata["memory_prefix_tokens"] = strconv.Itoa(identity.MemoryPrefixTokens)
	updated.Metadata["rag_prefix_tokens"] = strconv.Itoa(identity.RAGPrefixTokens)
	updated.CurrentTokens = p.counter.CountText(updated.OptimizedPrompt)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"prompt_ready": "true",
		},
		Semantic: core.StepSemanticAudit{
			Retained:            appendNonEmpty(nil, "messages", "rag_fragments", "citations"),
			Reasons:             appendNonEmpty(nil, "assemble_final_prompt"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func analyzePrefixRegistration(identity core.PrefixCacheIdentity, registration repository.PrefixCacheRegistration) core.PrefixMissAnalysis {
	analysis := core.PrefixMissAnalysis{
		LookupResult:  registration.Result,
		MissReason:    registration.MissReason,
		SegmentReason: registration.SegmentReason,
		CurrentSegments: core.PrefixSegmentKeys{
			System: identity.SystemPrefixHash,
			Memory: identity.MemoryPrefixHash,
			RAG:    identity.RAGPrefixHash,
		},
	}
	if registration.Result == "hit" {
		if analysis.MissReason == "" {
			analysis.MissReason = "none"
		}
		if analysis.SegmentReason == "" {
			analysis.SegmentReason = "none"
		}
		return analysis
	}
	analysis.PreviousSegments = core.PrefixSegmentKeys{
		System: registration.PreviousEntry.SystemPrefixHash,
		Memory: registration.PreviousEntry.MemoryPrefixHash,
		RAG:    registration.PreviousEntry.RAGPrefixHash,
	}
	if analysis.MissReason == "" {
		analysis.MissReason = "created"
	}
	if analysis.SegmentReason == "" {
		analysis.SegmentReason = dominantSegment(identity)
	}
	return analysis
}

func dominantSegment(identity core.PrefixCacheIdentity) string {
	switch {
	case identity.SystemPrefixTokens >= identity.MemoryPrefixTokens && identity.SystemPrefixTokens >= identity.RAGPrefixTokens:
		return "system"
	case identity.MemoryPrefixTokens >= identity.RAGPrefixTokens:
		return "memory"
	default:
		return "rag"
	}
}

func (p *PrefixCacheProcessor) resolveTenant(req *core.RefineRequest) string {
	if req == nil || req.Metadata == nil {
		return p.policy.DefaultTenant
	}
	return metadataFirstNonBlank(req.Metadata, p.policy.DefaultTenant, "tenant", "tenant_id")
}

func (p *PrefixCacheProcessor) buildNamespace(req *core.RefineRequest, identity core.PrefixCacheIdentity) string {
	tenant := p.policy.DefaultTenant
	if req != nil && req.Metadata != nil {
		tenant = metadataFirstNonBlank(req.Metadata, p.policy.DefaultTenant, "tenant", "tenant_id")
	}
	policy := ""
	if req != nil && req.Metadata != nil {
		policy = req.Metadata["policy"]
	}
	return core.BuildPrefixNamespace(policy, tenant, identity.ModelID, p.policy.Namespace)
}

func metadataFirstNonBlank(items map[string]string, fallback string, keys ...string) string {
	for _, key := range keys {
		if value := items[key]; value != "" {
			return value
		}
	}
	return fallback
}

func applyPredictionMetadata(metadata map[string]string, identity core.PrefixCacheIdentity) {
	if metadata == nil {
		return
	}
	metadata["cache_observation_level"] = "application"
	metadata["cache_prediction_result"] = derivePredictionResult(
		metadata["prefix_cache_lookup"],
		metadata["prefix_cache_admission"],
		metadata["prefix_cache_miss_reason"],
	)
	metadata["predicted_reusable_tokens"] = strconv.Itoa(derivePredictedReusableTokens(identity, metadata["prefix_cache_admission"]))
	metadata["segment_churn_reason"] = firstPredictionReason(metadata["prefix_cache_segment_reason"], "none")
}

func derivePredictionResult(lookup string, admission string, missReason string) string {
	if admission != "admitted" {
		switch missReason {
		case "empty", "short_prefix", "low_value_prefix":
			return "predicted_miss"
		default:
			return "unknown"
		}
	}
	if lookup == "hit" {
		return "predicted_hit"
	}
	switch missReason {
	case "created", "ttl_expired":
		return "partial_reusable"
	case "hash_changed", "model_changed", "system_changed", "memory_changed", "rag_changed", "normalization_changed":
		return "unstable_prefix"
	default:
		return "unknown"
	}
}

func derivePredictedReusableTokens(identity core.PrefixCacheIdentity, admission string) int {
	if admission != "admitted" || identity.StablePrefixTokens <= 0 {
		return 0
	}
	return identity.StablePrefixTokens
}

func firstPredictionReason(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
