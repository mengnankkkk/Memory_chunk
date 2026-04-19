package core

import "context-refiner/internal/domain/core/components"

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
