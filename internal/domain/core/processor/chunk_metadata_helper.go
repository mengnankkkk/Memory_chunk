package processor

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"context-refiner/internal/domain/core"
)

func appendNonEmpty(items []string, values ...string) []string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			items = append(items, value)
		}
	}
	return items
}

func preserveFlags(chunk core.RAGChunk) (bool, bool) {
	codeFence := false
	errorStack := false
	for _, fragment := range chunk.Fragments {
		if fragment.Type == core.FragmentTypeCode || strings.Contains(fragment.Content, "```") {
			codeFence = true
		}
		if fragment.Type == core.FragmentTypeErrorStack {
			errorStack = true
		}
	}
	return codeFence, errorStack
}

func joinSources(chunk core.RAGChunk) []string {
	return core.StableSources(chunk.Sources, chunk.Source)
}

func safeRuneLen(text string) int {
	return utf8.RuneCountInString(text)
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum[:6])
}

func stableArtifactKeyParts(parts ...string) string {
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(strings.ToLower(part))
		if value == "" {
			continue
		}
		items = append(items, sanitizeKeyPart(value))
	}
	sort.Strings(items)
	return strings.Join(items, ":")
}
