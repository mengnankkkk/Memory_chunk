package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type FragmentType string

const (
	FragmentTypeTitle      FragmentType = "title"
	FragmentTypeBody       FragmentType = "body"
	FragmentTypeCode       FragmentType = "code"
	FragmentTypeTable      FragmentType = "table"
	FragmentTypeJSON       FragmentType = "json"
	FragmentTypeToolOutput FragmentType = "tool-output"
	FragmentTypeLog        FragmentType = "log"
	FragmentTypeErrorStack FragmentType = "error-stack"
)

type Message struct {
	Role    string
	Content string
}

type RAGFragment struct {
	Type     FragmentType
	Content  string
	Language string
}

type RAGChunk struct {
	ID        string
	Source    string
	Sources   []string
	Fragments []RAGFragment
	PageRefs  []string
}

type ModelConfig struct {
	Name             string
	MaxContextTokens int
}

type SnipConfig struct {
	KeepHeadLines int
	KeepTailLines int
}

type RuntimePolicy struct {
	Name                 string
	BudgetRatio          float64
	Steps                []string
	Snip                 SnipConfig
	AutoCompactThreshold int
}

type PagedChunk struct {
	SessionID string
	RequestID string
	ChunkID   string
	PageKeys  []string
}

type ProcessorCapabilities struct {
	Aggressive          bool
	Lossy               bool
	StructuredInputOnly bool
	MinTriggerTokens    int
	PreserveCitation    bool
}

type ProcessorDescriptor struct {
	Name         string
	Capabilities ProcessorCapabilities
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

type StepAudit struct {
	Name         string
	BeforeTokens int
	AfterTokens  int
	DurationMS   int64
	Details      map[string]string
	Capabilities ProcessorCapabilities
	Semantic     StepSemanticAudit
}

type RefineRequest struct {
	SessionID            string
	RequestID            string
	Messages             []Message
	RAGChunks            []RAGChunk
	Model                ModelConfig
	Budget               int
	Policy               string
	RuntimePolicy        RuntimePolicy
	CurrentTokens        int
	InputTokens          int
	OptimizedPrompt      string
	Audits               []StepAudit
	Metadata             map[string]string
	PendingSummaryJobIDs []string
}

type RefineResponse struct {
	OptimizedPrompt      string
	InputTokens          int
	OutputTokens         int
	BudgetMet            bool
	Audits               []StepAudit
	PagedChunks          []PagedChunk
	Metadata             map[string]string
	PendingSummaryJobIDs []string
}

type TokenCounter interface {
	CountText(text string) int
	CountFragment(fragment RAGFragment) int
	CountChunk(chunk RAGChunk) int
	CountRequest(req *RefineRequest) int
}

type ProcessResult struct {
	Details  map[string]string
	Semantic StepSemanticAudit
}

type Processor interface {
	Descriptor() ProcessorDescriptor
	Process(ctx context.Context, req *RefineRequest) (*RefineRequest, ProcessResult, error)
}

type Pipeline struct {
	processors []Processor
	counter    TokenCounter
}

func NewPipeline(processors []Processor, counter TokenCounter) *Pipeline {
	return &Pipeline{
		processors: processors,
		counter:    counter,
	}
}

func (p *Pipeline) Run(ctx context.Context, req *RefineRequest) (*RefineResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]string)
	}
	if req.CurrentTokens == 0 {
		req.CurrentTokens = p.counter.CountRequest(req)
	}
	req.InputTokens = req.CurrentTokens

	for _, processor := range p.processors {
		descriptor := processor.Descriptor()
		if shouldSkipProcessor(req, descriptor) {
			continue
		}

		before := req.CurrentTokens
		start := time.Now()

		nextReq, result, err := processor.Process(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("processor %s failed: %w", descriptor.Name, err)
		}
		if nextReq == nil {
			return nil, fmt.Errorf("processor %s returned nil request", descriptor.Name)
		}
		if nextReq.Metadata == nil {
			nextReq.Metadata = req.Metadata
		}
		if nextReq.CurrentTokens == 0 {
			nextReq.CurrentTokens = p.counter.CountRequest(nextReq)
		}

		nextReq.Audits = append(nextReq.Audits, StepAudit{
			Name:         descriptor.Name,
			BeforeTokens: before,
			AfterTokens:  nextReq.CurrentTokens,
			DurationMS:   time.Since(start).Milliseconds(),
			Details:      cloneMap(result.Details),
			Capabilities: descriptor.Capabilities,
			Semantic:     cloneSemanticAudit(result.Semantic),
		})
		req = nextReq
	}

	if strings.TrimSpace(req.OptimizedPrompt) == "" {
		req.OptimizedPrompt = AssemblePrompt(req)
		req.CurrentTokens = p.counter.CountText(req.OptimizedPrompt)
	}

	return &RefineResponse{
		OptimizedPrompt:      req.OptimizedPrompt,
		InputTokens:          req.InputTokens,
		OutputTokens:         req.CurrentTokens,
		BudgetMet:            req.CurrentTokens <= req.Budget,
		Audits:               append([]StepAudit(nil), req.Audits...),
		PagedChunks:          extractPagedChunks(req),
		Metadata:             cloneMap(req.Metadata),
		PendingSummaryJobIDs: append([]string(nil), req.PendingSummaryJobIDs...),
	}, nil
}

func shouldSkipProcessor(req *RefineRequest, descriptor ProcessorDescriptor) bool {
	capabilities := descriptor.Capabilities
	if req.CurrentTokens <= req.Budget && capabilities.Aggressive {
		return true
	}
	if capabilities.MinTriggerTokens > 0 && req.CurrentTokens < capabilities.MinTriggerTokens {
		return true
	}
	if capabilities.StructuredInputOnly && !hasStructuredChunks(req) {
		return true
	}
	return false
}

func hasStructuredChunks(req *RefineRequest) bool {
	for _, chunk := range req.RAGChunks {
		if len(chunk.Fragments) > 0 {
			return true
		}
	}
	return false
}

func extractPagedChunks(req *RefineRequest) []PagedChunk {
	paged := make([]PagedChunk, 0)
	for _, chunk := range req.RAGChunks {
		if len(chunk.PageRefs) == 0 {
			continue
		}
		paged = append(paged, PagedChunk{
			SessionID: req.SessionID,
			RequestID: req.RequestID,
			ChunkID:   chunk.ID,
			PageKeys:  append([]string(nil), chunk.PageRefs...),
		})
	}
	return paged
}

func AssemblePrompt(req *RefineRequest) string {
	var builder strings.Builder
	builder.WriteString("# Messages\n")
	for _, msg := range req.Messages {
		builder.WriteString(fmt.Sprintf("[%s]\n%s\n\n", strings.ToUpper(msg.Role), strings.TrimSpace(msg.Content)))
	}
	if len(req.RAGChunks) > 0 {
		builder.WriteString("# RAG\n")
		for _, chunk := range req.RAGChunks {
			sources := chunk.Sources
			if len(sources) == 0 && strings.TrimSpace(chunk.Source) != "" {
				sources = []string{chunk.Source}
			}
			sourceLabel := strings.Join(sources, ", ")
			if sourceLabel == "" {
				sourceLabel = "unknown"
			}
			builder.WriteString(fmt.Sprintf("- (%s)\n%s\n", sourceLabel, strings.TrimSpace(ChunkText(chunk))))
		}
	}
	return strings.TrimSpace(builder.String())
}

func ChunkText(chunk RAGChunk) string {
	return FragmentsText(chunk.Fragments)
}

func FragmentsText(fragments []RAGFragment) string {
	parts := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		rendered := FragmentText(fragment)
		if strings.TrimSpace(rendered) != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func FragmentText(fragment RAGFragment) string {
	content := strings.TrimSpace(fragment.Content)
	if content == "" {
		return ""
	}
	switch fragment.Type {
	case FragmentTypeCode:
		return fmt.Sprintf("```%s\n%s\n```", strings.TrimSpace(fragment.Language), content)
	case FragmentTypeTable:
		return "Table:\n" + content
	case FragmentTypeJSON:
		return "JSON:\n" + content
	case FragmentTypeToolOutput:
		return "Tool Output:\n" + content
	case FragmentTypeLog:
		return "Log:\n" + content
	case FragmentTypeErrorStack:
		return "Error Stack:\n" + content
	case FragmentTypeTitle:
		return "Title: " + content
	default:
		return content
	}
}

func cloneMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneSemanticAudit(src StepSemanticAudit) StepSemanticAudit {
	return StepSemanticAudit{
		Removed:             append([]string(nil), src.Removed...),
		Retained:            append([]string(nil), src.Retained...),
		Reasons:             append([]string(nil), src.Reasons...),
		SourcePreserved:     src.SourcePreserved,
		CodeFencePreserved:  src.CodeFencePreserved,
		ErrorStackPreserved: src.ErrorStackPreserved,
		DroppedCitations:    src.DroppedCitations,
	}
}
