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
	registry      *core.Registry
	counter       core.TokenCounter
	pageStore     repository.PageRepository
	metrics       observability.Recorder
	policies      map[string]core.RuntimePolicy
	defaultPolicy string
}

func NewRefinerApplicationService(
	registry *core.Registry,
	counter core.TokenCounter,
	pageStore repository.PageRepository,
	metrics observability.Recorder,
	policies map[string]core.RuntimePolicy,
	defaultPolicy string,
) *RefinerApplicationService {
	return &RefinerApplicationService{
		registry:      registry,
		counter:       counter,
		pageStore:     pageStore,
		metrics:       metrics,
		policies:      policies,
		defaultPolicy: defaultPolicy,
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
		attribute.Int("refiner.rag_chunk_count", len(req.RAGChunks)),
	)
	policy, err := s.resolvePolicy(req.Policy)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		s.metrics.ObserveRefine(policyName, "error", "unknown", 0, 0, 0, 0, 0, 0, 0, 0, time.Since(start))
		return nil, err
	}

	engineReq := mapper.MapRefineDTOToDomainRequest(req, policy)
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
		attribute.Int("refiner.stable_prefix_tokens", metadataInt(resp.Metadata, "stable_prefix_tokens")),
		attribute.String("refiner.prefix_cache_lookup", metadataString(resp.Metadata, "prefix_cache_lookup")),
		attribute.String("refiner.prefix_cache_miss_reason", metadataString(resp.Metadata, "prefix_cache_miss_reason")),
		attribute.String("refiner.prefix_cache_segment_reason", metadataString(resp.Metadata, "prefix_cache_segment_reason")),
		attribute.String("refiner.prefix_cache_key", metadataString(resp.Metadata, "prefix_cache_key")),
		attribute.Int("refiner.pending_summary_jobs", len(resp.PendingSummaryJobIDs)),
		attribute.Int("refiner.paged_chunks", len(resp.PagedChunks)),
	)
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
			Key:          key,
			Content:      page.Content,
			IsSummary:    page.IsSummary,
			SummaryJobID: page.SummaryJobID,
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
