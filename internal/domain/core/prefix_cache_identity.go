package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	whitespacePattern = regexp.MustCompile(`\s+`)
	uuidPattern       = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	hexIDPattern      = regexp.MustCompile(`(?i)\b[0-9a-f]{16,}\b`)
	isoTimePattern    = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}([tT ]\d{2}:\d{2}:\d{2}(\.\d+)?([zZ]|[+-]\d{2}:\d{2})?)?\b`)
	volatileJSONKeys  = map[string]struct{}{
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

type PrefixCacheIdentity struct {
	ModelID              string
	StablePrefixPrompt   string
	CombinedPrefixHash   string
	StablePrefixTokens   int
	SystemPrefixPrompt   string
	SystemPrefixHash     string
	SystemPrefixTokens   int
	MemoryPrefixPrompt   string
	MemoryPrefixHash     string
	MemoryPrefixTokens   int
	RAGPrefixPrompt      string
	RAGPrefixHash        string
	RAGPrefixTokens      int
	NormalizationVersion string
}

type PrefixNamespaceConfig struct {
	IncludePolicy bool
	IncludeModel  bool
	IncludeTenant bool
}

type PrefixCachePolicy struct {
	MinStablePrefixTokens int
	MinSegmentCount       int
	DefaultTenant         string
	Namespace             PrefixNamespaceConfig
}

type PrefixSegmentKeys struct {
	System string
	Memory string
	RAG    string
}

type PrefixMissAnalysis struct {
	LookupResult     string
	MissReason       string
	SegmentReason    string
	CurrentSegments  PrefixSegmentKeys
	PreviousSegments PrefixSegmentKeys
}

func AssembleStablePrefix(req *RefineRequest) string {
	if req == nil {
		return ""
	}
	identity := BuildPrefixCacheIdentity(req, nil)
	return identity.StablePrefixPrompt
}

func BuildPrefixCacheIdentity(req *RefineRequest, counter TokenCounter) PrefixCacheIdentity {
	modelID := "unknown"
	if req != nil && strings.TrimSpace(req.Model.Name) != "" {
		modelID = strings.TrimSpace(req.Model.Name)
	}

	systemMessages, memoryMessages, _ := StablePromptSegments(nilSafeMessages(req))
	stableChunks := StableRAGChunks(nilSafeChunks(req))

	systemPrompt := renderMessageSection("# Stable System", systemMessages)
	memoryPrompt := renderMessageSection("# Conversation Memory", memoryMessages)
	ragPrompt := renderRAGSection("# Stable Context", stableChunks)
	conversationPrompt := renderMessageSection("# Conversation Memory", append(append([]Message(nil), systemMessages...), memoryMessages...))
	stablePrompt := strings.TrimSpace(strings.Join(nonEmptyParts(ragPrompt, conversationPrompt), "\n\n"))

	identity := PrefixCacheIdentity{
		ModelID:              modelID,
		StablePrefixPrompt:   stablePrompt,
		SystemPrefixPrompt:   systemPrompt,
		MemoryPrefixPrompt:   memoryPrompt,
		RAGPrefixPrompt:      ragPrompt,
		NormalizationVersion: "stable-prefix-v2",
	}
	if stablePrompt != "" {
		identity.CombinedPrefixHash = hashStrings(modelID, stablePrompt)
	}
	if systemPrompt != "" {
		identity.SystemPrefixHash = hashStrings(modelID, systemPrompt)
	}
	if memoryPrompt != "" {
		identity.MemoryPrefixHash = hashStrings(modelID, memoryPrompt)
	}
	if ragPrompt != "" {
		identity.RAGPrefixHash = hashStrings(modelID, ragPrompt)
	}
	if counter != nil {
		identity.StablePrefixTokens = countIfPresent(counter, stablePrompt)
		identity.SystemPrefixTokens = countIfPresent(counter, systemPrompt)
		identity.MemoryPrefixTokens = countIfPresent(counter, memoryPrompt)
		identity.RAGPrefixTokens = countIfPresent(counter, ragPrompt)
	}
	return identity
}

func BuildPrefixCacheIdentityFromSegments(modelID string, systemPrompt string, memoryPrompt string, ragPrompt string, counter TokenCounter) PrefixCacheIdentity {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		modelID = "unknown"
	}
	systemPrompt = strings.TrimSpace(NormalizeTextContent(systemPrompt, "system"))
	memoryPrompt = strings.TrimSpace(NormalizeTextContent(memoryPrompt, "memory"))
	ragPrompt = strings.TrimSpace(NormalizeTextContent(ragPrompt, "rag"))
	if ragPrompt != "" {
		ragPrompt = strings.TrimSpace(strings.Join(nonEmptyParts("# Stable Context\n## RAG\n"+ragPrompt), "\n\n"))
	}
	if systemPrompt != "" {
		systemPrompt = strings.TrimSpace(strings.Join(nonEmptyParts("# Stable System\n"+systemPrompt), "\n\n"))
	}
	if memoryPrompt != "" {
		memoryPrompt = strings.TrimSpace(strings.Join(nonEmptyParts("# Conversation Memory\n"+memoryPrompt), "\n\n"))
	}
	conversationPrompt := strings.TrimSpace(strings.Join(nonEmptyParts(systemPrompt, memoryPrompt), "\n\n"))
	stablePrompt := strings.TrimSpace(strings.Join(nonEmptyParts(ragPrompt, conversationPrompt), "\n\n"))
	identity := PrefixCacheIdentity{
		ModelID:              modelID,
		StablePrefixPrompt:   stablePrompt,
		SystemPrefixPrompt:   systemPrompt,
		MemoryPrefixPrompt:   memoryPrompt,
		RAGPrefixPrompt:      ragPrompt,
		NormalizationVersion: "stable-prefix-v2",
	}
	if stablePrompt != "" {
		identity.CombinedPrefixHash = hashStrings(modelID, stablePrompt)
	}
	if systemPrompt != "" {
		identity.SystemPrefixHash = hashStrings(modelID, systemPrompt)
	}
	if memoryPrompt != "" {
		identity.MemoryPrefixHash = hashStrings(modelID, memoryPrompt)
	}
	if ragPrompt != "" {
		identity.RAGPrefixHash = hashStrings(modelID, ragPrompt)
	}
	if counter != nil {
		identity.StablePrefixTokens = countIfPresent(counter, stablePrompt)
		identity.SystemPrefixTokens = countIfPresent(counter, systemPrompt)
		identity.MemoryPrefixTokens = countIfPresent(counter, memoryPrompt)
		identity.RAGPrefixTokens = countIfPresent(counter, ragPrompt)
	}
	return identity
}

func NormalizeSourceLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.Fragment = ""
		parsed.RawQuery = ""
		return strings.TrimSpace(parsed.String())
	}
	return normalizeWhitespace(trimmed)
}

func BuildPrefixNamespace(policy string, tenant string, modelID string, cfg PrefixNamespaceConfig) string {
	parts := make([]string, 0, 3)
	if cfg.IncludeTenant {
		parts = append(parts, "tenant="+firstNonBlank(tenant, "global"))
	}
	if cfg.IncludePolicy {
		parts = append(parts, "policy="+firstNonBlank(policy, "default"))
	}
	if cfg.IncludeModel {
		parts = append(parts, "model="+firstNonBlank(modelID, "unknown"))
	}
	if len(parts) == 0 {
		return "global"
	}
	return strings.Join(parts, "|")
}

func StableSegmentCount(identity PrefixCacheIdentity) int {
	count := 0
	if identity.SystemPrefixHash != "" {
		count++
	}
	if identity.MemoryPrefixHash != "" {
		count++
	}
	if identity.RAGPrefixHash != "" {
		count++
	}
	return count
}

func NormalizeFragments(fragments []RAGFragment) []RAGFragment {
	out := make([]RAGFragment, 0, len(fragments))
	for _, fragment := range fragments {
		next := fragment
		next.Language = strings.TrimSpace(strings.ToLower(fragment.Language))
		next.Content = NormalizeFragmentContent(fragment)
		out = append(out, next)
	}
	return out
}

func NormalizeFragmentContent(fragment RAGFragment) string {
	switch fragment.Type {
	case FragmentTypeJSON:
		if normalized, ok := normalizeJSON(fragment.Content); ok {
			return normalized
		}
		return NormalizeTextContent(fragment.Content, "rag")
	case FragmentTypeCode:
		return normalizeCodeContent(fragment.Content)
	default:
		return NormalizeTextContent(fragment.Content, "rag")
	}
}

func NormalizeTextContent(value string, segment string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
	if trimmed == "" {
		return ""
	}
	if segment != "active_turn" {
		trimmed = isoTimePattern.ReplaceAllString(trimmed, "<timestamp>")
		trimmed = uuidPattern.ReplaceAllString(trimmed, "<uuid>")
		trimmed = hexIDPattern.ReplaceAllStringFunc(trimmed, func(value string) string {
			if strings.HasPrefix(strings.ToLower(value), "0x") {
				return value
			}
			return "<hex-id>"
		})
		trimmed = normalizeKeyLikeValue(trimmed, "request_id", "<request-id>")
		trimmed = normalizeKeyLikeValue(trimmed, "requestId", "<request-id>")
		trimmed = normalizeKeyLikeValue(trimmed, "session_id", "<session-id>")
		trimmed = normalizeKeyLikeValue(trimmed, "trace_id", "<trace-id>")
		trimmed = normalizeKeyLikeValue(trimmed, "traceId", "<trace-id>")
	}
	return normalizeWhitespace(trimmed)
}

func hashStrings(parts ...string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\n"))))
}

func renderRAGSection(title string, chunks []RAGChunk) string {
	if len(chunks) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(title)
	builder.WriteString("\n## RAG\n")
	for _, chunk := range chunks {
		builder.WriteString(renderChunk(chunk))
	}
	return strings.TrimSpace(builder.String())
}

func renderMessageSection(title string, messages []Message) string {
	if len(messages) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(title)
	builder.WriteString("\n")
	for _, message := range messages {
		builder.WriteString(renderMessage(message))
	}
	return strings.TrimSpace(builder.String())
}

func nilSafeMessages(req *RefineRequest) []Message {
	if req == nil {
		return nil
	}
	return req.Messages
}

func nilSafeChunks(req *RefineRequest) []RAGChunk {
	if req == nil {
		return nil
	}
	return req.RAGChunks
}

func nonEmptyParts(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func countIfPresent(counter TokenCounter, text string) int {
	if counter == nil || strings.TrimSpace(text) == "" {
		return 0
	}
	return counter.CountText(text)
}

func normalizeRole(value string) string {
	role := strings.ToLower(strings.TrimSpace(value))
	if role == "" {
		return "user"
	}
	return role
}

func normalizeWhitespace(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = whitespacePattern.ReplaceAllString(strings.TrimSpace(line), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func normalizeCodeContent(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func normalizeJSON(value string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return "", true
	}
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return "", false
	}
	payload = stripVolatileJSON(payload)
	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(normalized), true
}

func stripVolatileJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if _, ok := volatileJSONKeys[strings.ToLower(strings.TrimSpace(key))]; ok {
				continue
			}
			out[key] = stripVolatileJSON(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, stripVolatileJSON(item))
		}
		return out
	default:
		return typed
	}
}

func normalizePageRefs(values []string) []string {
	seen := make(map[string]string)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = trimmed
	}
	out := make([]string, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if strings.ToLower(out[i]) > strings.ToLower(out[j]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func normalizeKeyLikeValue(input string, key string, replacement string) string {
	replacer := regexp.MustCompile(`(?i)(` + regexp.QuoteMeta(key) + `\s*[:=]\s*)([^\s,;]+)`)
	return replacer.ReplaceAllString(input, `${1}`+replacement)
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
