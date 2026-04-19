package core

import (
	"strings"

	"context-refiner/internal/domain/core/components"
)

var (
	defaultTextSanitizer   = components.NewTextSanitizer()
	defaultRAGNormalizer   = components.NewRAGNormalizer()
	defaultPromptComponent = components.NewPromptComponent()
)

func NormalizeTextContent(value string, segment string) string {
	profile := components.TextSanitizerProfileStableText
	if segment == "active_turn" {
		profile = components.TextSanitizerProfileActiveTurn
	}
	return defaultTextSanitizer.Sanitize(value, profile).Text
}

func normalizeWhitespace(value string) string {
	return components.NormalizeWhitespace(value)
}

func toComponentChunk(chunk RAGChunk) components.RAGChunk {
	return components.RAGChunk{
		ID:        chunk.ID,
		Source:    chunk.Source,
		Sources:   append([]string(nil), chunk.Sources...),
		Fragments: toComponentFragments(chunk.Fragments),
		PageRefs:  append([]string(nil), chunk.PageRefs...),
	}
}

func toComponentChunks(chunks []RAGChunk) []components.RAGChunk {
	out := make([]components.RAGChunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, toComponentChunk(chunk))
	}
	return out
}

func fromComponentChunk(chunk components.RAGChunk) RAGChunk {
	return RAGChunk{
		ID:        chunk.ID,
		Source:    chunk.Source,
		Sources:   append([]string(nil), chunk.Sources...),
		Fragments: fromComponentFragments(chunk.Fragments),
		PageRefs:  append([]string(nil), chunk.PageRefs...),
	}
}

func fromComponentChunks(chunks []components.RAGChunk) []RAGChunk {
	out := make([]RAGChunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, fromComponentChunk(chunk))
	}
	return out
}

func toComponentFragment(fragment RAGFragment) components.RAGFragment {
	return components.RAGFragment{
		Type:     string(fragment.Type),
		Content:  fragment.Content,
		Language: fragment.Language,
	}
}

func toComponentFragments(fragments []RAGFragment) []components.RAGFragment {
	out := make([]components.RAGFragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, toComponentFragment(fragment))
	}
	return out
}

func fromComponentFragments(fragments []components.RAGFragment) []RAGFragment {
	out := make([]RAGFragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, RAGFragment{
			Type:     FragmentType(fragment.Type),
			Content:  fragment.Content,
			Language: fragment.Language,
		})
	}
	return out
}

func toPromptMessages(messages []Message) []components.PromptMessage {
	out := make([]components.PromptMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, components.PromptMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return out
}

func fromPromptMessages(messages []components.PromptMessage) []Message {
	out := make([]Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, Message{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return out
}

func buildRAGSection(title string, chunks []RAGChunk) string {
	if len(chunks) == 0 {
		return ""
	}
	lines := []string{title, "## RAG"}
	for _, chunk := range chunks {
		lines = append(lines, defaultPromptComponent.RenderChunk(toComponentChunk(chunk)))
	}
	return NormalizeTextContent(joinSectionLines(lines...), "rag")
}

func buildMessageSection(title string, messages []Message) string {
	if len(messages) == 0 {
		return ""
	}
	lines := []string{title}
	for _, message := range messages {
		lines = append(lines, defaultPromptComponent.RenderMessage(components.PromptMessage{
			Role:    message.Role,
			Content: message.Content,
		}))
	}
	return NormalizeTextContent(joinSectionLines(lines...), "memory")
}

func joinSectionLines(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(part))
	}
	return strings.Join(out, "\n")
}
