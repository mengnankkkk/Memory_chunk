package processor

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"

	"context-refiner/internal/engine"
)

func cloneRequest(req *engine.RefineRequest) *engine.RefineRequest {
	updated := *req
	updated.Messages = append([]engine.Message(nil), req.Messages...)
	updated.RAGChunks = make([]engine.RAGChunk, 0, len(req.RAGChunks))
	for _, chunk := range req.RAGChunks {
		cloned := chunk
		cloned.Sources = append([]string(nil), chunk.Sources...)
		cloned.PageRefs = append([]string(nil), chunk.PageRefs...)
		cloned.Fragments = append([]engine.RAGFragment(nil), chunk.Fragments...)
		updated.RAGChunks = append(updated.RAGChunks, cloned)
	}
	updated.Audits = append([]engine.StepAudit(nil), req.Audits...)
	updated.PendingSummaryJobIDs = append([]string(nil), req.PendingSummaryJobIDs...)
	if req.Metadata != nil {
		updated.Metadata = make(map[string]string, len(req.Metadata))
		for k, v := range req.Metadata {
			updated.Metadata[k] = v
		}
	}
	return &updated
}

func normalizeWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func tokenSplitText(counter engine.TokenCounter, text string, maxTokens int) []string {
	text = strings.TrimSpace(text)
	if text == "" || maxTokens <= 0 || counter.CountText(text) <= maxTokens {
		return []string{text}
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 1 {
		return splitLinesByToken(counter, lines, maxTokens)
	}
	return splitRunesByToken(counter, text, maxTokens)
}

func splitLinesByToken(counter engine.TokenCounter, lines []string, maxTokens int) []string {
	parts := make([]string, 0)
	var current strings.Builder
	for _, line := range lines {
		candidate := strings.TrimSpace(line)
		if candidate == "" {
			candidate = line
		}
		if current.Len() == 0 {
			current.WriteString(candidate)
		} else {
			next := current.String() + "\n" + candidate
			if counter.CountText(next) > maxTokens {
				parts = appendNonEmpty(parts, current.String())
				if counter.CountText(candidate) > maxTokens {
					parts = append(parts, splitRunesByToken(counter, candidate, maxTokens)...)
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
	}
	if current.Len() > 0 {
		parts = appendNonEmpty(parts, current.String())
	}
	return parts
}

func splitRunesByToken(counter engine.TokenCounter, text string, maxTokens int) []string {
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

func splitFragmentByTokens(counter engine.TokenCounter, fragment engine.RAGFragment, maxTokens int) []engine.RAGFragment {
	if maxTokens <= 0 || counter.CountFragment(fragment) <= maxTokens {
		return []engine.RAGFragment{fragment}
	}
	parts := tokenSplitText(counter, fragment.Content, maxTokens)
	fragments := make([]engine.RAGFragment, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		fragments = append(fragments, engine.RAGFragment{
			Type:     fragment.Type,
			Content:  part,
			Language: fragment.Language,
		})
	}
	return fragments
}

func appendNonEmpty(items []string, values ...string) []string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			items = append(items, value)
		}
	}
	return items
}

func preserveFlags(chunk engine.RAGChunk) (bool, bool) {
	codeFence := false
	errorStack := false
	for _, fragment := range chunk.Fragments {
		if fragment.Type == engine.FragmentTypeCode || strings.Contains(fragment.Content, "```") {
			codeFence = true
		}
		if fragment.Type == engine.FragmentTypeErrorStack {
			errorStack = true
		}
	}
	return codeFence, errorStack
}

func joinSources(chunk engine.RAGChunk) []string {
	if len(chunk.Sources) > 0 {
		return append([]string(nil), chunk.Sources...)
	}
	if strings.TrimSpace(chunk.Source) != "" {
		return []string{chunk.Source}
	}
	return nil
}

func safeRuneLen(text string) int {
	return utf8.RuneCountInString(text)
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum[:6])
}
