package components

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"
)

type RAGFragment struct {
	Type     string
	Content  string
	Language string
}

type RAGChunk struct {
	ID        string
	Source    string
	Sources   []string
	Fragments []RAGFragment
	PageRefs  []string
}

type RAGNormalizationReport struct {
	NormalizedChunks    int
	NormalizedFragments int
	NormalizedSources   int
	NormalizedPageRefs  int
	AppliedNormalizers  []string
}

type RAGNormalizationResult struct {
	Chunk  RAGChunk
	Report RAGNormalizationReport
}

type RAGNormalizer struct {
	textSanitizer *TextSanitizer
}

func NewRAGNormalizer() *RAGNormalizer {
	return &RAGNormalizer{textSanitizer: NewTextSanitizer()}
}

func (n *RAGNormalizer) StableChunks(chunks []RAGChunk) []RAGChunk {
	stable := make([]RAGChunk, 0, len(chunks))
	for _, chunk := range chunks {
		stable = append(stable, n.NormalizeChunk(chunk).Chunk)
	}
	sort.SliceStable(stable, func(i, j int) bool {
		left := stableChunkSortKey(stable[i])
		right := stableChunkSortKey(stable[j])
		if left == right {
			return stableChunkTieKey(stable[i]) < stableChunkTieKey(stable[j])
		}
		return left < right
	})
	return stable
}

func (n *RAGNormalizer) NormalizeChunk(chunk RAGChunk) RAGNormalizationResult {
	next := chunk
	applied := map[string]struct{}{}
	normalizedSources := 0
	normalizedFragments := 0
	normalizedPageRefs := 0

	source := n.NormalizeSourceLabel(chunk.Source)
	if source != chunk.Source {
		normalizedSources++
		applied["source_label"] = struct{}{}
	}
	next.Source = source

	sources := n.StableSources(chunk.Sources, chunk.Source)
	if !equalStrings(sources, chunk.Sources) {
		normalizedSources += countStringDiff(chunk.Sources, sources)
		applied["sources"] = struct{}{}
	}
	next.Sources = sources

	fragments := n.NormalizeFragments(chunk.Fragments)
	if !equalFragments(fragments, chunk.Fragments) {
		normalizedFragments = countFragmentDiff(chunk.Fragments, fragments)
		applied["fragments"] = struct{}{}
	}
	next.Fragments = fragments

	pageRefs := normalizePageRefs(chunk.PageRefs)
	if !equalStrings(pageRefs, chunk.PageRefs) {
		normalizedPageRefs = countStringDiff(chunk.PageRefs, pageRefs)
		applied["page_refs"] = struct{}{}
	}
	next.PageRefs = pageRefs

	report := RAGNormalizationReport{
		NormalizedFragments: normalizedFragments,
		NormalizedSources:   normalizedSources,
		NormalizedPageRefs:  normalizedPageRefs,
		AppliedNormalizers:  sortedRuleNames(applied),
	}
	if len(report.AppliedNormalizers) > 0 {
		report.NormalizedChunks = 1
	}
	return RAGNormalizationResult{Chunk: next, Report: report}
}

func (n *RAGNormalizer) StableSources(values []string, fallback string) []string {
	candidates := append([]string(nil), values...)
	if len(candidates) == 0 && strings.TrimSpace(fallback) != "" {
		candidates = append(candidates, fallback)
	}
	seen := make(map[string]string)
	for _, value := range candidates {
		normalized := n.NormalizeSourceLabel(value)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = normalized
	}
	out := make([]string, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func (n *RAGNormalizer) NormalizeSourceLabel(value string) string {
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
	return NormalizeWhitespace(trimmed)
}

func (n *RAGNormalizer) NormalizeFragments(fragments []RAGFragment) []RAGFragment {
	out := make([]RAGFragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, n.NormalizeFragment(fragment))
	}
	return out
}

func (n *RAGNormalizer) NormalizeFragment(fragment RAGFragment) RAGFragment {
	next := fragment
	next.Language = strings.TrimSpace(strings.ToLower(fragment.Language))
	next.Content = n.NormalizeFragmentContent(fragment)
	return next
}

func (n *RAGNormalizer) NormalizeFragmentContent(fragment RAGFragment) string {
	switch fragment.Type {
	case "json":
		if normalized, ok := n.normalizeJSON(fragment.Content); ok {
			return normalized
		}
		return n.textSanitizer.Sanitize(fragment.Content, TextSanitizerProfileStableText).Text
	case "code":
		return normalizeCodeContent(fragment.Content)
	default:
		return n.textSanitizer.Sanitize(fragment.Content, TextSanitizerProfileStableText).Text
	}
}

func (n *RAGNormalizer) normalizeJSON(value string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return "", true
	}
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return "", false
	}
	payload = n.stripVolatileJSON(payload)
	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(normalized), true
}

func (n *RAGNormalizer) stripVolatileJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if _, ok := volatileJSONKeys[strings.ToLower(strings.TrimSpace(key))]; ok {
				continue
			}
			out[key] = n.stripVolatileJSON(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, n.stripVolatileJSON(item))
		}
		return out
	default:
		return typed
	}
}

func normalizeCodeContent(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
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
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func stableChunkSortKey(chunk RAGChunk) string {
	sourceLabel := strings.Join(chunk.Sources, ",")
	if sourceLabel == "" {
		sourceLabel = strings.TrimSpace(chunk.Source)
	}
	return strings.ToLower(sourceLabel)
}

func stableChunkTieKey(chunk RAGChunk) string {
	id := strings.TrimSpace(chunk.ID)
	if id == "" {
		id = chunkTextForSort(chunk)
	}
	return strings.ToLower(id)
}

func chunkTextForSort(chunk RAGChunk) string {
	var prompt PromptComponent
	return prompt.ChunkText(chunk)
}

func countFragmentDiff(before, after []RAGFragment) int {
	limit := len(before)
	if len(after) > limit {
		limit = len(after)
	}
	diff := 0
	for i := 0; i < limit; i++ {
		var left, right RAGFragment
		if i < len(before) {
			left = before[i]
		}
		if i < len(after) {
			right = after[i]
		}
		if left != right {
			diff++
		}
	}
	return diff
}

func countStringDiff(before, after []string) int {
	limit := len(before)
	if len(after) > limit {
		limit = len(after)
	}
	diff := 0
	for i := 0; i < limit; i++ {
		var left, right string
		if i < len(before) {
			left = before[i]
		}
		if i < len(after) {
			right = after[i]
		}
		if left != right {
			diff++
		}
	}
	return diff
}

func equalFragments(left, right []RAGFragment) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
