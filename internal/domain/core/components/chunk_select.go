package components

import (
	"sort"
	"strings"
	"unicode/utf8"
)

type ChunkMetadataHelper struct {
	ragNormalizer *RAGNormalizer
}

func NewChunkMetadataHelper() *ChunkMetadataHelper {
	return &ChunkMetadataHelper{ragNormalizer: NewRAGNormalizer()}
}

func (h *ChunkMetadataHelper) PreserveFlags(chunk RAGChunk) (bool, bool) {
	codeFence := false
	errorStack := false
	for _, fragment := range chunk.Fragments {
		if fragment.Type == "code" || strings.Contains(fragment.Content, "```") {
			codeFence = true
		}
		if fragment.Type == "error-stack" {
			errorStack = true
		}
	}
	return codeFence, errorStack
}

func (h *ChunkMetadataHelper) JoinSources(chunk RAGChunk) []string {
	return h.ragNormalizer.StableSources(chunk.Sources, chunk.Source)
}

func (h *ChunkMetadataHelper) SafeRuneLen(text string) int {
	return utf8.RuneCountInString(text)
}

func (h *ChunkMetadataHelper) StableArtifactKey(parts ...string) string {
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(strings.ToLower(part))
		if value == "" {
			continue
		}
		items = append(items, h.SanitizeKeyPart(value))
	}
	sort.Strings(items)
	return strings.Join(items, ":")
}

func (h *ChunkMetadataHelper) SanitizeKeyPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(" ", "-", ":", "-", "/", "-", "\\", "-", "|", "-")
	return replacer.Replace(value)
}
