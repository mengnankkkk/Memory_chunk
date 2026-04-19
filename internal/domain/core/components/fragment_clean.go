package components

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	codeOutlineRE = regexp.MustCompile(`(?i)^\s*(package|import|func|type|class|interface|struct|enum|impl|pub|private|protected|const|var)\b`)
	errorSignalRE = regexp.MustCompile(`(?i)(error|exception|caused by|panic|fatal)`)
	stackFrameRE  = regexp.MustCompile(`^\s*(at\s+|#\d+\s+|[A-Za-z0-9_./$-]+:\d+|[A-Za-z0-9_.$-]+\()`)
)

func SplitSentences(text string) []string {
	return splitSentenceUnits(text)
}

func NonEmptyLines(text string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func FirstNonEmptyLine(lines []string) string {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func LastNonEmptyLine(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

func UniqueTrimmed(values []string) []string {
	seen := make(map[string]struct{}, len(values))
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

func AppendNonEmpty(items []string, values ...string) []string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}

func CodeOutlineLines(content string, limit int) []string {
	lines := strings.Split(content, "\n")
	outline := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if codeOutlineRE.MatchString(trimmed) {
			outline = AppendNonEmpty(outline, trimmed)
		}
	}
	outline = UniqueTrimmed(outline)
	if limit > 0 && len(outline) > limit {
		outline = outline[:limit]
	}
	return outline
}

func ErrorStackFocusLines(content string, limit int) []string {
	lines := strings.Split(content, "\n")
	selected := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if errorSignalRE.MatchString(trimmed) || strings.HasPrefix(trimmed, "at ") || strings.HasPrefix(trimmed, "#") {
			selected = append(selected, trimmed)
		}
	}
	selected = UniqueTrimmed(selected)
	if limit > 0 && len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func StackTraceLines(content string, limit int) []string {
	lines := NonEmptyLines(content)
	selected := make([]string, 0, len(lines))
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "exception") || strings.Contains(lower, "caused by") {
			selected = AppendNonEmpty(selected, line)
		}
		if stackFrameRE.MatchString(line) && len(selected) < limit {
			selected = AppendNonEmpty(selected, line)
		}
	}
	selected = UniqueTrimmed(selected)
	if limit > 0 && len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func CompactJSON(content string) (string, bool) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(content)); err != nil || buf.Len() == 0 || buf.Len() >= len(content) {
		return content, false
	}
	return buf.String(), true
}

func ParseJSON(content string) (any, error) {
	var data any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func DescribeJSONValue(value any, maxKeys int) string {
	switch item := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if maxKeys > 0 && len(keys) > maxKeys {
			keys = keys[:maxKeys]
		}
		return "JSON Object Keys: " + strings.Join(keys, ", ")
	case []any:
		return fmt.Sprintf("JSON Array Length: %d", len(item))
	default:
		return fmt.Sprintf("JSON Scalar: %v", item)
	}
}
