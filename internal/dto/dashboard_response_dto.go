package dto

import (
	"time"

	"context-refiner/internal/domain/core/repository"
)

type DashboardOverviewDTO struct {
	GeneratedAt          time.Time `json:"generated_at"`
	GRPCListenAddr       string    `json:"grpc_listen_addr"`
	WebListenAddr        string    `json:"web_listen_addr"`
	MetricsListenAddr    string    `json:"metrics_listen_addr"`
	MetricsPath          string    `json:"metrics_path"`
	RedisAddr            string    `json:"redis_addr"`
	TempoQueryURL        string    `json:"tempo_query_url"`
	DefaultPolicy        string    `json:"default_policy"`
	DefaultTenant        string    `json:"default_tenant"`
	TracingEnabled       bool      `json:"tracing_enabled"`
	SummaryWorkerEnabled bool      `json:"summary_worker_enabled"`
	PageArtifactCount    int       `json:"page_artifact_count"`
	SummaryArtifactCount int       `json:"summary_artifact_count"`
	PrefixEntryCount     int       `json:"prefix_entry_count"`
	SummaryStreamLength  int64     `json:"summary_stream_length"`
	SummaryPendingCount  int64     `json:"summary_pending_count"`
}

type DashboardSnapshotDTO struct {
	Overview         DashboardOverviewDTO                `json:"overview"`
	PageArtifacts    []repository.DashboardPageArtifact  `json:"page_artifacts"`
	SummaryArtifacts []repository.DashboardSummaryRecord `json:"summary_artifacts"`
	RecentPrefixes   []repository.DashboardPrefixRecord  `json:"recent_prefixes"`
	HotPrefixes      []repository.DashboardPrefixRecord  `json:"hot_prefixes"`
	SummaryQueue     repository.DashboardQueueSnapshot   `json:"summary_queue"`
}

type DashboardPageDetailDTO struct {
	Page repository.DashboardPageDetail `json:"page"`
}
