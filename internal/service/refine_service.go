package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/repository"
	"context-refiner/internal/dto"
	"context-refiner/internal/mapper"
	"context-refiner/internal/observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefinerApplicationService struct {
	registry       *core.Registry
	counter        core.TokenCounter
	pageStore      repository.PageRepository
	evaluationRepo repository.TraceEvaluationRepository
	metrics        observability.Recorder
	policies       map[string]core.RuntimePolicy
	defaultPolicy  string
}

func NewRefinerApplicationService(
	registry *core.Registry,
	counter core.TokenCounter,
	pageStore repository.PageRepository,
	evaluationRepo repository.TraceEvaluationRepository,
	metrics observability.Recorder,
	policies map[string]core.RuntimePolicy,
	defaultPolicy string,
) *RefinerApplicationService {
	return &RefinerApplicationService{
		registry:       registry,
		counter:        counter,
		pageStore:      pageStore,
		evaluationRepo: evaluationRepo,
		metrics:        metrics,
		policies:       policies,
		defaultPolicy:  defaultPolicy,
	}
}

func (s *RefinerApplicationService) Refine(ctx context.Context, req *refinerv1.RefineRequest) (*refinerv1.RefineResponse, error) {
	dtoReq := mapper.MapRefineProtoRequestToDTO(req)
	dtoResp, err := s.RefineDTO(ctx, dtoReq)
	if err != nil {
		return nil, err
	}
	return mapper.MapRefineDTOToProtoResponse(dtoResp), nil
}

func (s *RefinerApplicationService) RefineDTO(ctx context.Context, req *dto.RefineRequest) (*dto.RefineResponse, error) {
	start := time.Now()
	ctx, span := otel.Tracer("context-refiner/service").Start(ctx, "refiner.refine")
	defer span.End()

	policyName := strings.TrimSpace(req.Policy)
	if policyName == "" {
		policyName = s.defaultPolicy
	}
	span.SetAttributes(
		attribute.String("refiner.policy", policyName),
		attribute.String("refiner.session_id", req.SessionID),
		attribute.String("refiner.request_id", req.RequestID),
		attribute.Int("refiner.message_count", len(req.Messages)),
		attribute.Int("refiner.rag_chunk_count", len(req.Memory.RAGChunks)),
	)
	policy, err := s.resolvePolicy(req.Policy)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.metrics.ObserveRefine(policyName, "error", "unknown", 0, 0, 0, 0, 0, 0, 0, 0, time.Since(start))
		return nil, err
	}

	engineReq := mapper.MapRefineDTOToDomainRequest(req, policy)
	beforeContext := core.AssemblePrompt(engineReq)
	if engineReq.Budget <= 0 {
		err := status.Error(codes.InvalidArgument, "token_budget must be positive or derivable from model.max_context_tokens")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.metrics.ObserveRefine(policy.Name, "error", "unknown", 0, 0, 0, 0, 0, 0, 0, 0, time.Since(start))
		return nil, err
	}

	pipeline := core.NewPipeline(s.registry.Resolve(policy.Steps...), s.counter)
	resp, err := pipeline.Run(ctx, engineReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.metrics.ObserveRefine(policy.Name, "error", "unknown", engineReq.InputTokens, 0, 0, 0, 0, 0, 0, 0, time.Since(start))
		return nil, status.Errorf(codes.Internal, "run pipeline failed: %v", err)
	}
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]string)
	}
	if traceID := strings.TrimSpace(span.SpanContext().TraceID().String()); traceID != "" {
		resp.Metadata["trace_id"] = traceID
	}
	for _, audit := range resp.Audits {
		s.metrics.ObservePipelineStep(audit.Name, audit.BeforeTokens, audit.AfterTokens, time.Duration(audit.DurationMS)*time.Millisecond)
	}
	s.metrics.ObservePrefixCacheLookup(
		metadataString(resp.Metadata, "prefix_cache_lookup"),
		metadataString(resp.Metadata, "prefix_cache_miss_reason"),
		metadataInt(resp.Metadata, "stable_prefix_tokens"),
	)
	s.metrics.ObserveRefine(
		policy.Name,
		"ok",
		strconv.FormatBool(resp.BudgetMet),
		resp.InputTokens,
		resp.OutputTokens,
		metadataInt(resp.Metadata, "stable_prefix_tokens"),
		metadataInt(resp.Metadata, "stable_rag_chunks"),
		metadataInt(resp.Metadata, "stable_messages"),
		metadataInt(resp.Metadata, "dynamic_messages"),
		len(resp.PagedChunks),
		len(resp.PendingSummaryJobIDs),
		time.Since(start),
	)
	span.SetAttributes(
		attribute.Bool("refiner.budget_met", resp.BudgetMet),
		attribute.Int("refiner.input_tokens", resp.InputTokens),
		attribute.Int("refiner.output_tokens", resp.OutputTokens),
		attribute.Int("refiner.saved_tokens", maxInt(0, resp.InputTokens-resp.OutputTokens)),
		attribute.Float64("refiner.compression_ratio", compressionRatio(resp.InputTokens, resp.OutputTokens)),
		attribute.Int("refiner.stable_prefix_tokens", metadataInt(resp.Metadata, "stable_prefix_tokens")),
		attribute.String("refiner.prefix_cache_lookup", metadataString(resp.Metadata, "prefix_cache_lookup")),
		attribute.String("refiner.prefix_cache_miss_reason", metadataString(resp.Metadata, "prefix_cache_miss_reason")),
		attribute.String("refiner.prefix_cache_segment_reason", metadataString(resp.Metadata, "prefix_cache_segment_reason")),
		attribute.String("refiner.prefix_cache_key", metadataString(resp.Metadata, "prefix_cache_key")),
		attribute.Int("refiner.pending_summary_jobs", len(resp.PendingSummaryJobIDs)),
		attribute.Int("refiner.paged_chunks", len(resp.PagedChunks)),
	)
	if err := s.saveTraceEvaluation(ctx, span.SpanContext().TraceID().String(), req, engineReq, resp, beforeContext); err != nil {
		span.AddEvent("trace_evaluation_save_failed")
	}
	return mapper.MapRefineDomainResponseToDTO(resp), nil
}

func (s *RefinerApplicationService) PageIn(ctx context.Context, req *refinerv1.PageInRequest) (*refinerv1.PageInResponse, error) {
	dtoReq := mapper.MapPageInProtoRequestToDTO(req)
	dtoResp, err := s.PageInDTO(ctx, dtoReq)
	if err != nil {
		return nil, err
	}
	return mapper.MapPageInDTOToProtoResponse(dtoResp), nil
}

func (s *RefinerApplicationService) PageInDTO(ctx context.Context, req *dto.PageInRequest) (*dto.PageInResponse, error) {
	start := time.Now()
	ctx, span := otel.Tracer("context-refiner/service").Start(ctx, "refiner.pagein")
	defer span.End()
	span.SetAttributes(attribute.Int("pagein.requested_pages", len(req.PageKeys)))

	out := &dto.PageInResponse{
		Pages: make([]dto.StoredPage, 0, len(req.PageKeys)),
	}
	summaryHits := 0
	pageHits := 0
	for _, key := range req.PageKeys {
		page, err := s.pageStore.LoadResolvedPage(ctx, key)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			s.metrics.ObservePageIn("error", len(req.PageKeys), summaryHits, pageHits, time.Since(start))
			return nil, status.Errorf(codes.NotFound, "load page failed for %s: %v", key, err)
		}
		if page.IsSummary {
			summaryHits++
		} else {
			pageHits++
		}
		out.Pages = append(out.Pages, dto.StoredPage{
			Key:             key,
			Content:         page.Content,
			IsSummary:       page.IsSummary,
			SummaryJobID:    page.SummaryJobID,
			SummaryArtifact: mapSummaryArtifactToDTO(page.SummaryArtifact),
		})
	}
	s.metrics.ObservePageIn("ok", len(req.PageKeys), summaryHits, pageHits, time.Since(start))
	span.SetAttributes(
		attribute.Int("pagein.summary_hits", summaryHits),
		attribute.Int("pagein.page_hits", pageHits),
	)
	return out, nil
}

func (s *RefinerApplicationService) resolvePolicy(policyValue string) (core.RuntimePolicy, error) {
	policyName := strings.TrimSpace(policyValue)
	if policyName == "" {
		policyName = s.defaultPolicy
	}
	policy, ok := s.policies[policyName]
	if !ok {
		return core.RuntimePolicy{}, status.Errorf(codes.InvalidArgument, "unknown policy: %s", policyName)
	}
	return policy, nil
}

func metadataInt(items map[string]string, key string) int {
	if len(items) == 0 {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(items[key]))
	if err != nil {
		return 0
	}
	return value
}

func metadataString(items map[string]string, key string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(items[key])
}

func (s *RefinerApplicationService) saveTraceEvaluation(
	ctx context.Context,
	traceID string,
	req *dto.RefineRequest,
	engineReq *core.RefineRequest,
	resp *core.RefineResponse,
	beforeContext string,
) error {
	if s.evaluationRepo == nil {
		return nil
	}
	traceID = strings.TrimSpace(traceID)
	if traceID == "" || req == nil || engineReq == nil || resp == nil {
		return nil
	}

	snapshot := repository.TraceEvaluation{
		TraceID:          traceID,
		SessionID:        req.SessionID,
		RequestID:        req.RequestID,
		Policy:           engineReq.RuntimePolicy.Name,
		ModelName:        engineReq.Model.Name,
		Budget:           engineReq.Budget,
		BudgetMet:        resp.BudgetMet,
		MessageCount:     len(req.Messages),
		RAGChunkCount:    len(req.Memory.RAGChunks),
		InputTokens:      resp.InputTokens,
		OutputTokens:     resp.OutputTokens,
		SavedTokens:      maxInt(0, resp.InputTokens-resp.OutputTokens),
		CompressionRatio: compressionRatio(resp.InputTokens, resp.OutputTokens),
		InputContext:     mapEvaluationInputContext(req),
		OutputContext:    mapEvaluationOutputContext(resp),
		BeforeContext:    beforeContext,
		AfterContext:     resp.OptimizedPrompt,
		Metadata:         cloneStringMap(resp.Metadata),
		Steps:            mapEvaluationSteps(resp.Audits),
		PagedChunks:      mapEvaluationPagedChunks(resp.PagedChunks),
		CreatedAt:        time.Now().UTC(),
	}
	return s.evaluationRepo.SaveTraceEvaluation(ctx, snapshot)
}

func mapEvaluationInputContext(req *dto.RefineRequest) repository.TraceEvaluationContext {
	if req == nil {
		return repository.TraceEvaluationContext{}
	}
	return repository.TraceEvaluationContext{
		System:   strings.TrimSpace(req.System),
		Messages: mapEvaluationMessagesFromDTO(req.Messages),
		Memory: repository.TraceEvaluationMemory{
			RAG: mapEvaluationDTORAGChunks(req.Memory.RAGChunks),
		},
	}
}

func mapEvaluationOutputContext(resp *core.RefineResponse) repository.TraceEvaluationContext {
	if resp == nil {
		return repository.TraceEvaluationContext{}
	}
	systemMessages := make([]string, 0)
	messages := make([]repository.TraceEvaluationMessage, 0, len(resp.Messages))
	for _, item := range resp.Messages {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Role), "system") {
			systemMessages = append(systemMessages, strings.TrimSpace(item.Content))
			continue
		}
		messages = append(messages, repository.TraceEvaluationMessage{
			Role:    strings.TrimSpace(item.Role),
			Content: strings.TrimSpace(item.Content),
		})
	}
	return repository.TraceEvaluationContext{
		System:   strings.TrimSpace(strings.Join(systemMessages, "\n\n")),
		Messages: messages,
		Memory: repository.TraceEvaluationMemory{
			RAG: mapEvaluationRAGChunks(resp.RAGChunks),
		},
	}
}

func mapEvaluationMessagesFromDTO(items []dto.Message) []repository.TraceEvaluationMessage {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceEvaluationMessage, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		out = append(out, repository.TraceEvaluationMessage{
			Role:    strings.TrimSpace(item.Role),
			Content: strings.TrimSpace(item.Content),
		})
	}
	return out
}

func mapEvaluationRAGChunks(items []core.RAGChunk) []repository.TraceEvaluationRAGChunk {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceEvaluationRAGChunk, 0, len(items))
	for _, item := range items {
		out = append(out, repository.TraceEvaluationRAGChunk{
			ID:        strings.TrimSpace(item.ID),
			Source:    strings.TrimSpace(item.Source),
			Sources:   append([]string(nil), item.Sources...),
			Fragments: mapEvaluationRAGFragments(item.Fragments),
			PageRefs:  append([]string(nil), item.PageRefs...),
		})
	}
	return out
}

func mapEvaluationRAGFragments(items []core.RAGFragment) []repository.TraceEvaluationRAGFragment {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceEvaluationRAGFragment, 0, len(items))
	for _, item := range items {
		out = append(out, repository.TraceEvaluationRAGFragment{
			Type:     string(item.Type),
			Content:  strings.TrimSpace(item.Content),
			Language: strings.TrimSpace(item.Language),
		})
	}
	return out
}

func mapEvaluationDTORAGChunks(items []dto.RAGChunk) []repository.TraceEvaluationRAGChunk {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceEvaluationRAGChunk, 0, len(items))
	for _, item := range items {
		out = append(out, repository.TraceEvaluationRAGChunk{
			ID:        strings.TrimSpace(item.ID),
			Source:    strings.TrimSpace(item.Source),
			Sources:   append([]string(nil), item.Sources...),
			Fragments: mapEvaluationDTORAGFragments(item.Fragments),
		})
	}
	return out
}

func mapEvaluationDTORAGFragments(items []dto.RAGFragment) []repository.TraceEvaluationRAGFragment {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceEvaluationRAGFragment, 0, len(items))
	for _, item := range items {
		out = append(out, repository.TraceEvaluationRAGFragment{
			Type:     strings.TrimSpace(item.Type),
			Content:  strings.TrimSpace(item.Content),
			Language: strings.TrimSpace(item.Language),
		})
	}
	return out
}

func mapEvaluationSteps(items []core.StepAudit) []repository.TraceEvaluationStep {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceEvaluationStep, 0, len(items))
	for _, item := range items {
		out = append(out, repository.TraceEvaluationStep{
			Name:         item.Name,
			BeforeTokens: item.BeforeTokens,
			AfterTokens:  item.AfterTokens,
			DeltaTokens:  item.AfterTokens - item.BeforeTokens,
			DurationMS:   item.DurationMS,
			Details:      cloneStringMap(item.Details),
			Capabilities: repository.TraceEvaluationCapabilities{
				Aggressive:          item.Capabilities.Aggressive,
				Lossy:               item.Capabilities.Lossy,
				StructuredInputOnly: item.Capabilities.StructuredInputOnly,
				MinTriggerTokens:    item.Capabilities.MinTriggerTokens,
				PreserveCitation:    item.Capabilities.PreserveCitation,
			},
			Semantic: repository.TraceEvaluationStepSemanticInfo{
				Removed:             append([]string(nil), item.Semantic.Removed...),
				Retained:            append([]string(nil), item.Semantic.Retained...),
				Reasons:             append([]string(nil), item.Semantic.Reasons...),
				SourcePreserved:     item.Semantic.SourcePreserved,
				CodeFencePreserved:  item.Semantic.CodeFencePreserved,
				ErrorStackPreserved: item.Semantic.ErrorStackPreserved,
				DroppedCitations:    item.Semantic.DroppedCitations,
			},
		})
	}
	return out
}

func mapEvaluationPagedChunks(items []core.PagedChunk) []repository.TraceEvaluationPageSet {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceEvaluationPageSet, 0, len(items))
	for _, item := range items {
		out = append(out, repository.TraceEvaluationPageSet{
			SessionID: item.SessionID,
			RequestID: item.RequestID,
			ChunkID:   item.ChunkID,
			PageKeys:  append([]string(nil), item.PageKeys...),
		})
	}
	return out
}

func cloneStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(items))
	for key, value := range items {
		cloned[key] = value
	}
	return cloned
}

func compressionRatio(before int, after int) float64 {
	if before <= 0 {
		return 0
	}
	return float64(after) / float64(before)
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func mapSummaryArtifactToDTO(artifact *repository.SummaryArtifact) *dto.SummaryArtifact {
	if artifact == nil {
		return nil
	}
	return &dto.SummaryArtifact{
		ArtifactID:      artifact.ArtifactID,
		JobID:           artifact.JobID,
		SessionID:       artifact.SessionID,
		RequestID:       artifact.RequestID,
		Policy:          artifact.Policy,
		ChunkID:         artifact.ChunkID,
		Source:          artifact.Source,
		PageRefs:        append([]string(nil), artifact.PageRefs...),
		ContentHash:     artifact.ContentHash,
		SummaryText:     artifact.SummaryText,
		FragmentTypes:   append([]string(nil), artifact.FragmentTypes...),
		Provider:        artifact.Provider,
		ProviderVersion: artifact.ProviderVersion,
		SchemaVersion:   artifact.SchemaVersion,
		CreatedAt:       artifact.CreatedAt,
		ExpiresAt:       artifact.ExpiresAt,
	}
}
