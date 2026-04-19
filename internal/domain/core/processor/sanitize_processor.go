package processor

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
	stdhtml "html"

	"context-refiner/internal/domain/core"
)

var (
	xmlDeclarationRE = regexp.MustCompile(`(?is)<\?xml[^>]*\?>`)
	xmlDoctypeRE     = regexp.MustCompile(`(?is)<!DOCTYPE[^>]*>`)
	xmlCommentRE     = regexp.MustCompile(`(?is)<!--.*?-->`)
	xmlCDATARE       = regexp.MustCompile(`(?is)<!\[CDATA\[(.*?)\]\]>`)
)

type SanitizeProcessor struct {
	counter core.TokenCounter
}

func NewSanitizeProcessor(counter core.TokenCounter) *SanitizeProcessor {
	return &SanitizeProcessor{counter: counter}
}

func (p *SanitizeProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "sanitize",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            true,
			PreserveCitation: true,
		},
	}
}

func (p *SanitizeProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	sanitizedItems := 0
	removedChars := 0
	ruleHits := map[string]bool{}

	for i, msg := range updated.Messages {
		if isActiveTurnMessage(i, len(updated.Messages), msg.Role) {
			continue
		}
		after, hits := sanitizeRichText(msg.Content)
		if after == msg.Content {
			continue
		}
		updated.Messages[i].Content = after
		sanitizedItems++
		removedChars += len(msg.Content) - len(after)
		mergeRuleHits(ruleHits, hits)
	}

	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			if !sanitizeEligible(fragment.Type) {
				continue
			}
			after, hits := sanitizeRichText(fragment.Content)
			if after == fragment.Content {
				continue
			}
			updated.RAGChunks[i].Fragments[j].Content = after
			sanitizedItems++
			removedChars += len(fragment.Content) - len(after)
			mergeRuleHits(ruleHits, hits)
		}
	}

	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"sanitized_items": fmt.Sprintf("%d", sanitizedItems),
			"removed_chars":   fmt.Sprintf("%d", removedChars),
			"rule_hits":       strings.Join(sortedRuleHits(ruleHits), ","),
		},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("sanitized_items=%d", sanitizedItems), fmt.Sprintf("chars=%d", removedChars)),
			Retained:            appendNonEmpty(nil, "stable_messages", "rag_sources", "code_fragments", "json_fragments", "citations"),
			Reasons:             appendNonEmpty(nil, "strip_html_and_xml_noise", "remove_script_style_and_control_chars", "drop_emoji_noise"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func sanitizeEligible(fragmentType core.FragmentType) bool {
	switch fragmentType {
	case core.FragmentTypeCode, core.FragmentTypeJSON:
		return false
	default:
		return true
	}
}

func isActiveTurnMessage(index, total int, role string) bool {
	return total > 0 && index == total-1 && !strings.EqualFold(strings.TrimSpace(role), "system")
}

func sanitizeRichText(input string) (string, []string) {
	if strings.TrimSpace(input) == "" {
		return input, nil
	}

	hits := map[string]bool{}
	sanitized := strings.ToValidUTF8(strings.ReplaceAll(input, "\r\n", "\n"), "")
	if sanitized != input {
		hits["invalid_utf8"] = true
	}

	sanitized, changed := stripXMLNoise(sanitized)
	if changed {
		hits["xml_noise"] = true
	}

	sanitized, changed = stripHTMLLikeMarkup(sanitized)
	if changed {
		hits["html_markup"] = true
	}

	sanitized, changed = removeUnicodeNoise(sanitized)
	if changed {
		hits["unicode_noise"] = true
	}

	sanitized = normalizeWhitespace(strings.TrimSpace(sanitized))
	return sanitized, sortedRuleHits(hits)
}

func stripXMLNoise(input string) (string, bool) {
	output := xmlDeclarationRE.ReplaceAllString(input, " ")
	output = xmlDoctypeRE.ReplaceAllString(output, " ")
	output = xmlCommentRE.ReplaceAllString(output, " ")
	output = xmlCDATARE.ReplaceAllString(output, "$1")
	return output, output != input
}

func stripHTMLLikeMarkup(input string) (string, bool) {
	if !strings.Contains(input, "<") || !strings.Contains(input, ">") {
		return input, false
	}

	tokenizer := xhtml.NewTokenizer(strings.NewReader(input))
	var builder strings.Builder
	var stack []string

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case xhtml.ErrorToken:
			if builder.Len() == 0 {
				return input, false
			}
			return strings.TrimSpace(builder.String()), true
		case xhtml.TextToken:
			if len(stack) > 0 {
				continue
			}
			text := strings.TrimSpace(stdhtml.UnescapeString(string(tokenizer.Text())))
			if text == "" {
				continue
			}
			writeSanitizedText(&builder, text)
		case xhtml.StartTagToken:
			token := tokenizer.Token()
			if isScriptLikeTag(token.Data) {
				stack = append(stack, strings.ToLower(token.Data))
			}
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
		case xhtml.EndTagToken:
			token := tokenizer.Token()
			if len(stack) > 0 && stack[len(stack)-1] == strings.ToLower(token.Data) {
				stack = stack[:len(stack)-1]
			}
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
		case xhtml.SelfClosingTagToken, xhtml.CommentToken, xhtml.DoctypeToken:
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
		}
	}
}

func isScriptLikeTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "script", "style":
		return true
	default:
		return false
	}
}

func writeSanitizedText(builder *strings.Builder, text string) {
	if builder.Len() > 0 {
		last := builder.String()[builder.Len()-1]
		if last != ' ' && last != '\n' {
			builder.WriteByte(' ')
		}
	}
	builder.WriteString(text)
}

func removeUnicodeNoise(input string) (string, bool) {
	changed := false
	output := strings.Map(func(r rune) rune {
		if shouldDropRune(r) {
			changed = true
			return -1
		}
		return r
	}, input)
	return output, changed
}

func shouldDropRune(r rune) bool {
	switch r {
	case '\n', '\t':
		return false
	}
	if unicode.Is(unicode.C, r) {
		return true
	}
	if isEmojiRune(r) {
		return true
	}
	return false
}

func isEmojiRune(r rune) bool {
	switch {
	case r >= 0x1F1E6 && r <= 0x1F1FF:
		return true
	case r >= 0x1F300 && r <= 0x1F5FF:
		return true
	case r >= 0x1F600 && r <= 0x1F64F:
		return true
	case r >= 0x1F680 && r <= 0x1F6FF:
		return true
	case r >= 0x1F900 && r <= 0x1F9FF:
		return true
	case r >= 0x1FA70 && r <= 0x1FAFF:
		return true
	case r >= 0x2600 && r <= 0x26FF:
		return true
	case r >= 0x2700 && r <= 0x27BF:
		return true
	case r >= 0xFE00 && r <= 0xFE0F:
		return true
	case r >= 0x1F3FB && r <= 0x1F3FF:
		return true
	case r >= 0xE0020 && r <= 0xE007F:
		return true
	case r == 0x20E3:
		return true
	default:
		return false
	}
}

func mergeRuleHits(dst map[string]bool, hits []string) {
	for _, hit := range hits {
		if strings.TrimSpace(hit) == "" {
			continue
		}
		dst[hit] = true
	}
}

func sortedRuleHits(hits map[string]bool) []string {
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
