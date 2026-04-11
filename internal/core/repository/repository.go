package repository

import (
	"context"
	"time"
)

type PageRepository interface {
	SavePage(ctx context.Context, key string, content string) error
	LoadPage(ctx context.Context, key string) (string, error)
	LoadResolvedPage(ctx context.Context, key string) (ResolvedPage, error)
	SaveSummary(ctx context.Context, key string, result SummaryResult) error
}

type SummaryJobRepository interface {
	EnqueueSummaryJob(ctx context.Context, job SummaryJob) error
}

type SummaryJobConsumer interface {
	EnsureSummaryGroup(ctx context.Context, group string) error
	ConsumeSummaryJobs(ctx context.Context, group string, consumer string, count int64, block time.Duration) ([]SummaryJobMessage, error)
	AckSummaryJob(ctx context.Context, group string, messageID string) error
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

type SummaryResult struct {
	JobID     string    `json:"job_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ResolvedPage struct {
	Key          string
	Content      string
	IsSummary    bool
	SummaryJobID string
}
