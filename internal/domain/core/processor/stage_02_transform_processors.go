package processor

import (
	"context"
	"fmt"
	"strings"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/components"
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

type SnipProcessor struct {
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

func NewSnipProcessor(counter core.TokenCounter) *SnipProcessor {
	return &SnipProcessor{counter: counter}
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

func (p *SnipProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "snip",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			MinTriggerTokens:    48,
			PreserveCitation:    true,
		},
	}
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

func (p *SnipProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
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
		return updated, core.ProcessResult{Details: map[string]string{"snipped_items": "0"}}, nil
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

	return updated, core.ProcessResult{
		Details: map[string]string{
			"snipped_items": fmt.Sprintf("%d", snipped),
			"keep_head":     fmt.Sprintf("%d", keepHead),
			"keep_tail":     fmt.Sprintf("%d", keepTail),
		},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("snipped_chunks=%d", snipped)),
			Retained:            appendNonEmpty(nil, "head_lines", "tail_lines", "citations"),
			Reasons:             appendNonEmpty(nil, "middle_out_trim_for_large_fragments"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
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
	transformer := components.NewFragmentTransformer()
	nextChunks, report := transformer.TransformChunks(toComponentChunks(updated.RAGChunks), string(targetType), transform)
	updated.RAGChunks = fromComponentChunks(nextChunks)
	updated.CurrentTokens = counter.CountRequest(updated)
	return updated, core.ProcessResult{
		Details: map[string]string{detailKey: fmt.Sprintf("%d", report.ChangedFragments)},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("%s=%d", detailKey, report.ChangedFragments)),
			Retained:            appendNonEmpty(nil, retained...),
			Reasons:             appendNonEmpty(nil, reasons...),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *SnipProcessor) snipChunk(chunk core.RAGChunk, keepHead, keepTail int) (core.RAGChunk, bool) {
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

func snipEligible(fragmentType core.FragmentType) bool {
	switch fragmentType {
	case core.FragmentTypeCode, core.FragmentTypeToolOutput, core.FragmentTypeLog, core.FragmentTypeErrorStack, core.FragmentTypeJSON:
		return true
	default:
		return false
	}
}
