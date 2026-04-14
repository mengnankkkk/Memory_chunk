package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	SummaryArtifactSchemaVersionV1    = "summary-artifact-v1"
	SummaryProviderHeuristic          = "heuristic"
	SummaryProviderVersionHeuristicV1 = "heuristic-v1"
)

type PageRepository interface {
	SavePage(ctx context.Context, key string, content string) error
	LoadPage(ctx context.Context, key string) (string, error)
	LoadResolvedPage(ctx context.Context, key string) (ResolvedPage, error)
	SaveSummary(ctx context.Context, key string, artifact SummaryArtifact) error
}

type SummaryJobRepository interface {
	EnqueueSummaryJob(ctx context.Context, job SummaryJob) error
}

type SummaryJobConsumer interface {
	EnsureSummaryGroup(ctx context.Context, group string) error
	ConsumeSummaryJobs(ctx context.Context, group string, consumer string, count int64, block time.Duration) ([]SummaryJobMessage, error)
	AckSummaryJob(ctx context.Context, group string, messageID string) error
}

type PrefixCacheRepository interface {
	RegisterPrefix(ctx context.Context, entry PrefixCacheEntry) (PrefixCacheRegistration, error)
}

type PrefixCacheEntry struct {
	Key                  string    `json:"key"`
	SessionScope         string    `json:"-"`
	Namespace            string    `json:"namespace"`
	ModelID              string    `json:"model_id"`
	PrefixHash           string    `json:"prefix_hash"`
	SystemPrefixHash     string    `json:"system_prefix_hash"`
	MemoryPrefixHash     string    `json:"memory_prefix_hash"`
	RAGPrefixHash        string    `json:"rag_prefix_hash"`
	StablePrefixTokens   int       `json:"stable_prefix_tokens"`
	SystemPrefixTokens   int       `json:"system_prefix_tokens"`
	MemoryPrefixTokens   int       `json:"memory_prefix_tokens"`
	RAGPrefixTokens      int       `json:"rag_prefix_tokens"`
	PromptLayoutVersion  string    `json:"prompt_layout_version"`
	ArtifactKeyVersion   string    `json:"artifact_key_version"`
	CacheOptimizationAim string    `json:"cache_optimization_aim"`
	NormalizationVersion string    `json:"normalization_version"`
	CacheTier            string    `json:"cache_tier"`
	AdmissionDecision    string    `json:"admission_decision"`
	AppliedTTLSeconds    int64     `json:"applied_ttl_seconds"`
	Hot                  bool      `json:"hot"`
	HotScore             float64   `json:"hot_score"`
	CreatedAt            time.Time `json:"created_at"`
	LastSeenAt           time.Time `json:"last_seen_at"`
	HitCount             int64     `json:"hit_count"`
}

type PrefixCacheRegistration struct {
	Entry         PrefixCacheEntry
	PreviousEntry PrefixCacheEntry
	Result        string
	MissReason    string
	SegmentReason string
}

type SummaryFragment struct {
	Type     string `json:"type"`
	Content  string `json:"content"`
	Language string `json:"language,omitempty"`
}

type SummaryJob struct {
	JobID         string            `json:"job_id"`
	SessionID     string            `json:"session_id"`
	RequestID     string            `json:"request_id"`
	Policy        string            `json:"policy"`
	ChunkID       string            `json:"chunk_id"`
	Source        string            `json:"source"`
	ContentHash   string            `json:"content_hash"`
	PageRefs      []string          `json:"page_refs"`
	Fragments     []SummaryFragment `json:"fragments"`
	Content       string            `json:"content"`
	TargetTokens  int               `json:"target_tokens"`
	CurrentTokens int               `json:"current_tokens"`
	CreatedAt     time.Time         `json:"created_at"`
}

type SummaryJobMessage struct {
	ID  string
	Job SummaryJob
}

type SummaryArtifact struct {
	ArtifactID      string    `json:"artifact_id"`
	JobID           string    `json:"job_id"`
	SessionID       string    `json:"session_id,omitempty"`
	RequestID       string    `json:"request_id,omitempty"`
	Policy          string    `json:"policy,omitempty"`
	ChunkID         string    `json:"chunk_id,omitempty"`
	Source          string    `json:"source,omitempty"`
	PageRefs        []string  `json:"page_refs,omitempty"`
	ContentHash     string    `json:"content_hash"`
	SummaryText     string    `json:"summary_text"`
	FragmentTypes   []string  `json:"fragment_types,omitempty"`
	Provider        string    `json:"provider"`
	ProviderVersion string    `json:"provider_version"`
	SchemaVersion   string    `json:"schema_version"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
}

type ResolvedPage struct {
	Key             string
	Content         string
	IsSummary       bool
	SummaryJobID    string
	SummaryArtifact *SummaryArtifact
}

func BuildSummaryArtifactID(contentHash string, provider string, providerVersion string) string {
	parts := make([]string, 0, 4)
	for _, value := range []string{
		"summary",
		SummaryArtifactSchemaVersionV1,
		strings.TrimSpace(provider),
		strings.TrimSpace(providerVersion),
		strings.TrimSpace(contentHash),
	} {
		if value == "" {
			continue
		}
		parts = append(parts, value)
	}
	if len(parts) == 0 {
		return "summary"
	}
	return fmt.Sprintf("%s", strings.Join(parts, ":"))
}
