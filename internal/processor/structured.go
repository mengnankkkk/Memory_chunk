package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"context-refiner/internal/engine"
)

var (
	codeOutlineRE = regexp.MustCompile(`(?i)^\s*(package|import|func|type|class|interface|struct|enum|impl|pub|private|protected|const|var)\b`)
	errorSignalRE = regexp.MustCompile(`(?i)(error|exception|caused by|panic|fatal)`)
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
	return engine.ProcessorDescriptor{
		Name: "json_trim",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			MinTriggerTokens:    64,
			PreserveCitation:    true,
		},
	}
}

func (p *TableReduceProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "table_reduce",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			MinTriggerTokens:    64,
			PreserveCitation:    true,
		},
	}
}

func (p *CodeOutlineProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "code_outline",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			MinTriggerTokens:    96,
			PreserveCitation:    true,
		},
	}
}

func (p *ErrorStackFocusProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "error_stack_focus",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			MinTriggerTokens:    48,
			PreserveCitation:    true,
		},
	}
}

func (p *JSONTrimProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	changed := 0
	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			if fragment.Type != engine.FragmentTypeJSON {
				continue
			}
			next := trimJSON(fragment.Content)
			if next == fragment.Content {
				continue
			}
			updated.RAGChunks[i].Fragments[j].Content = next
			changed++
		}
	}
	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, engine.ProcessResult{
		Details: map[string]string{"json_fragments_trimmed": fmt.Sprintf("%d", changed)},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("json_fragments_trimmed=%d", changed)),
			Retained:            appendNonEmpty(nil, "top_level_json_keys"),
			Reasons:             appendNonEmpty(nil, "compact_or_outline_json"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *TableReduceProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	changed := 0
	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			if fragment.Type != engine.FragmentTypeTable {
				continue
			}
			next := reduceTable(fragment.Content)
			if next == fragment.Content {
				continue
			}
			updated.RAGChunks[i].Fragments[j].Content = next
			changed++
		}
	}
	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, engine.ProcessResult{
		Details: map[string]string{"tables_reduced": fmt.Sprintf("%d", changed)},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("tables_reduced=%d", changed)),
			Retained:            appendNonEmpty(nil, "header_rows", "sample_rows"),
			Reasons:             appendNonEmpty(nil, "reduce_large_table_preview"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *CodeOutlineProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	changed := 0
	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			if fragment.Type != engine.FragmentTypeCode {
				continue
			}
			next := outlineCode(fragment.Content)
			if next == fragment.Content {
				continue
			}
			updated.RAGChunks[i].Fragments[j].Content = next
			changed++
		}
	}
	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, engine.ProcessResult{
		Details: map[string]string{"code_fragments_outlined": fmt.Sprintf("%d", changed)},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("code_fragments_outlined=%d", changed)),
			Retained:            appendNonEmpty(nil, "type_definitions", "function_signatures"),
			Reasons:             appendNonEmpty(nil, "extract_code_outline"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *ErrorStackFocusProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	changed := 0
	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			if fragment.Type != engine.FragmentTypeErrorStack {
				continue
			}
			next := focusErrorStack(fragment.Content)
			if next == fragment.Content {
				continue
			}
			updated.RAGChunks[i].Fragments[j].Content = next
			changed++
		}
	}
	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, engine.ProcessResult{
		Details: map[string]string{"error_stacks_focused": fmt.Sprintf("%d", changed)},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("error_stacks_focused=%d", changed)),
			Retained:            appendNonEmpty(nil, "error_message", "top_frames", "causes"),
			Reasons:             appendNonEmpty(nil, "focus_error_stack_signal"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func trimJSON(content string) string {
	var data any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return content
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(content)); err == nil && buf.Len() > 0 && buf.Len() < len(content) {
		content = buf.String()
	}
	if len(content) < 320 {
		return content
	}
	switch value := data.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > 12 {
			keys = keys[:12]
		}
		return "JSON Object Keys: " + strings.Join(keys, ", ")
	case []any:
		return fmt.Sprintf("JSON Array Length: %d", len(value))
	default:
		return content
	}
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
	lines := strings.Split(content, "\n")
	outline := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if codeOutlineRE.MatchString(trimmed) {
			outline = appendNonEmpty(outline, trimmed)
		}
	}
	outline = uniqueOrdered(outline)
	if len(outline) == 0 {
		return content
	}
	if len(outline) > 12 {
		outline = outline[:12]
	}
	return "Code Outline:\n" + strings.Join(outline, "\n")
}

func focusErrorStack(content string) string {
	lines := strings.Split(content, "\n")
	selected := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if errorSignalRE.MatchString(trimmed) || strings.HasPrefix(trimmed, "at ") || strings.HasPrefix(trimmed, "#") {
			selected = append(selected, trimmed)
		}
	}
	selected = uniqueOrdered(selected)
	if len(selected) == 0 {
		return content
	}
	if len(selected) > 10 {
		selected = selected[:10]
	}
	return "Error Stack Focus:\n" + strings.Join(selected, "\n")
}

func uniqueOrdered(values []string) []string {
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
