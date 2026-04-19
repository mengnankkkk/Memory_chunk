package components

import (
	"fmt"
	"strings"
)

type PromptMessage struct {
	Role    string
	Content string
}

type PromptComponent struct {
	textSanitizer *TextSanitizer
	ragNormalizer *RAGNormalizer
}

type StablePrefixSections struct {
	SystemPrompt string
	MemoryPrompt string
	RAGPrompt    string
	StablePrompt string
}

func NewPromptComponent() *PromptComponent {
	return &PromptComponent{
		textSanitizer: NewTextSanitizer(),
		ragNormalizer: NewRAGNormalizer(),
	}
}

func (p *PromptComponent) StablePromptMessages(messages []PromptMessage) ([]PromptMessage, []PromptMessage) {
	systemMessages, memoryMessages, activeMessages := p.StablePromptSegments(messages)
	return append(systemMessages, memoryMessages...), activeMessages
}

func (p *PromptComponent) StablePromptSegments(messages []PromptMessage) ([]PromptMessage, []PromptMessage, []PromptMessage) {
	if len(messages) == 0 {
		return nil, nil, nil
	}
	normalized := make([]PromptMessage, 0, len(messages))
	for _, message := range messages {
		segment := "memory"
		if normalizeRole(message.Role) == "system" {
			segment = "system"
		}
		normalized = append(normalized, PromptMessage{
			Role:    normalizeRole(message.Role),
			Content: p.textSanitizer.Sanitize(message.Content, segmentProfile(segment)).Text,
		})
	}
	if len(normalized) == 1 {
		normalized[0].Content = p.textSanitizer.Sanitize(messages[0].Content, TextSanitizerProfileActiveTurn).Text
		return nil, nil, append([]PromptMessage(nil), normalized[0])
	}

	active := append([]PromptMessage(nil), normalized[len(normalized)-1])
	active[0].Content = p.textSanitizer.Sanitize(messages[len(messages)-1].Content, TextSanitizerProfileActiveTurn).Text
	stable := normalized[:len(normalized)-1]
	systemMessages := make([]PromptMessage, 0)
	memoryMessages := make([]PromptMessage, 0)
	for _, message := range stable {
		if message.Role == "system" {
			systemMessages = append(systemMessages, message)
			continue
		}
		memoryMessages = append(memoryMessages, message)
	}
	return systemMessages, memoryMessages, active
}

func (p *PromptComponent) AssemblePrompt(messages []PromptMessage, chunks []RAGChunk) string {
	var builder strings.Builder
	stableChunks := p.ragNormalizer.StableChunks(chunks)
	stableMessages, dynamicMessages := p.StablePromptMessages(messages)
	if len(stableChunks) > 0 {
		builder.WriteString("# Stable Context\n")
		builder.WriteString("## RAG\n")
		for _, chunk := range stableChunks {
			builder.WriteString(p.RenderChunk(chunk))
		}
		builder.WriteString("\n")
	}
	if len(stableMessages) > 0 {
		builder.WriteString("# Conversation Memory\n")
		for _, msg := range stableMessages {
			builder.WriteString(p.RenderMessage(msg))
		}
		builder.WriteString("\n")
	}
	if len(dynamicMessages) > 0 {
		builder.WriteString("# Active Turn\n")
		for _, msg := range dynamicMessages {
			builder.WriteString(p.RenderMessage(msg))
		}
	}
	return strings.TrimSpace(builder.String())
}

func (p *PromptComponent) BuildStablePrefixSections(systemPrompt string, memoryPrompt string, ragPrompt string) StablePrefixSections {
	systemPrompt = strings.TrimSpace(p.textSanitizer.Sanitize(systemPrompt, TextSanitizerProfileStableText).Text)
	memoryPrompt = strings.TrimSpace(p.textSanitizer.Sanitize(memoryPrompt, TextSanitizerProfileStableText).Text)
	ragPrompt = strings.TrimSpace(p.textSanitizer.Sanitize(ragPrompt, TextSanitizerProfileStableText).Text)

	if ragPrompt != "" {
		ragPrompt = joinSectionLines("# Stable Context", "## RAG", ragPrompt)
	}
	if systemPrompt != "" {
		systemPrompt = joinSectionLines("# Stable System", systemPrompt)
	}
	if memoryPrompt != "" {
		memoryPrompt = joinSectionLines("# Conversation Memory", memoryPrompt)
	}

	conversationPrompt := joinSectionLines(systemPrompt, memoryPrompt)
	stablePrompt := joinSectionLines(ragPrompt, conversationPrompt)
	return StablePrefixSections{
		SystemPrompt: systemPrompt,
		MemoryPrompt: memoryPrompt,
		RAGPrompt:    ragPrompt,
		StablePrompt: stablePrompt,
	}
}

func (p *PromptComponent) RenderChunk(chunk RAGChunk) string {
	sourceLabel := strings.Join(chunk.Sources, ", ")
	if sourceLabel == "" {
		sourceLabel = "unknown"
	}
	return fmt.Sprintf("- (%s)\n%s\n", sourceLabel, strings.TrimSpace(p.ChunkText(chunk)))
}

func (p *PromptComponent) RenderMessage(msg PromptMessage) string {
	return fmt.Sprintf("[%s]\n%s\n\n", strings.ToUpper(strings.TrimSpace(msg.Role)), strings.TrimSpace(msg.Content))
}

func (p *PromptComponent) ChunkText(chunk RAGChunk) string {
	return p.FragmentsText(chunk.Fragments)
}

func (p *PromptComponent) FragmentsText(fragments []RAGFragment) string {
	parts := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		rendered := p.FragmentText(fragment)
		if strings.TrimSpace(rendered) != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func (p *PromptComponent) FragmentText(fragment RAGFragment) string {
	content := strings.TrimSpace(fragment.Content)
	if content == "" {
		return ""
	}
	switch fragment.Type {
	case "code":
		return fmt.Sprintf("```%s\n%s\n```", strings.TrimSpace(fragment.Language), content)
	case "table":
		return "Table:\n" + content
	case "json":
		return "JSON:\n" + content
	case "tool-output":
		return "Tool Output:\n" + content
	case "log":
		return "Log:\n" + content
	case "error-stack":
		return "Error Stack:\n" + content
	case "title":
		return "Title: " + content
	default:
		return content
	}
}

func segmentProfile(segment string) TextSanitizerProfile {
	if segment == "active_turn" {
		return TextSanitizerProfileActiveTurn
	}
	return TextSanitizerProfileStableText
}

func normalizeRole(value string) string {
	role := strings.ToLower(strings.TrimSpace(value))
	if role == "" {
		return "user"
	}
	return role
}

func joinSectionLines(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(part))
	}
	return strings.TrimSpace(strings.Join(out, "\n\n"))
}
