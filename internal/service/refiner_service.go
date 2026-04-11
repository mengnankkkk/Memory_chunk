package service

import (
	"context"
	"strings"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/core"
	"context-refiner/internal/core/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefinerApplicationService struct {
	registry      *core.Registry
	counter       core.TokenCounter
	pageStore     repository.PageRepository
	policies      map[string]core.RuntimePolicy
	defaultPolicy string
}

func NewRefinerApplicationService(
	registry *core.Registry,
	counter core.TokenCounter,
	pageStore repository.PageRepository,
	policies map[string]core.RuntimePolicy,
	defaultPolicy string,
) *RefinerApplicationService {
	return &RefinerApplicationService{
		registry:      registry,
		counter:       counter,
		pageStore:     pageStore,
		policies:      policies,
		defaultPolicy: defaultPolicy,
	}
}

func (s *RefinerApplicationService) Refine(ctx context.Context, req *refinerv1.RefineRequest) (*refinerv1.RefineResponse, error) {
	policy, err := s.resolvePolicy(req)
	if err != nil {
		return nil, err
	}

	engineReq := mapRequest(req, policy)
	if engineReq.Budget <= 0 {
		return nil, status.Error(codes.InvalidArgument, "token_budget must be positive or derivable from model.max_context_tokens")
	}

	pipeline := core.NewPipeline(s.registry.Resolve(policy.Steps...), s.counter)
	resp, err := pipeline.Run(ctx, engineReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "run pipeline failed: %v", err)
	}
	return mapResponse(resp), nil
}

func (s *RefinerApplicationService) PageIn(ctx context.Context, req *refinerv1.PageInRequest) (*refinerv1.PageInResponse, error) {
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

func (s *RefinerApplicationService) resolvePolicy(req *refinerv1.RefineRequest) (core.RuntimePolicy, error) {
	policyName := strings.TrimSpace(req.GetPolicy())
	if policyName == "" {
		policyName = s.defaultPolicy
	}
	policy, ok := s.policies[policyName]
	if !ok {
		return core.RuntimePolicy{}, status.Errorf(codes.InvalidArgument, "unknown policy: %s", policyName)
	}
	return policy, nil
}
