package processor

import (
	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/components"
)

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
