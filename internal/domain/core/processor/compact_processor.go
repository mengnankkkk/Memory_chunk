package processor

import (
	"context"
	"fmt"
	"regexp"

	"context-refiner/internal/domain/core"
)

var blankLineRE = regexp.MustCompile(`\n{3,}`)

type CompactProcessor struct {
	counter core.TokenCounter
}

func NewCompactProcessor(counter core.TokenCounter) *CompactProcessor {
	return &CompactProcessor{counter: counter}
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

func microCompact(text string) string {
	return blankLineRE.ReplaceAllString(text, "\n\n")
}
