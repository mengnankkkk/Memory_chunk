package processor

import (
	"context"
	"fmt"
	"strings"

	"context-refiner/internal/core"
	"context-refiner/internal/support/heuristic"
)

type JSONTrimProcessor struct {
	counter core.TokenCounter
}

type TableReduceProcessor struct {
	counter core.TokenCounter
}

type CodeOutlineProcessor struct {
	counter core.TokenCounter
}

type ErrorStackFocusProcessor struct {
	counter core.TokenCounter
}

func NewJSONTrimProcessor(counter core.TokenCounter) *JSONTrimProcessor {
	return &JSONTrimProcessor{counter: counter}
}

func NewTableReduceProcessor(counter core.TokenCounter) *TableReduceProcessor {
	return &TableReduceProcessor{counter: counter}
}

func NewCodeOutlineProcessor(counter core.TokenCounter) *CodeOutlineProcessor {
	return &CodeOutlineProcessor{counter: counter}
}

func NewErrorStackFocusProcessor(counter core.TokenCounter) *ErrorStackFocusProcessor {
	return &ErrorStackFocusProcessor{counter: counter}
}

func (p *JSONTrimProcessor) Descriptor() core.ProcessorDescriptor {
	return newStructuredDescriptor("json_trim", 64)
}

func (p *TableReduceProcessor) Descriptor() core.ProcessorDescriptor {
	return newStructuredDescriptor("table_reduce", 64)
}

func (p *CodeOutlineProcessor) Descriptor() core.ProcessorDescriptor {
	return newStructuredDescriptor("code_outline", 96)
}

func (p *ErrorStackFocusProcessor) Descriptor() core.ProcessorDescriptor {
	return newStructuredDescriptor("error_stack_focus", 48)
}

func (p *JSONTrimProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	return runFragmentContentTransform(
		req,
		p.counter,
		core.FragmentTypeJSON,
		trimJSON,
		"json_fragments_trimmed",
		[]string{"top_level_json_keys"},
		[]string{"compact_or_outline_json"},
	)
}

func (p *TableReduceProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	return runFragmentContentTransform(
		req,
		p.counter,
		core.FragmentTypeTable,
		reduceTable,
		"tables_reduced",
		[]string{"header_rows", "sample_rows"},
		[]string{"reduce_large_table_preview"},
	)
}

func (p *CodeOutlineProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	return runFragmentContentTransform(
		req,
		p.counter,
		core.FragmentTypeCode,
		outlineCode,
		"code_fragments_outlined",
		[]string{"type_definitions", "function_signatures"},
		[]string{"extract_code_outline"},
	)
}

func (p *ErrorStackFocusProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	return runFragmentContentTransform(
		req,
		p.counter,
		core.FragmentTypeErrorStack,
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

func newStructuredDescriptor(name string, minTriggerTokens int) core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: name,
		Capabilities: core.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			MinTriggerTokens:    minTriggerTokens,
			PreserveCitation:    true,
		},
	}
}

func runFragmentContentTransform(
	req *core.RefineRequest,
	counter core.TokenCounter,
	targetType core.FragmentType,
	transform func(string) string,
	detailKey string,
	retained []string,
	reasons []string,
) (*core.RefineRequest, core.ProcessResult, error) {
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
	return updated, core.ProcessResult{
		Details: map[string]string{detailKey: fmt.Sprintf("%d", changed)},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("%s=%d", detailKey, changed)),
			Retained:            appendNonEmpty(nil, retained...),
			Reasons:             appendNonEmpty(nil, reasons...),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}
