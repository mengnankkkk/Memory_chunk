package repository

import (
	"context"
	"time"
)

type TraceEvaluationRepository interface {
	SaveTraceEvaluation(ctx context.Context, snapshot TraceEvaluation) error
	LoadTraceEvaluation(ctx context.Context, traceID string) (TraceEvaluation, error)
}

type TraceEvaluation struct {
	TraceID          string                   `json:"trace_id"`
	SessionID        string                   `json:"session_id,omitempty"`
	RequestID        string                   `json:"request_id,omitempty"`
	Policy           string                   `json:"policy,omitempty"`
	ModelName        string                   `json:"model_name,omitempty"`
	Budget           int                      `json:"budget"`
	BudgetMet        bool                     `json:"budget_met"`
	MessageCount     int                      `json:"message_count"`
	RAGChunkCount    int                      `json:"rag_chunk_count"`
	InputTokens      int                      `json:"input_tokens"`
	OutputTokens     int                      `json:"output_tokens"`
	SavedTokens      int                      `json:"saved_tokens"`
	CompressionRatio float64                  `json:"compression_ratio"`
	BeforeContext    string                   `json:"before_context,omitempty"`
	AfterContext     string                   `json:"after_context,omitempty"`
	Metadata         map[string]string        `json:"metadata,omitempty"`
	Steps            []TraceEvaluationStep    `json:"steps,omitempty"`
	PagedChunks      []TraceEvaluationPageSet `json:"paged_chunks,omitempty"`
	CreatedAt        time.Time                `json:"created_at"`
}

type TraceEvaluationPageSet struct {
	SessionID string   `json:"session_id,omitempty"`
	RequestID string   `json:"request_id,omitempty"`
	ChunkID   string   `json:"chunk_id,omitempty"`
	PageKeys  []string `json:"page_keys,omitempty"`
}

type TraceEvaluationStep struct {
	Name         string                          `json:"name"`
	BeforeTokens int                             `json:"before_tokens"`
	AfterTokens  int                             `json:"after_tokens"`
	DeltaTokens  int                             `json:"delta_tokens"`
	DurationMS   int64                           `json:"duration_ms"`
	Details      map[string]string               `json:"details,omitempty"`
	Capabilities TraceEvaluationCapabilities     `json:"capabilities"`
	Semantic     TraceEvaluationStepSemanticInfo `json:"semantic"`
}

type TraceEvaluationCapabilities struct {
	Aggressive          bool `json:"aggressive"`
	Lossy               bool `json:"lossy"`
	StructuredInputOnly bool `json:"structured_input_only"`
	MinTriggerTokens    int  `json:"min_trigger_tokens"`
	PreserveCitation    bool `json:"preserve_citation"`
}

type TraceEvaluationStepSemanticInfo struct {
	Removed             []string `json:"removed,omitempty"`
	Retained            []string `json:"retained,omitempty"`
	Reasons             []string `json:"reasons,omitempty"`
	SourcePreserved     bool     `json:"source_preserved"`
	CodeFencePreserved  bool     `json:"code_fence_preserved"`
	ErrorStackPreserved bool     `json:"error_stack_preserved"`
	DroppedCitations    int32    `json:"dropped_citations"`
}
