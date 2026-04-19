package processor

import (
	"crypto/sha256"
	"fmt"
	"sort"
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

func cloneRequest(req *core.RefineRequest) *core.RefineRequest {
	updated := *req
	updated.Messages = append([]core.Message(nil), req.Messages...)
	updated.RAGChunks = make([]core.RAGChunk, 0, len(req.RAGChunks))
	for _, chunk := range req.RAGChunks {
		cloned := chunk
		cloned.Sources = append([]string(nil), chunk.Sources...)
		cloned.PageRefs = append([]string(nil), chunk.PageRefs...)
		cloned.Fragments = append([]core.RAGFragment(nil), chunk.Fragments...)
		updated.RAGChunks = append(updated.RAGChunks, cloned)
	}
	updated.Audits = append([]core.StepAudit(nil), req.Audits...)
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

func toComponentChunk(chunk core.RAGChunk) components.RAGChunk {
	componentChunk := components.RAGChunk{
		ID:       chunk.ID,
		Source:   chunk.Source,
		Sources:  append([]string(nil), chunk.Sources...),
		PageRefs: append([]string(nil), chunk.PageRefs...),
	}
	componentChunk.Fragments = toComponentFragments(chunk.Fragments)
	return componentChunk
}

func fromComponentChunk(chunk components.RAGChunk) core.RAGChunk {
	coreChunk := core.RAGChunk{
		ID:       chunk.ID,
		Source:   chunk.Source,
		Sources:  append([]string(nil), chunk.Sources...),
		PageRefs: append([]string(nil), chunk.PageRefs...),
	}
	coreChunk.Fragments = fromComponentFragments(chunk.Fragments)
	return coreChunk
}

func toComponentChunks(chunks []core.RAGChunk) []components.RAGChunk {
	out := make([]components.RAGChunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, toComponentChunk(chunk))
	}
	return out
}

func fromComponentChunks(chunks []components.RAGChunk) []core.RAGChunk {
	out := make([]core.RAGChunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, fromComponentChunk(chunk))
	}
	return out
}

func toComponentFragments(fragments []core.RAGFragment) []components.RAGFragment {
	out := make([]components.RAGFragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, components.RAGFragment{
			Type:     string(fragment.Type),
			Content:  fragment.Content,
			Language: fragment.Language,
		})
	}
	return out
}

func fromComponentFragments(fragments []components.RAGFragment) []core.RAGFragment {
	out := make([]core.RAGFragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, core.RAGFragment{
			Type:     core.FragmentType(fragment.Type),
			Content:  fragment.Content,
			Language: fragment.Language,
		})
	}
	return out
}

func toPromptMessages(messages []core.Message) []components.PromptMessage {
	out := make([]components.PromptMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, components.PromptMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return out
}

func fromPromptMessages(messages []components.PromptMessage) []core.Message {
	out := make([]core.Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, core.Message{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return out
}

func sortedRuleHits(hits map[string]struct{}) []string {
	if len(hits) == 0 {
		return nil
	}
	out := make([]string, 0, len(hits))
	for hit := range hits {
		out = append(out, hit)
	}
	sort.Strings(out)
	return out
}
