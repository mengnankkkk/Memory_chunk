package summary

import (
	"fmt"
	"strings"

	"context-refiner/internal/heuristic"
	"context-refiner/internal/store"
)

func summarizeJob(job store.SummaryJob) string {
	if len(job.Fragments) == 0 {
		return summarizePlainText(job.Content)
	}
	parts := make([]string, 0, len(job.Fragments)+1)
	if strings.TrimSpace(job.Source) != "" {
		parts = append(parts, "Source: "+job.Source)
	}
	for _, fragment := range job.Fragments {
		if rendered := summarizeFragment(fragment); strings.TrimSpace(rendered) != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func summarizeFragment(fragment store.SummaryFragment) string {
	switch fragment.Type {
	case "title":
		return "Title: " + strings.TrimSpace(fragment.Content)
	case "body":
		return summarizeBody(fragment.Content)
	case "code":
		return summarizeCode(fragment.Content, fragment.Language)
	case "table":
		return summarizeTable(fragment.Content)
	case "json":
		return summarizeJSON(fragment.Content)
	case "tool-output":
		return summarizeToolOutput(fragment.Content)
	case "log":
		return summarizeLog(fragment.Content)
	case "error-stack":
		return summarizeErrorStack(fragment.Content)
	default:
		return summarizePlainText(fragment.Content)
	}
}

func summarizeBody(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	sentences := heuristic.SplitSentences(text)
	if len(sentences) <= 3 {
		return text
	}
	return strings.Join(sentences[:3], " ")
}

func summarizeCode(text string, language string) string {
	lines := heuristic.CodeOutlineLines(text, 8)
	if len(lines) == 0 {
		lines = heuristic.AppendNonEmpty(lines, heuristic.FirstNonEmptyLine(strings.Split(text, "\n")))
	}
	header := "Code Outline"
	if strings.TrimSpace(language) != "" {
		header += " [" + language + "]"
	}
	return header + ":\n" + strings.Join(lines, "\n")
}

func summarizeTable(text string) string {
	lines := heuristic.NonEmptyLines(text)
	if len(lines) <= 4 {
		return "Table:\n" + strings.Join(lines, "\n")
	}
	return "Table Preview:\n" + strings.Join(lines[:4], "\n") + fmt.Sprintf("\n... %d more rows omitted ...", len(lines)-4)
}

func summarizeJSON(text string) string {
	data, err := heuristic.ParseJSON(text)
	if err != nil {
		return summarizePlainText(text)
	}
	return heuristic.DescribeJSONValue(data, 12)
}

func summarizeToolOutput(text string) string {
	lines := heuristic.NonEmptyLines(text)
	selected := make([]string, 0, 6)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "warn") || strings.Contains(lower, "fail") || strings.Contains(lower, "success") {
			selected = heuristic.AppendNonEmpty(selected, line)
		}
	}
	selected = heuristic.AppendNonEmpty(selected, heuristic.FirstNonEmptyLine(lines), heuristic.LastNonEmptyLine(lines))
	selected = heuristic.UniqueTrimmed(selected)
	if len(selected) > 6 {
		selected = selected[:6]
	}
	return "Tool Output Summary:\n" + strings.Join(selected, "\n")
}

func summarizeLog(text string) string {
	lines := heuristic.NonEmptyLines(text)
	selected := make([]string, 0, 8)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "warn") || strings.Contains(lower, "panic") || strings.Contains(lower, "timeout") {
			selected = heuristic.AppendNonEmpty(selected, line)
		}
	}
	selected = heuristic.AppendNonEmpty(selected, heuristic.FirstNonEmptyLine(lines), heuristic.LastNonEmptyLine(lines))
	selected = heuristic.UniqueTrimmed(selected)
	if len(selected) > 8 {
		selected = selected[:8]
	}
	return "Log Summary:\n" + strings.Join(selected, "\n")
}

func summarizeErrorStack(text string) string {
	selected := heuristic.StackTraceLines(text, 10)
	return "Error Stack Focus:\n" + strings.Join(selected, "\n")
}

func summarizePlainText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := heuristic.NonEmptyLines(text)
	if len(lines) == 0 {
		return text
	}
	if len(lines) == 1 {
		return lines[0]
	}
	if len(lines) <= 4 {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:3], "\n") + fmt.Sprintf("\n... %d more lines omitted ...", len(lines)-3)
}
