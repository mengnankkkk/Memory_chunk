package heuristic

import (
	"regexp"
	"strings"
)

var (
	codeOutlineRE = regexp.MustCompile(`(?i)^\s*(package|import|func|type|class|interface|struct|enum|impl|pub|private|protected|const|var)\b`)
	errorSignalRE = regexp.MustCompile(`(?i)(error|exception|caused by|panic|fatal)`)
	stackFrameRE  = regexp.MustCompile(`^\s*(at\s+|#\d+\s+|[A-Za-z0-9_./$-]+:\d+|[A-Za-z0-9_.$-]+\()`)
)

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
