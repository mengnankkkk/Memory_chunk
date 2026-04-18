package service

import (
	"context"
	"strings"

	"context-refiner/internal/domain/core/repository"
	"context-refiner/internal/dto"
)

type DashboardServiceConfig struct {
	GRPCListenAddr       string
	WebListenAddr        string
	MetricsListenAddr    string
	MetricsPath          string
	RedisAddr            string
	TempoQueryURL        string
	DefaultPolicy        string
	DefaultTenant        string
	TracingEnabled       bool
	SummaryWorkerEnabled bool
	PageSize             int
	SummaryConsumerGroup string
}

type DashboardService struct {
	repository      repository.DashboardRepository
	traceRepository repository.TraceRepository
	evaluationRepo  repository.TraceEvaluationRepository
	cfg             DashboardServiceConfig
}

func NewDashboardService(repo repository.DashboardRepository, traceRepo repository.TraceRepository, evaluationRepo repository.TraceEvaluationRepository, cfg DashboardServiceConfig) *DashboardService {
	return &DashboardService{
		repository:      repo,
		traceRepository: traceRepo,
		evaluationRepo:  evaluationRepo,
		cfg:             cfg,
	}
}

func (s *DashboardService) Snapshot(ctx context.Context) (dto.DashboardSnapshotDTO, error) {
	snapshot, err := s.repository.LoadDashboardSnapshot(ctx, repository.DashboardQuery{
		Limit:                s.cfg.PageSize,
		SummaryConsumerGroup: s.cfg.SummaryConsumerGroup,
	})
	if err != nil {
		return dto.DashboardSnapshotDTO{}, err
	}

	return dto.DashboardSnapshotDTO{
		Overview: dto.DashboardOverviewDTO{
			GeneratedAt:          snapshot.GeneratedAt,
			GRPCListenAddr:       s.cfg.GRPCListenAddr,
			WebListenAddr:        s.cfg.WebListenAddr,
			MetricsListenAddr:    s.cfg.MetricsListenAddr,
			MetricsPath:          s.cfg.MetricsPath,
			RedisAddr:            s.cfg.RedisAddr,
			TempoQueryURL:        s.cfg.TempoQueryURL,
			DefaultPolicy:        s.cfg.DefaultPolicy,
			DefaultTenant:        s.cfg.DefaultTenant,
			TracingEnabled:       s.cfg.TracingEnabled,
			SummaryWorkerEnabled: s.cfg.SummaryWorkerEnabled,
			PageArtifactCount:    snapshot.PageArtifactCount,
			SummaryArtifactCount: snapshot.SummaryArtifactCount,
			PrefixEntryCount:     snapshot.PrefixEntryCount,
			SummaryStreamLength:  snapshot.SummaryQueue.StreamLength,
			SummaryPendingCount:  snapshot.SummaryQueue.PendingCount,
		},
		PageArtifacts:    snapshot.PageArtifacts,
		SummaryArtifacts: snapshot.SummaryArtifacts,
		RecentPrefixes:   snapshot.RecentPrefixes,
		HotPrefixes:      snapshot.HotPrefixes,
		SummaryQueue:     snapshot.SummaryQueue,
	}, nil
}

func (s *DashboardService) PageDetail(ctx context.Context, key string) (dto.DashboardPageDetailDTO, error) {
	detail, err := s.repository.LoadPageDetail(ctx, strings.TrimSpace(key))
	if err != nil {
		return dto.DashboardPageDetailDTO{}, err
	}
	return dto.DashboardPageDetailDTO{Page: detail}, nil
}

func (s *DashboardService) SearchTraces(ctx context.Context, query repository.TraceSearchQuery) (repository.TraceSearchResult, error) {
	return s.traceRepository.SearchTraces(ctx, query)
}

func (s *DashboardService) TraceDetail(ctx context.Context, traceID string, start int64, end int64) (repository.TraceDetail, error) {
	return s.traceRepository.LoadTraceDetail(ctx, strings.TrimSpace(traceID), start, end)
}

func (s *DashboardService) TraceEvaluation(ctx context.Context, traceID string) (repository.TraceEvaluation, error) {
	return s.evaluationRepo.LoadTraceEvaluation(ctx, strings.TrimSpace(traceID))
}
