package processor

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/components"
)

var chunkMetadataHelper = components.NewChunkMetadataHelper()

func appendNonEmpty(items []string, values ...string) []string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			items = append(items, value)
		}
	}
	return items
}

func preserveFlags(chunk core.RAGChunk) (bool, bool) {
	return chunkMetadataHelper.PreserveFlags(toComponentChunk(chunk))
}

func joinSources(chunk core.RAGChunk) []string {
	return chunkMetadataHelper.JoinSources(toComponentChunk(chunk))
}

func safeRuneLen(text string) int {
	return chunkMetadataHelper.SafeRuneLen(text)
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum[:6])
}

func stableArtifactKeyParts(parts ...string) string {
	return chunkMetadataHelper.StableArtifactKey(parts...)
}
