package processor

import (
	"context"
	"fmt"
	"strings"

	"context-refiner/internal/engine"
)

type CollapseProcessor struct {
	counter engine.TokenCounter
}

func NewCollapseProcessor(counter engine.TokenCounter) *CollapseProcessor {
	return &CollapseProcessor{counter: counter}
}

func (p *CollapseProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "collapse",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *CollapseProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	seen := make(map[string]int)
	filtered := make([]engine.RAGChunk, 0, len(updated.RAGChunks))
	removed := 0
	mergedSources := 0

	for _, chunk := range updated.RAGChunks {
		key := strings.TrimSpace(normalizeWhitespace(engine.ChunkText(chunk)))
		if idx, ok := seen[key]; ok {
			removed++
			before := len(filtered[idx].Sources)
			filtered[idx].Sources = uniqueStrings(append(filtered[idx].Sources, joinSources(chunk)...))
			if len(filtered[idx].Sources) > before {
				mergedSources += len(filtered[idx].Sources) - before
			}
			continue
		}
		seen[key] = len(filtered)
		chunk.Sources = uniqueStrings(joinSources(chunk))
		if len(chunk.Fragments) == 0 && strings.TrimSpace(key) != "" {
			chunk.Fragments = []engine.RAGFragment{{Type: engine.FragmentTypeBody, Content: key}}
		}
		filtered = append(filtered, chunk)
	}
	updated.RAGChunks = filtered

	for i, msg := range updated.Messages {
		updated.Messages[i].Content = strings.TrimSpace(normalizeWhitespace(msg.Content))
	}

	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, engine.ProcessResult{
		Details: map[string]string{
			"deduped_chunks": fmt.Sprintf("%d", removed),
			"merged_sources": fmt.Sprintf("%d", mergedSources),
		},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("duplicate_chunks=%d", removed)),
			Retained:            appendNonEmpty(nil, "canonical_chunk_content", "merged_citations"),
			Reasons:             appendNonEmpty(nil, "collapse_duplicate_rag_chunks"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
			DroppedCitations:    0,
		},
	}, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
