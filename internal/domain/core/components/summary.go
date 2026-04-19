package components

import (
	"fmt"
	"strings"
)

type SummaryFragment struct {
	Type     string
	Content  string
	Language string
}

type SummaryJob struct {
	Source    string
	Content   string
	Fragments []SummaryFragment
}

type SummaryComponent struct{}

func NewSummaryComponent() *SummaryComponent {
	return &SummaryComponent{}
}

func (c *SummaryComponent) SummarizeJob(job SummaryJob) string {
	if len(job.Fragments) == 0 {
		return c.summarizePlainText(job.Content)
	}
	parts := make([]string, 0, len(job.Fragments)+1)
	if strings.TrimSpace(job.Source) != "" {
		parts = append(parts, "Source: "+job.Source)
	}
	for _, fragment := range job.Fragments {
		if rendered := c.summarizeFragment(fragment); strings.TrimSpace(rendered) != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func (c *SummaryComponent) summarizeFragment(fragment SummaryFragment) string {
	switch fragment.Type {
	case "title":
		return "Title: " + strings.TrimSpace(fragment.Content)
	case "body":
		return c.summarizeBody(fragment.Content)
	case "code":
		return c.summarizeCode(fragment.Content, fragment.Language)
	case "table":
		return c.summarizeTable(fragment.Content)
	case "json":
		return c.summarizeJSON(fragment.Content)
	case "tool-output":
		return c.summarizeToolOutput(fragment.Content)
	case "log":
		return c.summarizeLog(fragment.Content)
	case "error-stack":
		return c.summarizeErrorStack(fragment.Content)
	default:
		return c.summarizePlainText(fragment.Content)
	}
}

func (c *SummaryComponent) summarizeBody(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	sentences := SplitSentences(text)
	if len(sentences) <= 3 {
		return text
	}
	return strings.Join(sentences[:3], " ")
}

func (c *SummaryComponent) summarizeCode(text string, language string) string {
	lines := CodeOutlineLines(text, 8)
	if len(lines) == 0 {
		lines = AppendNonEmpty(lines, FirstNonEmptyLine(strings.Split(text, "\n")))
	}
	header := "Code Outline"
	if strings.TrimSpace(language) != "" {
		header += " [" + language + "]"
	}
	return header + ":\n" + strings.Join(lines, "\n")
}

func (c *SummaryComponent) summarizeTable(text string) string {
	lines := NonEmptyLines(text)
	if len(lines) <= 4 {
		return "Table:\n" + strings.Join(lines, "\n")
	}
	return "Table Preview:\n" + strings.Join(lines[:4], "\n") + fmt.Sprintf("\n... %d more rows omitted ...", len(lines)-4)
}

func (c *SummaryComponent) summarizeJSON(text string) string {
	data, err := ParseJSON(text)
	if err != nil {
		return c.summarizePlainText(text)
	}
	return DescribeJSONValue(data, 12)
}

func (c *SummaryComponent) summarizeToolOutput(text string) string {
	lines := NonEmptyLines(text)
	selected := make([]string, 0, 6)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "warn") || strings.Contains(lower, "fail") || strings.Contains(lower, "success") {
			selected = AppendNonEmpty(selected, line)
		}
	}
	selected = AppendNonEmpty(selected, FirstNonEmptyLine(lines), LastNonEmptyLine(lines))
	selected = UniqueTrimmed(selected)
	if len(selected) > 6 {
		selected = selected[:6]
	}
	return "Tool Output Summary:\n" + strings.Join(selected, "\n")
}

func (c *SummaryComponent) summarizeLog(text string) string {
	lines := NonEmptyLines(text)
	selected := make([]string, 0, 8)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "warn") || strings.Contains(lower, "panic") || strings.Contains(lower, "timeout") {
			selected = AppendNonEmpty(selected, line)
		}
	}
	selected = AppendNonEmpty(selected, FirstNonEmptyLine(lines), LastNonEmptyLine(lines))
	selected = UniqueTrimmed(selected)
	if len(selected) > 8 {
		selected = selected[:8]
	}
	return "Log Summary:\n" + strings.Join(selected, "\n")
}

func (c *SummaryComponent) summarizeErrorStack(text string) string {
	selected := StackTraceLines(text, 10)
	return "Error Stack Focus:\n" + strings.Join(selected, "\n")
}

func (c *SummaryComponent) summarizePlainText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := NonEmptyLines(text)
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
