package components

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	htmlHeadingPattern      = regexp.MustCompile(`(?is)<h([1-6])[^>]*>(.*?)</h\1>`)
	inlineTagPattern        = regexp.MustCompile(`(?is)<[^>]+>`)
	markdownHeadingPattern  = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.+?)\s*$`)
	numberedHeadingPattern  = regexp.MustCompile(`^\s*((\d+(\.\d+){0,3})|[IVXLC]+)[.)]?\s+.+$`)
	repeatedBlankLineRE     = regexp.MustCompile(`\n{3,}`)
	boilerplateLinePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(table of contents|toc|contents)$`),
		regexp.MustCompile(`(?i)^(skip to content|back to top|on this page|edit this page)$`),
		regexp.MustCompile(`(?i)^(previous|next|home|menu|navigation|breadcrumbs?)$`),
		regexp.MustCompile(`(?i)^copyright\b.*$`),
		regexp.MustCompile(`(?i)^all rights reserved\.?$`),
		regexp.MustCompile(`(?i)^(privacy policy|terms of service|cookie policy)$`),
		regexp.MustCompile(`(?i)^was this page helpful\??$`),
	}
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

type RAGPreprocessReport struct {
	ProcessedChunks         int
	ChangedFragments        int
	OutputFragments         int
	PromotedTitles          int
	RemovedBoilerplateLines int
	CreatedSectionFragments int
	AppliedRules            []string
}

type RAGPreprocessResult struct {
	Chunk  RAGChunk
	Report RAGPreprocessReport
}

type TextTokenCounter func(string) int

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

func (n *RAGNormalizer) PreprocessChunk(chunk RAGChunk) RAGPreprocessResult {
	next := chunk
	applied := map[string]struct{}{}
	changedFragments := 0
	promotedTitles := 0
	removedBoilerplate := 0
	createdSections := 0
	outputFragments := 0
	fragments := make([]RAGFragment, 0, len(chunk.Fragments))

	for _, fragment := range chunk.Fragments {
		preprocessed, report := n.preprocessFragment(fragment)
		if report.ChangedFragments > 0 {
			changedFragments += report.ChangedFragments
		}
		promotedTitles += report.PromotedTitles
		removedBoilerplate += report.RemovedBoilerplateLines
		createdSections += report.CreatedSectionFragments
		outputFragments += len(preprocessed)
		for _, rule := range report.AppliedRules {
			applied[rule] = struct{}{}
		}
		fragments = append(fragments, preprocessed...)
	}

	next.Fragments = fragments
	report := RAGPreprocessReport{
		ChangedFragments:        changedFragments,
		OutputFragments:         outputFragments,
		PromotedTitles:          promotedTitles,
		RemovedBoilerplateLines: removedBoilerplate,
		CreatedSectionFragments: createdSections,
		AppliedRules:            sortedRuleNames(applied),
	}
	if changedFragments > 0 || promotedTitles > 0 || removedBoilerplate > 0 || createdSections > 0 {
		report.ProcessedChunks = 1
	}
	return RAGPreprocessResult{Chunk: next, Report: report}
}

func (n *RAGNormalizer) SplitFragmentByTokenBudget(fragment RAGFragment, maxTokens int, count TextTokenCounter) []RAGFragment {
	content := strings.TrimSpace(fragment.Content)
	if content == "" {
		return nil
	}
	if maxTokens <= 0 || count == nil || count(content) <= maxTokens {
		next := fragment
		next.Content = content
		return []RAGFragment{next}
	}

	if isStructuredFragmentType(fragment.Type) {
		return rebuildFragments(fragment, splitTextByTokenBudget(content, maxTokens, count, false))
	}
	return rebuildFragments(fragment, splitTextByTokenBudget(content, maxTokens, count, true))
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

func (n *RAGNormalizer) preprocessFragment(fragment RAGFragment) ([]RAGFragment, RAGPreprocessReport) {
	switch fragment.Type {
	case "body":
		return n.preprocessBodyFragment(fragment)
	case "title":
		sanitized := n.textSanitizer.Sanitize(fragment.Content, TextSanitizerProfileStableText).Text
		next := fragment
		next.Content = sanitized
		report := RAGPreprocessReport{OutputFragments: 1}
		if sanitized != fragment.Content {
			report.ChangedFragments = 1
			report.ProcessedChunks = 1
			report.AppliedRules = []string{"normalize_title_text"}
		}
		return []RAGFragment{next}, report
	default:
		next := fragment
		next.Content = strings.TrimSpace(fragment.Content)
		return []RAGFragment{next}, RAGPreprocessReport{OutputFragments: 1}
	}
}

func (n *RAGNormalizer) preprocessBodyFragment(fragment RAGFragment) ([]RAGFragment, RAGPreprocessReport) {
	applied := map[string]struct{}{}
	report := RAGPreprocessReport{}
	changed := false

	withHeadingBreaks := injectHTMLHeadingBreaks(fragment.Content)
	if withHeadingBreaks != fragment.Content {
		applied["html_heading_boundaries"] = struct{}{}
		changed = true
	}
	sanitized := n.textSanitizer.Sanitize(withHeadingBreaks, TextSanitizerProfileRichText).Text
	if sanitized != fragment.Content {
		changed = true
	}
	lines := strings.Split(sanitized, "\n")
	lineFrequency := countLineFrequency(lines)

	fragments := make([]RAGFragment, 0, 4)
	bodyParagraphs := make([]string, 0, 4)
	paragraphLines := make([]string, 0, 4)

	flushParagraph := func() {
		if len(paragraphLines) == 0 {
			return
		}
		bodyParagraphs = append(bodyParagraphs, strings.TrimSpace(strings.Join(paragraphLines, " ")))
		paragraphLines = paragraphLines[:0]
	}
	flushBody := func() {
		if len(bodyParagraphs) == 0 {
			return
		}
		content := strings.TrimSpace(strings.Join(bodyParagraphs, "\n\n"))
		bodyParagraphs = bodyParagraphs[:0]
		if content == "" {
			return
		}
		fragments = append(fragments, RAGFragment{
			Type:     "body",
			Content:  content,
			Language: fragment.Language,
		})
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			flushParagraph()
			continue
		}
		if isBoilerplateLine(line, lineFrequency) {
			report.RemovedBoilerplateLines++
			applied["boilerplate_lines"] = struct{}{}
			changed = true
			continue
		}
		if i+1 < len(lines) && isUnderlinedHeadingLine(lines[i+1]) && looksHeadingText(line) {
			flushParagraph()
			flushBody()
			fragments = append(fragments, RAGFragment{
				Type:     "title",
				Content:  strings.TrimSpace(line),
				Language: fragment.Language,
			})
			report.PromotedTitles++
			report.CreatedSectionFragments++
			applied["section_headings"] = struct{}{}
			changed = true
			i++
			continue
		}
		if title, ok := parseHeadingText(line); ok {
			flushParagraph()
			flushBody()
			fragments = append(fragments, RAGFragment{
				Type:     "title",
				Content:  title,
				Language: fragment.Language,
			})
			report.PromotedTitles++
			report.CreatedSectionFragments++
			applied["section_headings"] = struct{}{}
			changed = true
			continue
		}
		paragraphLines = append(paragraphLines, line)
	}

	flushParagraph()
	flushBody()

	if len(fragments) == 0 {
		fallback := strings.TrimSpace(repeatedBlankLineRE.ReplaceAllString(sanitized, "\n\n"))
		if fallback == "" {
			fallback = strings.TrimSpace(fragment.Content)
		}
		if fallback == "" {
			return nil, RAGPreprocessReport{}
		}
		return []RAGFragment{{
				Type:     fragment.Type,
				Content:  fallback,
				Language: fragment.Language,
			}}, RAGPreprocessReport{
				ChangedFragments: report.ChangedFragments,
				OutputFragments:  1,
				AppliedRules:     sortedRuleNames(applied),
			}
	}

	report.OutputFragments = len(fragments)
	if len(fragments) != 1 || fragments[0].Content != strings.TrimSpace(fragment.Content) {
		changed = true
	}
	if changed {
		report.ChangedFragments = 1
	}
	if report.ChangedFragments > 0 || report.PromotedTitles > 0 || report.RemovedBoilerplateLines > 0 {
		report.ProcessedChunks = 1
	}
	report.AppliedRules = sortedRuleNames(applied)
	return fragments, report
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

func splitTextByTokenBudget(text string, maxTokens int, count TextTokenCounter, preferSentences bool) []string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" || maxTokens <= 0 || count == nil || count(text) <= maxTokens {
		return []string{text}
	}
	paragraphs := splitParagraphUnits(text)
	return combineUnitsByBudget(paragraphs, maxTokens, count, preferSentences, "\n\n")
}

func combineUnitsByBudget(units []string, maxTokens int, count TextTokenCounter, preferSentences bool, separator string) []string {
	parts := make([]string, 0, len(units))
	current := make([]string, 0, 4)

	flushCurrent := func() {
		if len(current) == 0 {
			return
		}
		parts = append(parts, strings.TrimSpace(strings.Join(current, separator)))
		current = current[:0]
	}

	for _, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		if len(current) == 0 {
			if count(unit) > maxTokens {
				parts = append(parts, splitOversizedUnit(unit, maxTokens, count, preferSentences)...)
				continue
			}
			current = append(current, unit)
			continue
		}
		candidate := strings.TrimSpace(strings.Join(append(append([]string(nil), current...), unit), separator))
		if count(candidate) <= maxTokens {
			current = append(current, unit)
			continue
		}
		flushCurrent()
		if count(unit) > maxTokens {
			parts = append(parts, splitOversizedUnit(unit, maxTokens, count, preferSentences)...)
			continue
		}
		current = append(current, unit)
	}

	flushCurrent()
	return parts
}

func splitOversizedUnit(unit string, maxTokens int, count TextTokenCounter, preferSentences bool) []string {
	if !preferSentences {
		lines := splitLineUnits(unit)
		if len(lines) > 1 {
			return combineUnitsByBudget(lines, maxTokens, count, false, "\n")
		}
		return splitRunesByTokenBudget(unit, maxTokens, count)
	}
	sentences := splitSentenceUnits(unit)
	if len(sentences) > 1 {
		return combineUnitsByBudget(sentences, maxTokens, count, false, " ")
	}
	lines := splitLineUnits(unit)
	if len(lines) > 1 {
		return combineUnitsByBudget(lines, maxTokens, count, false, "\n")
	}
	return splitRunesByTokenBudget(unit, maxTokens, count)
}

func splitParagraphUnits(text string) []string {
	text = strings.TrimSpace(repeatedBlankLineRE.ReplaceAllString(text, "\n\n"))
	if text == "" {
		return nil
	}
	raw := strings.Split(text, "\n\n")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func splitLineUnits(text string) []string {
	raw := strings.Split(text, "\n")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func splitSentenceUnits(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	out := make([]string, 0, len(runes)/32+1)
	start := 0

	for i, r := range runes {
		if !isSentenceBoundaryRune(r) {
			continue
		}
		if !shouldSplitSentence(runes, i) {
			continue
		}
		part := strings.TrimSpace(string(runes[start : i+1]))
		if part != "" {
			out = append(out, part)
		}
		start = i + 1
	}

	if start < len(runes) {
		tail := strings.TrimSpace(string(runes[start:]))
		if tail != "" {
			out = append(out, tail)
		}
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}

func splitRunesByTokenBudget(text string, maxTokens int, count TextTokenCounter) []string {
	if maxTokens <= 0 || text == "" || count == nil {
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
			if count(strings.TrimSpace(candidate)) <= maxTokens {
				best = mid
				low = mid + 1
				continue
			}
			high = mid - 1
		}
		part := strings.TrimSpace(string(runes[:best]))
		if part != "" {
			parts = append(parts, part)
		}
		runes = runes[best:]
	}
	return parts
}

func rebuildFragments(fragment RAGFragment, parts []string) []RAGFragment {
	out := make([]RAGFragment, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		next := fragment
		next.Content = part
		out = append(out, next)
	}
	return out
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

func injectHTMLHeadingBreaks(input string) string {
	if !strings.Contains(strings.ToLower(input), "<h") {
		return input
	}
	return htmlHeadingPattern.ReplaceAllStringFunc(input, func(match string) string {
		submatches := htmlHeadingPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		level, err := strconv.Atoi(submatches[1])
		if err != nil || level <= 0 {
			level = 1
		}
		title := strings.TrimSpace(html.UnescapeString(inlineTagPattern.ReplaceAllString(submatches[2], " ")))
		if title == "" {
			return "\n"
		}
		title = strings.Join(strings.Fields(title), " ")
		return "\n" + strings.Repeat("#", level) + " " + title + "\n"
	})
}

func countLineFrequency(lines []string) map[string]int {
	out := make(map[string]int, len(lines))
	for _, line := range lines {
		normalized := normalizeLineKey(line)
		if normalized == "" {
			continue
		}
		out[normalized]++
	}
	return out
}

func normalizeLineKey(line string) string {
	line = strings.ToLower(strings.TrimSpace(line))
	line = strings.Join(strings.Fields(line), " ")
	return line
}

func isBoilerplateLine(line string, frequency map[string]int) bool {
	normalized := normalizeLineKey(line)
	if normalized == "" {
		return false
	}
	if looksLowInformationLine(line) && frequency[normalized] > 1 {
		return true
	}
	for _, pattern := range boilerplateLinePatterns {
		if pattern.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}
	if looksBreadcrumbLine(normalized) || looksMenuLine(normalized) {
		return true
	}
	return false
}

func looksLowInformationLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	runes := []rune(line)
	if len(runes) > 120 {
		return false
	}
	if hasTerminalSentencePunctuation(line) {
		return false
	}
	return wordCount(line) <= 12
}

func looksBreadcrumbLine(line string) bool {
	if strings.Count(line, ">") >= 2 {
		return true
	}
	if strings.Count(line, "/") >= 3 && wordCount(line) <= 10 {
		return true
	}
	return false
}

func looksMenuLine(line string) bool {
	if !(strings.Contains(line, "|") || strings.Contains(line, "•")) {
		return false
	}
	return wordCount(line) <= 12
}

func parseHeadingText(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	if matches := markdownHeadingPattern.FindStringSubmatch(line); len(matches) == 3 {
		return strings.TrimSpace(matches[2]), true
	}
	if numberedHeadingPattern.MatchString(line) && wordCount(line) <= 16 {
		return strings.TrimSpace(line), true
	}
	if looksAllCapsHeading(line) {
		return strings.TrimSpace(line), true
	}
	return "", false
}

func looksHeadingText(line string) bool {
	_, ok := parseHeadingText(line)
	return ok || wordCount(line) <= 12
}

func isUnderlinedHeadingLine(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 3 {
		return false
	}
	for _, r := range line {
		if r != '=' && r != '-' {
			return false
		}
	}
	return true
}

func looksAllCapsHeading(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || wordCount(line) > 12 {
		return false
	}
	letters := 0
	uppers := 0
	for _, r := range line {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				uppers++
			}
		}
	}
	return letters >= 3 && uppers == letters
}

func hasTerminalSentencePunctuation(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	last := []rune(trimmed)[len([]rune(trimmed))-1]
	switch last {
	case '.', '!', '?', '。', '！', '？':
		return true
	default:
		return false
	}
}

func wordCount(text string) int {
	return len(strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || r == '|' || r == '>' || r == '/' || r == '•'
	}))
}

func isSentenceBoundaryRune(r rune) bool {
	switch r {
	case '.', '!', '?', ';', '。', '！', '？', '；':
		return true
	default:
		return false
	}
}

func shouldSplitSentence(runes []rune, index int) bool {
	current := runes[index]
	if current == '.' {
		if index > 0 && index+1 < len(runes) && unicode.IsDigit(runes[index-1]) && unicode.IsDigit(runes[index+1]) {
			return false
		}
		token := previousToken(runes, index)
		if isCommonAbbreviation(token) {
			return false
		}
	}
	for index+1 < len(runes) && unicode.IsSpace(runes[index+1]) {
		index++
	}
	if index+1 >= len(runes) {
		return true
	}
	next := runes[index+1]
	return unicode.IsUpper(next) || unicode.IsDigit(next) || isCJKRune(next)
}

func previousToken(runes []rune, index int) string {
	start := index - 1
	for start >= 0 && !unicode.IsSpace(runes[start]) {
		start--
	}
	return strings.ToLower(strings.TrimSpace(string(runes[start+1 : index+1])))
}

func isCommonAbbreviation(token string) bool {
	switch token {
	case "mr.", "mrs.", "ms.", "dr.", "prof.", "sr.", "jr.", "vs.", "etc.", "e.g.", "i.e.":
		return true
	default:
		return false
	}
}

func isCJKRune(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana)
}

func isStructuredFragmentType(fragmentType string) bool {
	switch strings.TrimSpace(fragmentType) {
	case "code", "json", "table":
		return true
	default:
		return false
	}
}
