package processor

import (
	"context"

	"context-refiner/internal/engine"
)

type AssembleProcessor struct {
	counter engine.TokenCounter
}

func NewAssembleProcessor(counter engine.TokenCounter) *AssembleProcessor {
	return &AssembleProcessor{counter: counter}
}

func (p *AssembleProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "assemble",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *AssembleProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	updated.OptimizedPrompt = engine.AssemblePrompt(updated)
	updated.CurrentTokens = p.counter.CountText(updated.OptimizedPrompt)
	return updated, engine.ProcessResult{
		Details: map[string]string{
			"prompt_ready": "true",
		},
		Semantic: engine.StepSemanticAudit{
			Retained:            appendNonEmpty(nil, "messages", "rag_fragments", "citations"),
			Reasons:             appendNonEmpty(nil, "assemble_final_prompt"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}
