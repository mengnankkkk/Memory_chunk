package server

import (
	"context"
	"strings"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/engine"
	"context-refiner/internal/store"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefinerServer struct {
	refinerv1.UnimplementedRefinerServiceServer
	registry      *engine.Registry
	counter       engine.TokenCounter
	pageStore     store.PageStore
	policies      map[string]engine.RuntimePolicy
	defaultPolicy string
}

func NewRefinerServer(
	registry *engine.Registry,
	counter engine.TokenCounter,
	pageStore store.PageStore,
	policies map[string]engine.RuntimePolicy,
	defaultPolicy string,
) *RefinerServer {
	return &RefinerServer{
		registry:      registry,
		counter:       counter,
		pageStore:     pageStore,
		policies:      policies,
		defaultPolicy: defaultPolicy,
	}
}

func (s *RefinerServer) Refine(ctx context.Context, req *refinerv1.RefineRequest) (*refinerv1.RefineResponse, error) {
	policyName := strings.TrimSpace(req.GetPolicy())
	if policyName == "" {
		policyName = s.defaultPolicy
	}
	policy, ok := s.policies[policyName]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unknown policy: %s", policyName)
	}

	engineReq := mapRequest(req, policy)
	if engineReq.Budget <= 0 {
		return nil, status.Error(codes.InvalidArgument, "token_budget must be positive or derivable from model.max_context_tokens")
	}

	pipeline := engine.NewPipeline(s.registry.Resolve(policy.Steps...), s.counter)
	resp, err := pipeline.Run(ctx, engineReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "run pipeline failed: %v", err)
	}
	return mapResponse(resp), nil
}

func (s *RefinerServer) PageIn(ctx context.Context, req *refinerv1.PageInRequest) (*refinerv1.PageInResponse, error) {
	out := &refinerv1.PageInResponse{
		Pages: make([]*refinerv1.StoredPage, 0, len(req.GetPageKeys())),
	}
	for _, key := range req.GetPageKeys() {
		page, err := s.pageStore.LoadResolvedPage(ctx, key)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "load page failed for %s: %v", key, err)
		}
		out.Pages = append(out.Pages, &refinerv1.StoredPage{
			Key:          key,
			Content:      page.Content,
			IsSummary:    page.IsSummary,
			SummaryJobId: page.SummaryJobID,
		})
	}
	return out, nil
}

func mapRequest(req *refinerv1.RefineRequest, policy engine.RuntimePolicy) *engine.RefineRequest {
	messages := make([]engine.Message, 0, len(req.GetMessages()))
	for _, item := range req.GetMessages() {
		messages = append(messages, engine.Message{
			Role:    item.GetRole(),
			Content: item.GetContent(),
		})
	}
	chunks := make([]engine.RAGChunk, 0, len(req.GetRagChunks()))
	for _, item := range req.GetRagChunks() {
		fragments := make([]engine.RAGFragment, 0, len(item.GetFragments()))
		for _, fragment := range item.GetFragments() {
			fragments = append(fragments, engine.RAGFragment{
				Type:     mapFragmentType(fragment.GetType()),
				Content:  fragment.GetContent(),
				Language: fragment.GetLanguage(),
			})
		}
		if len(fragments) == 0 && strings.TrimSpace(item.GetContent()) != "" {
			fragments = []engine.RAGFragment{{
				Type:    engine.FragmentTypeBody,
				Content: item.GetContent(),
			}}
		}
		sources := append([]string(nil), item.GetSources()...)
		if len(sources) == 0 && strings.TrimSpace(item.GetSource()) != "" {
			sources = []string{item.GetSource()}
		}
		chunks = append(chunks, engine.RAGChunk{
			ID:        item.GetId(),
			Source:    item.GetSource(),
			Sources:   sources,
			Fragments: fragments,
		})
	}

	model := engine.ModelConfig{}
	if modelCfg := req.GetModel(); modelCfg != nil {
		model = engine.ModelConfig{
			Name:             modelCfg.GetModel(),
			MaxContextTokens: int(modelCfg.GetMaxContextTokens()),
		}
	}

	budget := int(req.GetTokenBudget())
	if budget <= 0 && model.MaxContextTokens > 0 && policy.BudgetRatio > 0 {
		budget = int(float64(model.MaxContextTokens) * policy.BudgetRatio)
	}

	sessionID := strings.TrimSpace(req.GetSessionId())
	requestID := strings.TrimSpace(req.GetRequestId())
	if requestID == "" {
		requestID = uuid.NewString()
	}
	if sessionID == "" {
		sessionID = "session-" + requestID
	}

	return &engine.RefineRequest{
		SessionID:     sessionID,
		RequestID:     requestID,
		Messages:      messages,
		RAGChunks:     chunks,
		Model:         model,
		Budget:        budget,
		Policy:        firstNonEmpty(req.GetPolicy(), policy.Name),
		RuntimePolicy: policy,
		Metadata: map[string]string{
			"policy":     policy.Name,
			"session_id": sessionID,
			"request_id": requestID,
		},
	}
}

func mapResponse(resp *engine.RefineResponse) *refinerv1.RefineResponse {
	audits := make([]*refinerv1.StepAudit, 0, len(resp.Audits))
	for _, item := range resp.Audits {
		audits = append(audits, &refinerv1.StepAudit{
			Name:         item.Name,
			BeforeTokens: int32(item.BeforeTokens),
			AfterTokens:  int32(item.AfterTokens),
			DurationMs:   item.DurationMS,
			Details:      item.Details,
			Capabilities: &refinerv1.ProcessorCapabilities{
				Aggressive:          item.Capabilities.Aggressive,
				Lossy:               item.Capabilities.Lossy,
				StructuredInputOnly: item.Capabilities.StructuredInputOnly,
				MinTriggerTokens:    int32(item.Capabilities.MinTriggerTokens),
				PreserveCitation:    item.Capabilities.PreserveCitation,
			},
			Semantic: &refinerv1.StepSemanticAudit{
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

	paged := make([]*refinerv1.PagedChunk, 0, len(resp.PagedChunks))
	for _, item := range resp.PagedChunks {
		paged = append(paged, &refinerv1.PagedChunk{
			ChunkId:   item.ChunkID,
			PageKeys:  append([]string(nil), item.PageKeys...),
			SessionId: item.SessionID,
			RequestId: item.RequestID,
		})
	}

	return &refinerv1.RefineResponse{
		OptimizedPrompt:      resp.OptimizedPrompt,
		InputTokens:          int32(resp.InputTokens),
		OutputTokens:         int32(resp.OutputTokens),
		Audits:               audits,
		PagedChunks:          paged,
		Metadata:             resp.Metadata,
		BudgetMet:            resp.BudgetMet,
		PendingSummaryJobIds: append([]string(nil), resp.PendingSummaryJobIDs...),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapFragmentType(fragmentType refinerv1.FragmentType) engine.FragmentType {
	switch fragmentType {
	case refinerv1.FragmentType_FRAGMENT_TYPE_TITLE:
		return engine.FragmentTypeTitle
	case refinerv1.FragmentType_FRAGMENT_TYPE_CODE:
		return engine.FragmentTypeCode
	case refinerv1.FragmentType_FRAGMENT_TYPE_TABLE:
		return engine.FragmentTypeTable
	case refinerv1.FragmentType_FRAGMENT_TYPE_JSON:
		return engine.FragmentTypeJSON
	case refinerv1.FragmentType_FRAGMENT_TYPE_TOOL_OUTPUT:
		return engine.FragmentTypeToolOutput
	case refinerv1.FragmentType_FRAGMENT_TYPE_LOG:
		return engine.FragmentTypeLog
	case refinerv1.FragmentType_FRAGMENT_TYPE_ERROR_STACK:
		return engine.FragmentTypeErrorStack
	default:
		return engine.FragmentTypeBody
	}
}
