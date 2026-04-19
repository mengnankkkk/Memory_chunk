package processor

import (
	"context"
	"strconv"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/components"
)

type AssembleProcessor struct {
	counter         core.TokenCounter
	promptComponent *components.PromptComponent
	ragNormalizer   *components.RAGNormalizer
}

func NewAssembleProcessor(counter core.TokenCounter) *AssembleProcessor {
	return &AssembleProcessor{
		counter:         counter,
		promptComponent: components.NewPromptComponent(),
		ragNormalizer:   components.NewRAGNormalizer(),
	}
}

func (p *AssembleProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "assemble",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *AssembleProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	updated.OptimizedPrompt = p.promptComponent.AssemblePrompt(toPromptMessages(updated.Messages), toComponentChunks(updated.RAGChunks))
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]string)
	}
	systemMessages, memoryMessages, dynamicMessages := p.promptComponent.StablePromptSegments(toPromptMessages(updated.Messages))
	identity := core.BuildPrefixCacheIdentity(updated, p.counter)
	updated.Metadata["prompt_layout"] = "stable-context-first"
	updated.Metadata["stable_prefix_tokens"] = strconv.Itoa(identity.StablePrefixTokens)
	updated.Metadata["stable_rag_chunks"] = strconv.Itoa(len(p.ragNormalizer.StableChunks(toComponentChunks(updated.RAGChunks))))
	updated.Metadata["stable_messages"] = strconv.Itoa(len(systemMessages) + len(memoryMessages))
	updated.Metadata["stable_system_messages"] = strconv.Itoa(len(systemMessages))
	updated.Metadata["stable_memory_messages"] = strconv.Itoa(len(memoryMessages))
	updated.Metadata["dynamic_messages"] = strconv.Itoa(len(dynamicMessages))
	updated.Metadata["system_prefix_tokens"] = strconv.Itoa(identity.SystemPrefixTokens)
	updated.Metadata["memory_prefix_tokens"] = strconv.Itoa(identity.MemoryPrefixTokens)
	updated.Metadata["rag_prefix_tokens"] = strconv.Itoa(identity.RAGPrefixTokens)
	updated.CurrentTokens = p.counter.CountText(updated.OptimizedPrompt)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"prompt_ready": "true",
		},
		Semantic: core.StepSemanticAudit{
			Retained:            appendNonEmpty(nil, "messages", "rag_fragments", "citations"),
			Reasons:             appendNonEmpty(nil, "assemble_final_prompt"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}
