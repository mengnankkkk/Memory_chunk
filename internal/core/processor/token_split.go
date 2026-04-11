package processor

import (
	"strings"

	"context-refiner/internal/core"
)

func normalizeWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func splitTextByTokenBudget(counter core.TokenCounter, text string, maxTokens int) []string {
	text = strings.TrimSpace(text)
	if text == "" || maxTokens <= 0 || counter.CountText(text) <= maxTokens {
		return []string{text}
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 1 {
		return splitLinesByTokenBudget(counter, lines, maxTokens)
	}
	return splitRunesByTokenBudget(counter, text, maxTokens)
}

func splitLinesByTokenBudget(counter core.TokenCounter, lines []string, maxTokens int) []string {
	parts := make([]string, 0)
	var current strings.Builder
	for _, line := range lines {
		candidate := strings.TrimSpace(line)
		if candidate == "" {
			candidate = line
		}
		if current.Len() == 0 {
			current.WriteString(candidate)
			continue
		}

		next := current.String() + "\n" + candidate
		if counter.CountText(next) > maxTokens {
			parts = appendNonEmpty(parts, current.String())
			if counter.CountText(candidate) > maxTokens {
				parts = append(parts, splitRunesByTokenBudget(counter, candidate, maxTokens)...)
				current.Reset()
				continue
			}
			current.Reset()
			current.WriteString(candidate)
			continue
		}

		current.WriteString("\n")
		current.WriteString(candidate)
	}
	if current.Len() > 0 {
		parts = appendNonEmpty(parts, current.String())
	}
	return parts
}

func splitRunesByTokenBudget(counter core.TokenCounter, text string, maxTokens int) []string {
	if maxTokens <= 0 || text == "" {
		return []string{text}
	}
	runes := []rune(text)
	parts := make([]string, 0, len(runes)/maxTokens+1)
	for len(runes) > 0 {
		high := len(runes)
		low := 1
		best := 1
		for low <= high {
			mid := (low + high) / 2
			candidate := string(runes[:mid])
			if counter.CountText(candidate) <= maxTokens {
				best = mid
				low = mid + 1
				continue
			}
			high = mid - 1
		}
		parts = append(parts, strings.TrimSpace(string(runes[:best])))
		runes = runes[best:]
	}
	return parts
}

func splitFragmentByTokenBudget(counter core.TokenCounter, fragment core.RAGFragment, maxTokens int) []core.RAGFragment {
	if maxTokens <= 0 || counter.CountFragment(fragment) <= maxTokens {
		return []core.RAGFragment{fragment}
	}
	parts := splitTextByTokenBudget(counter, fragment.Content, maxTokens)
	fragments := make([]core.RAGFragment, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		fragments = append(fragments, core.RAGFragment{
			Type:     fragment.Type,
			Content:  part,
			Language: fragment.Language,
		})
	}
	return fragments
}
