package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"context-refiner/internal/store"
)

var (
	codeSymbolRE = regexp.MustCompile(`(?i)^\s*(func|type|class|interface|struct|enum|impl|pub|private|protected|package|import|const|var)\b`)
	stackFrameRE = regexp.MustCompile(`^\s*(at\s+|#\d+\s+|[A-Za-z0-9_./$-]+:\d+|[A-Za-z0-9_.$-]+\()`)
)

type Worker struct {
	consumer     store.SummaryJobConsumer
	pageStore    store.PageStore
	group        string
	consumerName string
	batchSize    int64
	blockTimeout time.Duration
}

func NewWorker(consumer store.SummaryJobConsumer, pageStore store.PageStore, group string, consumerName string, batchSize int64, blockTimeout time.Duration) *Worker {
	if strings.TrimSpace(group) == "" {
		group = "context-refiner-summary"
	}
	if strings.TrimSpace(consumerName) == "" {
		consumerName = "worker-1"
	}
	if batchSize <= 0 {
		batchSize = 8
	}
	if blockTimeout <= 0 {
		blockTimeout = 2 * time.Second
	}
	return &Worker{
		consumer:     consumer,
		pageStore:    pageStore,
		group:        group,
		consumerName: consumerName,
		batchSize:    batchSize,
		blockTimeout: blockTimeout,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.consumer.EnsureSummaryGroup(ctx, w.group); err != nil {
		return fmt.Errorf("ensure summary group failed: %w", err)
	}

	for {
		if ctx.Err() != nil {
			return nil
		}
		messages, err := w.consumer.ConsumeSummaryJobs(ctx, w.group, w.consumerName, w.batchSize, w.blockTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("consume summary jobs failed: %w", err)
		}
		if len(messages) == 0 {
			continue
		}
		for _, message := range messages {
			if err := w.handleJob(ctx, message); err != nil {
				return err
			}
			if err := w.consumer.AckSummaryJob(ctx, w.group, message.ID); err != nil {
				return fmt.Errorf("ack summary job failed: %w", err)
			}
		}
	}
}

func (w *Worker) handleJob(ctx context.Context, message store.SummaryJobMessage) error {
	summaryContent := summarizeJob(message.Job)
	result := store.SummaryResult{
		JobID:     message.Job.JobID,
		Content:   summaryContent,
		CreatedAt: time.Now().UTC(),
	}
	for _, pageRef := range message.Job.PageRefs {
		if err := w.pageStore.SaveSummary(ctx, pageRef, result); err != nil {
			return fmt.Errorf("save summary result failed: %w", err)
		}
	}
	return nil
}

func summarizeJob(job store.SummaryJob) string {
	if len(job.Fragments) == 0 {
		return summarizePlainText(job.Content)
	}
	parts := make([]string, 0, len(job.Fragments)+2)
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
	sentences := splitSentences(text)
	if len(sentences) <= 3 {
		return text
	}
	return strings.Join(sentences[:3], " ")
}

func summarizeCode(text string, language string) string {
	lines := strings.Split(text, "\n")
	symbols := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if codeSymbolRE.MatchString(line) {
			symbols = appendNonEmpty(symbols, line)
		}
	}
	if len(symbols) == 0 {
		symbols = appendNonEmpty(symbols, firstNonEmptyLine(lines))
	}
	if len(symbols) > 8 {
		symbols = symbols[:8]
	}
	header := "Code Outline"
	if strings.TrimSpace(language) != "" {
		header += " [" + language + "]"
	}
	return header + ":\n" + strings.Join(symbols, "\n")
}

func summarizeTable(text string) string {
	lines := nonEmptyLines(text)
	if len(lines) <= 4 {
		return "Table:\n" + strings.Join(lines, "\n")
	}
	head := lines[:min(4, len(lines))]
	tail := ""
	if len(lines) > 4 {
		tail = fmt.Sprintf("\n... %d more rows omitted ...", len(lines)-4)
	}
	return "Table Preview:\n" + strings.Join(head, "\n") + tail
}

func summarizeJSON(text string) string {
	var data any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return summarizePlainText(text)
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
		return fmt.Sprintf("JSON Scalar: %v", value)
	}
}

func summarizeToolOutput(text string) string {
	lines := nonEmptyLines(text)
	selected := make([]string, 0)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "warn") || strings.Contains(lower, "fail") || strings.Contains(lower, "success") {
			selected = appendNonEmpty(selected, line)
		}
	}
	selected = appendNonEmpty(selected, firstNonEmptyLine(lines))
	selected = appendNonEmpty(selected, lastNonEmptyLine(lines))
	selected = uniqueOrdered(selected)
	if len(selected) > 6 {
		selected = selected[:6]
	}
	return "Tool Output Summary:\n" + strings.Join(selected, "\n")
}

func summarizeLog(text string) string {
	lines := nonEmptyLines(text)
	selected := make([]string, 0)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "warn") || strings.Contains(lower, "panic") || strings.Contains(lower, "timeout") {
			selected = appendNonEmpty(selected, line)
		}
	}
	selected = appendNonEmpty(selected, firstNonEmptyLine(lines))
	selected = appendNonEmpty(selected, lastNonEmptyLine(lines))
	selected = uniqueOrdered(selected)
	if len(selected) > 8 {
		selected = selected[:8]
	}
	return "Log Summary:\n" + strings.Join(selected, "\n")
}

func summarizeErrorStack(text string) string {
	lines := nonEmptyLines(text)
	selected := make([]string, 0)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "exception") || strings.Contains(lower, "caused by") {
			selected = appendNonEmpty(selected, line)
		}
		if stackFrameRE.MatchString(line) && len(selected) < 8 {
			selected = appendNonEmpty(selected, line)
		}
	}
	selected = uniqueOrdered(selected)
	if len(selected) > 10 {
		selected = selected[:10]
	}
	return "Error Stack Focus:\n" + strings.Join(selected, "\n")
}

func summarizePlainText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := nonEmptyLines(text)
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

func splitSentences(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func nonEmptyLines(text string) []string {
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

func firstNonEmptyLine(lines []string) string {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func lastNonEmptyLine(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
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

func appendNonEmpty(items []string, values ...string) []string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
