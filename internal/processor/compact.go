package processor

import (
	"context"
	"fmt"
	"regexp"

	"context-refiner/internal/engine"
)

var blankLineRE = regexp.MustCompile(`\n{3,}`)

type CompactProcessor struct {
	counter engine.TokenCounter
}

func NewCompactProcessor(counter engine.TokenCounter) *CompactProcessor {
	return &CompactProcessor{counter: counter}
}

func (p *CompactProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "compact",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *CompactProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	charDelta := 0

	for i, msg := range updated.Messages {
		after := microCompact(msg.Content)
		charDelta += len(msg.Content) - len(after)
		updated.Messages[i].Content = after
	}
	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			after := microCompact(fragment.Content)
			charDelta += len(fragment.Content) - len(after)
			updated.RAGChunks[i].Fragments[j].Content = after
		}
	}

	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, engine.ProcessResult{
		Details: map[string]string{
			"removed_chars": fmt.Sprintf("%d", charDelta),
		},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("chars=%d", charDelta)),
			Retained:            appendNonEmpty(nil, "fragment_order", "all_sources"),
			Reasons:             appendNonEmpty(nil, "remove_redundant_blank_lines"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func microCompact(text string) string {
	return blankLineRE.ReplaceAllString(text, "\n\n")
}
