package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"context-refiner/internal/domain/core/components"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
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
	Messages             []Message
	RAGChunks            []RAGChunk
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
	ctx, span := otel.Tracer("context-refiner/core/pipeline").Start(ctx, "pipeline.run")
	defer span.End()

	if req == nil {
		err := fmt.Errorf("nil request")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, err
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]string)
	}
	if req.CurrentTokens == 0 {
		req.CurrentTokens = p.counter.CountRequest(req)
	}
	req.InputTokens = req.CurrentTokens
	span.SetAttributes(
		attribute.String("pipeline.policy", req.RuntimePolicy.Name),
		attribute.Int("pipeline.budget", req.Budget),
		attribute.Int("pipeline.initial_tokens", req.InputTokens),
		attribute.Int("pipeline.processor_count", len(p.processors)),
	)

	for _, processor := range p.processors {
		descriptor := processor.Descriptor()
		if shouldSkipProcessor(req, descriptor) {
			span.AddEvent("processor_skipped")
			continue
		}

		before := req.CurrentTokens
		start := time.Now()
		stepCtx, stepSpan := otel.Tracer("context-refiner/core/pipeline").Start(ctx, "pipeline."+descriptor.Name)
		stepSpan.SetAttributes(
			attribute.String("processor.name", descriptor.Name),
			attribute.Int("processor.before_tokens", before),
		)

		nextReq, result, err := processor.Process(stepCtx, req)
		if err != nil {
			stepSpan.RecordError(err)
			stepSpan.SetStatus(otelcodes.Error, err.Error())
			stepSpan.End()
			err = fmt.Errorf("processor %s failed: %w", descriptor.Name, err)
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			return nil, err
		}
		if nextReq == nil {
			stepSpan.End()
			err := fmt.Errorf("processor %s returned nil request", descriptor.Name)
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			return nil, err
		}
		if nextReq.Metadata == nil {
			nextReq.Metadata = req.Metadata
		}
		if nextReq.CurrentTokens == 0 {
			nextReq.CurrentTokens = p.counter.CountRequest(nextReq)
		}
		stepSpan.SetAttributes(
			attribute.Int("processor.after_tokens", nextReq.CurrentTokens),
			attribute.Int64("processor.duration_ms", time.Since(start).Milliseconds()),
		)
		stepSpan.End()

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
	span.SetAttributes(
		attribute.Int("pipeline.final_tokens", req.CurrentTokens),
		attribute.Bool("pipeline.budget_met", req.CurrentTokens <= req.Budget),
	)

	return &RefineResponse{
		OptimizedPrompt:      req.OptimizedPrompt,
		InputTokens:          req.InputTokens,
		OutputTokens:         req.CurrentTokens,
		BudgetMet:            req.CurrentTokens <= req.Budget,
		Audits:               append([]StepAudit(nil), req.Audits...),
		PagedChunks:          extractPagedChunks(req),
		Messages:             append([]Message(nil), req.Messages...),
		RAGChunks:            append([]RAGChunk(nil), req.RAGChunks...),
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
	return defaultPromptComponent.AssemblePrompt(toPromptMessages(req.Messages), toComponentChunks(req.RAGChunks))
}

func StableRAGChunks(chunks []RAGChunk) []RAGChunk {
	stable := make([]RAGChunk, 0, len(chunks))
	normalizer := defaultRAGNormalizer
	for _, chunk := range chunks {
		stable = append(stable, fromComponentChunk(normalizer.NormalizeChunk(toComponentChunk(chunk)).Chunk))
	}
	sort.SliceStable(stable, func(i, j int) bool {
		left := stableChunkSortKey(stable[i])
		right := stableChunkSortKey(stable[j])
		if left == right {
			return stableChunkTieKey(stable[i]) < stableChunkTieKey(stable[j])
		}
		return left < right
	})
	return stable
}

func StablePromptMessages(messages []Message) ([]Message, []Message) {
	stable, active := defaultPromptComponent.StablePromptMessages(toPromptMessages(messages))
	return fromPromptMessages(stable), fromPromptMessages(active)
}

func StablePromptSegments(messages []Message) ([]Message, []Message, []Message) {
	system, memory, active := defaultPromptComponent.StablePromptSegments(toPromptMessages(messages))
	return fromPromptMessages(system), fromPromptMessages(memory), fromPromptMessages(active)
}

func StableSources(values []string, fallback string) []string {
	return defaultRAGNormalizer.StableSources(values, fallback)
}

func renderChunk(chunk RAGChunk) string {
	return defaultPromptComponent.RenderChunk(toComponentChunk(chunk))
}

func renderMessage(msg Message) string {
	return defaultPromptComponent.RenderMessage(components.PromptMessage{Role: msg.Role, Content: msg.Content})
}

func stableChunkSortKey(chunk RAGChunk) string {
	sourceLabel := strings.Join(chunk.Sources, ",")
	if sourceLabel == "" {
		sourceLabel = strings.TrimSpace(chunk.Source)
	}
	return strings.ToLower(sourceLabel)
}

func stableChunkTieKey(chunk RAGChunk) string {
	id := strings.TrimSpace(chunk.ID)
	if id == "" {
		id = ChunkText(chunk)
	}
	return strings.ToLower(id)
}

func ChunkText(chunk RAGChunk) string {
	return defaultPromptComponent.ChunkText(toComponentChunk(chunk))
}

func FragmentsText(fragments []RAGFragment) string {
	return defaultPromptComponent.FragmentsText(toComponentFragments(fragments))
}

func FragmentText(fragment RAGFragment) string {
	return defaultPromptComponent.FragmentText(toComponentFragment(fragment))
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
