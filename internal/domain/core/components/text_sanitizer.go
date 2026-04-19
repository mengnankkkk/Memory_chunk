package components

import (
	"html"
	"regexp"
	"sort"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
)

var (
	blankLinePattern = regexp.MustCompile(`\n{3,}`)
	uuidPattern      = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	hexIDPattern     = regexp.MustCompile(`(?i)\b[0-9a-f]{16,}\b`)
	isoTimePattern   = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}([tT ]\d{2}:\d{2}:\d{2}(\.\d+)?([zZ]|[+-]\d{2}:\d{2})?)?\b`)
	xmlDeclarationRE = regexp.MustCompile(`(?is)<\?xml[^>]*\?>`)
	xmlDoctypeRE     = regexp.MustCompile(`(?is)<!DOCTYPE[^>]*>`)
	xmlCommentRE     = regexp.MustCompile(`(?is)<!--.*?-->`)
	xmlCDATARE       = regexp.MustCompile(`(?is)<!\[CDATA\[(.*?)\]\]>`)
	volatileJSONKeys = map[string]struct{}{
		"request_id":    {},
		"requestid":     {},
		"session_id":    {},
		"sessionid":     {},
		"trace_id":      {},
		"traceid":       {},
		"span_id":       {},
		"spanid":        {},
		"timestamp":     {},
		"created_at":    {},
		"updated_at":    {},
		"correlationid": {},
	}
)

type TextSanitizerProfile string

const (
	TextSanitizerProfileCompactLayout TextSanitizerProfile = "compact_layout"
	TextSanitizerProfileStableText    TextSanitizerProfile = "stable_text"
	TextSanitizerProfileActiveTurn    TextSanitizerProfile = "active_turn"
	TextSanitizerProfileRichText      TextSanitizerProfile = "rich_text"
)

type TextSanitizerReport struct {
	Profile      TextSanitizerProfile
	InputLength  int
	OutputLength int
	RemovedChars int
	AppliedRules []string
}

type TextSanitizerResult struct {
	Text   string
	Report TextSanitizerReport
}

type TextSanitizer struct{}

type textSanitizerStep struct {
	name string
	run  func(string) (string, bool)
}

func NewTextSanitizer() *TextSanitizer {
	return &TextSanitizer{}
}

func (s *TextSanitizer) Sanitize(input string, profile TextSanitizerProfile) TextSanitizerResult {
	steps := sanitizerStepsFor(profile)
	current := input
	rules := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		next, changed := step.run(current)
		if changed {
			rules[step.name] = struct{}{}
		}
		current = next
	}
	return TextSanitizerResult{
		Text: current,
		Report: TextSanitizerReport{
			Profile:      profile,
			InputLength:  len(input),
			OutputLength: len(current),
			RemovedChars: len(input) - len(current),
			AppliedRules: sortedRuleNames(rules),
		},
	}
}

func sanitizerStepsFor(profile TextSanitizerProfile) []textSanitizerStep {
	switch profile {
	case TextSanitizerProfileCompactLayout:
		return []textSanitizerStep{
			{name: "collapse_blank_lines", run: compactLayout},
		}
	case TextSanitizerProfileActiveTurn:
		return []textSanitizerStep{
			{name: "normalize_line_endings", run: normalizeLineEndings},
			{name: "trim_space", run: trimOuterSpace},
			{name: "normalize_whitespace", run: normalizeWhitespaceStep},
		}
	case TextSanitizerProfileStableText:
		return []textSanitizerStep{
			{name: "normalize_line_endings", run: normalizeLineEndings},
			{name: "trim_space", run: trimOuterSpace},
			{name: "normalize_stable_identifiers", run: normalizeStableIdentifiers},
			{name: "normalize_whitespace", run: normalizeWhitespaceStep},
		}
	case TextSanitizerProfileRichText:
		return []textSanitizerStep{
			{name: "normalize_line_endings", run: normalizeLineEndings},
			{name: "drop_invalid_utf8", run: dropInvalidUTF8},
			{name: "strip_xml_noise", run: stripXMLNoise},
			{name: "strip_html_markup", run: stripHTMLLikeMarkup},
			{name: "drop_unicode_noise", run: removeUnicodeNoise},
			{name: "trim_space", run: trimOuterSpace},
			{name: "normalize_whitespace", run: normalizeWhitespaceStep},
		}
	default:
		return []textSanitizerStep{
			{name: "normalize_line_endings", run: normalizeLineEndings},
			{name: "trim_space", run: trimOuterSpace},
		}
	}
}

func NormalizeWhitespace(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			lines[i] = ""
			continue
		}
		lines[i] = strings.Join(strings.Fields(trimmed), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func compactLayout(input string) (string, bool) {
	output := blankLinePattern.ReplaceAllString(input, "\n\n")
	return output, output != input
}

func normalizeLineEndings(input string) (string, bool) {
	output := strings.ReplaceAll(input, "\r\n", "\n")
	return output, output != input
}

func dropInvalidUTF8(input string) (string, bool) {
	output := strings.ToValidUTF8(input, "")
	return output, output != input
}

func trimOuterSpace(input string) (string, bool) {
	output := strings.TrimSpace(input)
	return output, output != input
}

func normalizeWhitespaceStep(input string) (string, bool) {
	output := NormalizeWhitespace(input)
	return output, output != input
}

func normalizeStableIdentifiers(input string) (string, bool) {
	output := isoTimePattern.ReplaceAllString(input, "<timestamp>")
	output = uuidPattern.ReplaceAllString(output, "<uuid>")
	output = hexIDPattern.ReplaceAllStringFunc(output, func(value string) string {
		if strings.HasPrefix(strings.ToLower(value), "0x") {
			return value
		}
		return "<hex-id>"
	})
	output = normalizeKeyLikeValue(output, "request_id", "<request-id>")
	output = normalizeKeyLikeValue(output, "requestId", "<request-id>")
	output = normalizeKeyLikeValue(output, "session_id", "<session-id>")
	output = normalizeKeyLikeValue(output, "trace_id", "<trace-id>")
	output = normalizeKeyLikeValue(output, "traceId", "<trace-id>")
	return output, output != input
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
			text := strings.TrimSpace(html.UnescapeString(string(tokenizer.Text())))
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

func normalizeKeyLikeValue(input string, key string, replacement string) string {
	replacer := regexp.MustCompile(`(?i)(` + regexp.QuoteMeta(key) + `\s*[:=]\s*)([^\s,;]+)`)
	return replacer.ReplaceAllString(input, `${1}`+replacement)
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

func isScriptLikeTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "script", "style":
		return true
	default:
		return false
	}
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

func sortedRuleNames(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
