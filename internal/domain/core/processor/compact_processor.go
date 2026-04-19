package processor

import (
	"context"
	"fmt"

	"context-refiner/internal/domain/core"
)

type CompactProcessor struct {
	counter   core.TokenCounter
	sanitizer *core.TextSanitizer
}

func NewCompactProcessor(counter core.TokenCounter) *CompactProcessor {
	return &CompactProcessor{counter: counter, sanitizer: core.NewTextSanitizer()}
}

func (p *CompactProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "compact",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *CompactProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	charDelta := 0

	for i, msg := range updated.Messages {
		after := p.sanitizer.Sanitize(msg.Content, core.TextSanitizerProfileCompactLayout).Text
		charDelta += len(msg.Content) - len(after)
		updated.Messages[i].Content = after
	}
	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			after := p.sanitizer.Sanitize(fragment.Content, core.TextSanitizerProfileCompactLayout).Text
			charDelta += len(fragment.Content) - len(after)
			updated.RAGChunks[i].Fragments[j].Content = after
		}
	}

	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"removed_chars": fmt.Sprintf("%d", charDelta),
		},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("chars=%d", charDelta)),
			Retained:            appendNonEmpty(nil, "fragment_order", "all_sources"),
			Reasons:             appendNonEmpty(nil, "remove_redundant_blank_lines"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}
