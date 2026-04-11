package processor

import (
	"context"

	"context-refiner/internal/core"
)

type AssembleProcessor struct {
	counter core.TokenCounter
}

func NewAssembleProcessor(counter core.TokenCounter) *AssembleProcessor {
	return &AssembleProcessor{counter: counter}
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
	updated.OptimizedPrompt = core.AssemblePrompt(updated)
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
