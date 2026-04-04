package processor

import (
	"context"
	"fmt"
	"strings"

	"context-refiner/internal/engine"
)

type SnipProcessor struct {
	counter engine.TokenCounter
}

func NewSnipProcessor(counter engine.TokenCounter) *SnipProcessor {
	return &SnipProcessor{counter: counter}
}

func (p *SnipProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "snip",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			MinTriggerTokens:    48,
			PreserveCitation:    true,
		},
	}
}

func (p *SnipProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	snipped := 0
	keepHead := updated.RuntimePolicy.Snip.KeepHeadLines
	keepTail := updated.RuntimePolicy.Snip.KeepTailLines
	if keepHead <= 0 {
		keepHead = 18
	}
	if keepTail <= 0 {
		keepTail = 8
	}

	if updated.CurrentTokens <= updated.Budget {
		return updated, engine.ProcessResult{Details: map[string]string{"snipped_items": "0"}}, nil
	}

	for i, chunk := range updated.RAGChunks {
		if updated.CurrentTokens <= updated.Budget {
			break
		}
		before := p.counter.CountChunk(chunk)
		if before <= 48 {
			continue
		}
		nextChunk, changed := p.snipChunk(chunk, keepHead, keepTail)
		if changed {
			updated.RAGChunks[i] = nextChunk
			snipped++
			updated.CurrentTokens = p.counter.CountRequest(updated)
		}
	}

	return updated, engine.ProcessResult{
		Details: map[string]string{
			"snipped_items": fmt.Sprintf("%d", snipped),
			"keep_head":     fmt.Sprintf("%d", keepHead),
			"keep_tail":     fmt.Sprintf("%d", keepTail),
		},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("snipped_chunks=%d", snipped)),
			Retained:            appendNonEmpty(nil, "head_lines", "tail_lines", "citations"),
			Reasons:             appendNonEmpty(nil, "middle_out_trim_for_large_fragments"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *SnipProcessor) snipChunk(chunk engine.RAGChunk, keepHead, keepTail int) (engine.RAGChunk, bool) {
	updated := chunk
	changed := false
	for i, fragment := range updated.Fragments {
		if !snipEligible(fragment.Type) {
			continue
		}
		nextContent := p.snipContent(fragment.Content, keepHead, keepTail)
		if nextContent == fragment.Content {
			continue
		}
		updated.Fragments[i].Content = nextContent
		changed = true
	}
	return updated, changed
}

func (p *SnipProcessor) snipContent(content string, keepHead, keepTail int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= keepHead+keepTail {
		return content
	}
	head := strings.Join(lines[:keepHead], "\n")
	tail := strings.Join(lines[len(lines)-keepTail:], "\n")
	return fmt.Sprintf("%s\n[... middle content snipped ...]\n%s", head, tail)
}

func snipEligible(fragmentType engine.FragmentType) bool {
	switch fragmentType {
	case engine.FragmentTypeCode, engine.FragmentTypeToolOutput, engine.FragmentTypeLog, engine.FragmentTypeErrorStack, engine.FragmentTypeJSON:
		return true
	default:
		return false
	}
}
