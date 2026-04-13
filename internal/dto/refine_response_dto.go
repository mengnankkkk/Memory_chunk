package dto

type RefineResponse struct {
	OptimizedPrompt      string
	InputTokens          int
	OutputTokens         int
	Audits               []StepAudit
	PagedChunks          []PagedChunk
	Metadata             map[string]string
	BudgetMet            bool
	PendingSummaryJobIDs []string
}

type StepAudit struct {
	Name         string
	BeforeTokens int
	AfterTokens  int
	DurationMS   int64
	Details      map[string]string
	Capabilities ProcessorCapabilities
	Semantic     StepSemanticAudit
}

type ProcessorCapabilities struct {
	Aggressive          bool
	Lossy               bool
	StructuredInputOnly bool
	MinTriggerTokens    int
	PreserveCitation    bool
}

type StepSemanticAudit struct {
	Removed             []string
	Retained            []string
	Reasons             []string
	SourcePreserved     bool
	CodeFencePreserved  bool
	ErrorStackPreserved bool
	DroppedCitations    int32
}

type PagedChunk struct {
	SessionID string
	RequestID string
	ChunkID   string
	PageKeys  []string
}
