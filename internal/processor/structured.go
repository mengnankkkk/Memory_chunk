package processor

import (
	"context"
	"fmt"
	"strings"

	"context-refiner/internal/engine"
	"context-refiner/internal/heuristic"
)

type JSONTrimProcessor struct {
	counter engine.TokenCounter
}

type TableReduceProcessor struct {
	counter engine.TokenCounter
}

type CodeOutlineProcessor struct {
	counter engine.TokenCounter
}

type ErrorStackFocusProcessor struct {
	counter engine.TokenCounter
}

func NewJSONTrimProcessor(counter engine.TokenCounter) *JSONTrimProcessor {
	return &JSONTrimProcessor{counter: counter}
}

func NewTableReduceProcessor(counter engine.TokenCounter) *TableReduceProcessor {
	return &TableReduceProcessor{counter: counter}
}

func NewCodeOutlineProcessor(counter engine.TokenCounter) *CodeOutlineProcessor {
	return &CodeOutlineProcessor{counter: counter}
}

func NewErrorStackFocusProcessor(counter engine.TokenCounter) *ErrorStackFocusProcessor {
	return &ErrorStackFocusProcessor{counter: counter}
}

func (p *JSONTrimProcessor) Descriptor() engine.ProcessorDescriptor {
	return newStructuredDescriptor("json_trim", 64)
}

func (p *TableReduceProcessor) Descriptor() engine.ProcessorDescriptor {
	return newStructuredDescriptor("table_reduce", 64)
}

func (p *CodeOutlineProcessor) Descriptor() engine.ProcessorDescriptor {
	return newStructuredDescriptor("code_outline", 96)
}

func (p *ErrorStackFocusProcessor) Descriptor() engine.ProcessorDescriptor {
	return newStructuredDescriptor("error_stack_focus", 48)
}

func (p *JSONTrimProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	return runFragmentContentTransform(
		req,
		p.counter,
		engine.FragmentTypeJSON,
		trimJSON,
		"json_fragments_trimmed",
		[]string{"top_level_json_keys"},
		[]string{"compact_or_outline_json"},
	)
}

func (p *TableReduceProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	return runFragmentContentTransform(
		req,
		p.counter,
		engine.FragmentTypeTable,
		reduceTable,
		"tables_reduced",
		[]string{"header_rows", "sample_rows"},
		[]string{"reduce_large_table_preview"},
	)
}

func (p *CodeOutlineProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	return runFragmentContentTransform(
		req,
		p.counter,
		engine.FragmentTypeCode,
		outlineCode,
		"code_fragments_outlined",
		[]string{"type_definitions", "function_signatures"},
		[]string{"extract_code_outline"},
	)
}

func (p *ErrorStackFocusProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	return runFragmentContentTransform(
		req,
		p.counter,
		engine.FragmentTypeErrorStack,
		focusErrorStack,
		"error_stacks_focused",
		[]string{"error_message", "top_frames", "causes"},
		[]string{"focus_error_stack_signal"},
	)
}

func trimJSON(content string) string {
	data, err := heuristic.ParseJSON(content)
	if err != nil {
		return content
	}
	if compacted, ok := heuristic.CompactJSON(content); ok {
		content = compacted
	}
	if len(content) < 320 {
		return content
	}
	summary := heuristic.DescribeJSONValue(data, 12)
	if strings.HasPrefix(summary, "JSON Scalar: ") {
		return content
	}
	return summary
}

func reduceTable(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) <= 5 {
		return content
	}
	headCount := 4
	if len(lines) < headCount {
		headCount = len(lines)
	}
	return strings.Join(lines[:headCount], "\n") + fmt.Sprintf("\n... %d more rows omitted ...", len(lines)-headCount)
}

func outlineCode(content string) string {
	outline := heuristic.CodeOutlineLines(content, 12)
	if len(outline) == 0 {
		return content
	}
	return "Code Outline:\n" + strings.Join(outline, "\n")
}

func focusErrorStack(content string) string {
	selected := heuristic.ErrorStackFocusLines(content, 10)
	if len(selected) == 0 {
		return content
	}
	return "Error Stack Focus:\n" + strings.Join(selected, "\n")
}

func newStructuredDescriptor(name string, minTriggerTokens int) engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: name,
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			MinTriggerTokens:    minTriggerTokens,
			PreserveCitation:    true,
		},
	}
}

func runFragmentContentTransform(
	req *engine.RefineRequest,
	counter engine.TokenCounter,
	targetType engine.FragmentType,
	transform func(string) string,
	detailKey string,
	retained []string,
	reasons []string,
) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	changed := 0
	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			if fragment.Type != targetType {
				continue
			}
			next := transform(fragment.Content)
			if next == fragment.Content {
				continue
			}
			updated.RAGChunks[i].Fragments[j].Content = next
			changed++
		}
	}
	updated.CurrentTokens = counter.CountRequest(updated)
	return updated, engine.ProcessResult{
		Details: map[string]string{detailKey: fmt.Sprintf("%d", changed)},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("%s=%d", detailKey, changed)),
			Retained:            appendNonEmpty(nil, retained...),
			Reasons:             appendNonEmpty(nil, reasons...),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}
