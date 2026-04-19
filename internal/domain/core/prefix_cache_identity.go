package core

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"context-refiner/internal/domain/core/components"
)

type PrefixCacheIdentity struct {
	ModelID              string
	StablePrefixPrompt   string
	CombinedPrefixHash   string
	StablePrefixTokens   int
	SystemPrefixPrompt   string
	SystemPrefixHash     string
	SystemPrefixTokens   int
	MemoryPrefixPrompt   string
	MemoryPrefixHash     string
	MemoryPrefixTokens   int
	RAGPrefixPrompt      string
	RAGPrefixHash        string
	RAGPrefixTokens      int
	NormalizationVersion string
}

type PrefixNamespaceConfig struct {
	IncludePolicy bool
	IncludeModel  bool
	IncludeTenant bool
}

type PrefixCachePolicy struct {
	MinStablePrefixTokens int
	MinSegmentCount       int
	DefaultTenant         string
	Namespace             PrefixNamespaceConfig
}

type PrefixSegmentKeys struct {
	System string
	Memory string
	RAG    string
}

type PrefixMissAnalysis struct {
	LookupResult     string
	MissReason       string
	SegmentReason    string
	CurrentSegments  PrefixSegmentKeys
	PreviousSegments PrefixSegmentKeys
}

func AssembleStablePrefix(req *RefineRequest) string {
	if req == nil {
		return ""
	}
	identity := BuildPrefixCacheIdentity(req, nil)
	return identity.StablePrefixPrompt
}

func BuildPrefixCacheIdentity(req *RefineRequest, counter TokenCounter) PrefixCacheIdentity {
	modelID := normalizedModelID(req)
	sections := defaultPromptComponent.BuildStablePrefixSectionsFromMessages(
		toPromptMessages(nilSafeMessages(req)),
		toComponentChunks(nilSafeChunks(req)),
	)
	return buildPrefixCacheIdentity(modelID, sections, counter)
}

func BuildPrefixCacheIdentityFromSegments(modelID string, systemPrompt string, memoryPrompt string, ragPrompt string, counter TokenCounter) PrefixCacheIdentity {
	modelID = firstNonBlank(modelID, "unknown")
	sections := defaultPromptComponent.BuildStablePrefixSections(systemPrompt, memoryPrompt, ragPrompt)
	return buildPrefixCacheIdentity(modelID, sections, counter)
}

func buildPrefixCacheIdentity(modelID string, sections components.StablePrefixSections, counter TokenCounter) PrefixCacheIdentity {
	identity := PrefixCacheIdentity{
		ModelID:              modelID,
		StablePrefixPrompt:   sections.StablePrompt,
		SystemPrefixPrompt:   sections.SystemPrompt,
		MemoryPrefixPrompt:   sections.MemoryPrompt,
		RAGPrefixPrompt:      sections.RAGPrompt,
		NormalizationVersion: "stable-prefix-v2",
	}
	if sections.StablePrompt != "" {
		identity.CombinedPrefixHash = hashStrings(modelID, sections.StablePrompt)
	}
	if sections.SystemPrompt != "" {
		identity.SystemPrefixHash = hashStrings(modelID, sections.SystemPrompt)
	}
	if sections.MemoryPrompt != "" {
		identity.MemoryPrefixHash = hashStrings(modelID, sections.MemoryPrompt)
	}
	if sections.RAGPrompt != "" {
		identity.RAGPrefixHash = hashStrings(modelID, sections.RAGPrompt)
	}
	if counter != nil {
		identity.StablePrefixTokens = countIfPresent(counter, sections.StablePrompt)
		identity.SystemPrefixTokens = countIfPresent(counter, sections.SystemPrompt)
		identity.MemoryPrefixTokens = countIfPresent(counter, sections.MemoryPrompt)
		identity.RAGPrefixTokens = countIfPresent(counter, sections.RAGPrompt)
	}
	return identity
}

func normalizedModelID(req *RefineRequest) string {
	if req == nil {
		return "unknown"
	}
	return firstNonBlank(req.Model.Name, "unknown")
}

func BuildPrefixNamespace(policy string, tenant string, modelID string, cfg PrefixNamespaceConfig) string {
	parts := make([]string, 0, 3)
	if cfg.IncludeTenant {
		parts = append(parts, "tenant="+firstNonBlank(tenant, "global"))
	}
	if cfg.IncludePolicy {
		parts = append(parts, "policy="+firstNonBlank(policy, "default"))
	}
	if cfg.IncludeModel {
		parts = append(parts, "model="+firstNonBlank(modelID, "unknown"))
	}
	if len(parts) == 0 {
		return "global"
	}
	return strings.Join(parts, "|")
}

func StableSegmentCount(identity PrefixCacheIdentity) int {
	count := 0
	if identity.SystemPrefixHash != "" {
		count++
	}
	if identity.MemoryPrefixHash != "" {
		count++
	}
	if identity.RAGPrefixHash != "" {
		count++
	}
	return count
}

func hashStrings(parts ...string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\n"))))
}

func nilSafeMessages(req *RefineRequest) []Message {
	if req == nil {
		return nil
	}
	return req.Messages
}

func nilSafeChunks(req *RefineRequest) []RAGChunk {
	if req == nil {
		return nil
	}
	return req.RAGChunks
}

func countIfPresent(counter TokenCounter, text string) int {
	if counter == nil || strings.TrimSpace(text) == "" {
		return 0
	}
	return counter.CountText(text)
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
