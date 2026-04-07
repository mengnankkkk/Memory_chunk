package processor

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"

	"context-refiner/internal/engine"
)

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
