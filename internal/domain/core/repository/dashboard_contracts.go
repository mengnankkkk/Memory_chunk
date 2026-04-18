package repository

import (
	"context"
	"time"
)

type DashboardQuery struct {
	Limit                int
	SummaryConsumerGroup string
}

type DashboardRepository interface {
	LoadDashboardSnapshot(ctx context.Context, query DashboardQuery) (DashboardSnapshot, error)
	LoadPageDetail(ctx context.Context, key string) (DashboardPageDetail, error)
}

type DashboardSnapshot struct {
	GeneratedAt          time.Time                `json:"generated_at"`
	PageArtifactCount    int                      `json:"page_artifact_count"`
	SummaryArtifactCount int                      `json:"summary_artifact_count"`
	PrefixEntryCount     int                      `json:"prefix_entry_count"`
	PageArtifacts        []DashboardPageArtifact  `json:"page_artifacts"`
	SummaryArtifacts     []DashboardSummaryRecord `json:"summary_artifacts"`
	RecentPrefixes       []DashboardPrefixRecord  `json:"recent_prefixes"`
	HotPrefixes          []DashboardPrefixRecord  `json:"hot_prefixes"`
	SummaryQueue         DashboardQueueSnapshot   `json:"summary_queue"`
}

type DashboardPageArtifact struct {
	Key             string           `json:"key"`
	ContentLength   int              `json:"content_length"`
	TTLSeconds      int64            `json:"ttl_seconds"`
	Preview         string           `json:"preview"`
	SummaryArtifact *SummaryArtifact `json:"summary_artifact,omitempty"`
}

type DashboardSummaryRecord struct {
	PageKey         string    `json:"page_key"`
	ArtifactID      string    `json:"artifact_id"`
	JobID           string    `json:"job_id"`
	Provider        string    `json:"provider"`
	ProviderVersion string    `json:"provider_version"`
	ContentHash     string    `json:"content_hash"`
	FragmentTypes   []string  `json:"fragment_types,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	TTLSeconds      int64     `json:"ttl_seconds"`
	SummaryPreview  string    `json:"summary_preview"`
}

type DashboardPrefixRecord struct {
	StorageKey           string    `json:"storage_key"`
	Namespace            string    `json:"namespace"`
	PrefixHash           string    `json:"prefix_hash"`
	ModelID              string    `json:"model_id"`
	StablePrefixTokens   int       `json:"stable_prefix_tokens"`
	CacheTier            string    `json:"cache_tier"`
	Hot                  bool      `json:"hot"`
	HotScore             float64   `json:"hot_score"`
	HitCount             int64     `json:"hit_count"`
	AdmissionDecision    string    `json:"admission_decision"`
	NormalizationVersion string    `json:"normalization_version"`
	CreatedAt            time.Time `json:"created_at"`
	LastSeenAt           time.Time `json:"last_seen_at"`
}

type DashboardQueueSnapshot struct {
	Stream          string           `json:"stream"`
	ConsumerGroup   string           `json:"consumer_group"`
	StreamLength    int64            `json:"stream_length"`
	PendingCount    int64            `json:"pending_count"`
	ConsumerCount   int              `json:"consumer_count"`
	LastGeneratedID string           `json:"last_generated_id,omitempty"`
	OldestPendingID string           `json:"oldest_pending_id,omitempty"`
	NewestPendingID string           `json:"newest_pending_id,omitempty"`
	Consumers       map[string]int64 `json:"consumers,omitempty"`
}

type DashboardPageDetail struct {
	Key               string           `json:"key"`
	RawContent        string           `json:"raw_content,omitempty"`
	RawTTLSeconds     int64            `json:"raw_ttl_seconds"`
	ResolvedContent   string           `json:"resolved_content,omitempty"`
	ResolvedIsSummary bool             `json:"resolved_is_summary"`
	SummaryTTLSeconds int64            `json:"summary_ttl_seconds"`
	SummaryArtifact   *SummaryArtifact `json:"summary_artifact,omitempty"`
}
