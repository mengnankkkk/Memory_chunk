package core

import "context-refiner/internal/domain/core/components"

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
