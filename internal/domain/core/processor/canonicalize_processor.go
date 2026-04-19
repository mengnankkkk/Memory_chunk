package processor

import (
	"context"
	"fmt"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/components"
)

type CanonicalizeProcessor struct {
	counter         core.TokenCounter
	promptComponent *components.PromptComponent
	ragNormalizer   *components.RAGNormalizer
}

func NewCanonicalizeProcessor(counter core.TokenCounter) *CanonicalizeProcessor {
	return &CanonicalizeProcessor{
		counter:         counter,
		promptComponent: components.NewPromptComponent(),
		ragNormalizer:   components.NewRAGNormalizer(),
	}
}

func (p *CanonicalizeProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "canonicalize",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *CanonicalizeProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	before := len(updated.RAGChunks)
	systemMessages, memoryMessages, activeMessages := p.promptComponent.StablePromptSegments(toPromptMessages(updated.Messages))
	updated.Messages = append(append(fromPromptMessages(systemMessages), fromPromptMessages(memoryMessages)...), fromPromptMessages(activeMessages)...)
	updated.RAGChunks = fromComponentChunks(p.ragNormalizer.StableChunks(toComponentChunks(updated.RAGChunks)))
	for i, chunk := range updated.RAGChunks {
		updated.RAGChunks[i].Sources = p.ragNormalizer.StableSources(chunk.Sources, chunk.Source)
	}
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]string)
	}
	updated.Metadata["canonicalized_rag"] = "true"
	updated.Metadata["canonicalized_messages"] = "true"
	updated.Metadata["normalization_version"] = "stable-prefix-v2"
	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"canonicalized_chunks":   fmt.Sprintf("%d", before),
			"stable_system_messages": fmt.Sprintf("%d", len(systemMessages)),
			"stable_memory_messages": fmt.Sprintf("%d", len(memoryMessages)),
			"active_messages":        fmt.Sprintf("%d", len(activeMessages)),
		},
		Semantic: core.StepSemanticAudit{
			Retained:            appendNonEmpty(nil, "rag_ordering", "sources", "fragments", "message_roles", "message_content"),
			Reasons:             appendNonEmpty(nil, "stabilize_prompt_prefix_for_cache_hits", "normalize_high_churn_fields"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}
