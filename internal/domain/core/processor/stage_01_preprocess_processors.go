package processor

import (
	"context"
	"fmt"
	"strings"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/components"
	"context-refiner/internal/domain/core/repository"
)

type PagingProcessor struct {
	counter   core.TokenCounter
	store     repository.PageRepository
	pageLimit int
	rag       *components.RAGNormalizer
}

type CollapseProcessor struct {
	counter core.TokenCounter
}

type CompactProcessor struct {
	counter   core.TokenCounter
	sanitizer *components.TextSanitizer
}

type SanitizeProcessor struct {
	counter   core.TokenCounter
	sanitizer *components.TextSanitizer
}

type CanonicalizeProcessor struct {
	counter         core.TokenCounter
	promptComponent *components.PromptComponent
	ragNormalizer   *components.RAGNormalizer
}

func NewPagingProcessor(counter core.TokenCounter, pageStore repository.PageRepository, pageLimit int) *PagingProcessor {
	return &PagingProcessor{
		counter:   counter,
		store:     pageStore,
		pageLimit: pageLimit,
		rag:       components.NewRAGNormalizer(),
	}
}

func NewCollapseProcessor(counter core.TokenCounter) *CollapseProcessor {
	return &CollapseProcessor{counter: counter}
}

func NewCompactProcessor(counter core.TokenCounter) *CompactProcessor {
	return &CompactProcessor{counter: counter, sanitizer: components.NewTextSanitizer()}
}

func NewSanitizeProcessor(counter core.TokenCounter) *SanitizeProcessor {
	return &SanitizeProcessor{
		counter:   counter,
		sanitizer: components.NewTextSanitizer(),
	}
}

func NewCanonicalizeProcessor(counter core.TokenCounter) *CanonicalizeProcessor {
	return &CanonicalizeProcessor{
		counter:         counter,
		promptComponent: components.NewPromptComponent(),
		ragNormalizer:   components.NewRAGNormalizer(),
	}
}

func (p *PagingProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "paging",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:          false,
			Lossy:               false,
			StructuredInputOnly: true,
			MinTriggerTokens:    p.pageLimit + 1,
			PreserveCitation:    true,
		},
	}
}

func (p *CollapseProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "collapse",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *CompactProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "compact",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
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

func (p *CanonicalizeProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "canonicalize",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:       false,
			Lossy:            false,
			PreserveCitation: true,
		},
	}
}

func (p *PagingProcessor) Process(ctx context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	paged := 0
	updated := cloneRequest(req)
	pageCount := 0

	for i, chunk := range updated.RAGChunks {
		if p.counter.CountChunk(chunk) <= p.pageLimit {
			continue
		}
		pages := paginateChunk(p.counter, p.rag, chunk, p.pageLimit)
		pageRefs := make([]string, 0, len(pages))
		canonicalChunk := core.ChunkText(core.StableRAGChunks([]core.RAGChunk{chunk})[0])
		chunkHash := hashText(canonicalChunk)
		for idx, page := range pages {
			key := contentAddressedPageKey(chunk, chunkHash, idx+1)
			if err := p.store.SavePage(ctx, key, core.FragmentsText(page)); err != nil {
				return nil, core.ProcessResult{}, err
			}
			pageRefs = append(pageRefs, key)
			pageCount++
		}
		updated.RAGChunks[i].Fragments = append([]core.RAGFragment(nil), pages[0]...)
		updated.RAGChunks[i].PageRefs = pageRefs
		paged++
	}

	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"paged_chunks": fmt.Sprintf("%d", paged),
			"page_count":   fmt.Sprintf("%d", pageCount),
		},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("paged_chunks=%d", paged)),
			Retained:            appendNonEmpty(nil, "first_page_fragments", "page_refs", "all_sources"),
			Reasons:             appendNonEmpty(nil, "page_out_large_chunks"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *CollapseProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	seen := make(map[string]int)
	filtered := make([]core.RAGChunk, 0, len(updated.RAGChunks))
	removed := 0
	mergedSources := 0

	for _, chunk := range updated.RAGChunks {
		key := strings.TrimSpace(normalizeWhitespace(core.ChunkText(chunk)))
		if idx, ok := seen[key]; ok {
			removed++
			before := len(filtered[idx].Sources)
			filtered[idx].Sources = uniqueStrings(append(filtered[idx].Sources, joinSources(chunk)...))
			if len(filtered[idx].Sources) > before {
				mergedSources += len(filtered[idx].Sources) - before
			}
			continue
		}
		seen[key] = len(filtered)
		chunk.Sources = uniqueStrings(joinSources(chunk))
		if len(chunk.Fragments) == 0 && strings.TrimSpace(key) != "" {
			chunk.Fragments = []core.RAGFragment{{Type: core.FragmentTypeBody, Content: key}}
		}
		filtered = append(filtered, chunk)
	}
	updated.RAGChunks = filtered

	for i, msg := range updated.Messages {
		updated.Messages[i].Content = strings.TrimSpace(normalizeWhitespace(msg.Content))
	}

	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"deduped_chunks": fmt.Sprintf("%d", removed),
			"merged_sources": fmt.Sprintf("%d", mergedSources),
		},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("duplicate_chunks=%d", removed)),
			Retained:            appendNonEmpty(nil, "canonical_chunk_content", "merged_citations"),
			Reasons:             appendNonEmpty(nil, "collapse_duplicate_rag_chunks"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
			DroppedCitations:    0,
		},
	}, nil
}

func (p *CompactProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	charDelta := 0

	for i, msg := range updated.Messages {
		after := p.sanitizer.Sanitize(msg.Content, components.TextSanitizerProfileCompactLayout).Text
		charDelta += len(msg.Content) - len(after)
		updated.Messages[i].Content = after
	}
	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			after := p.sanitizer.Sanitize(fragment.Content, components.TextSanitizerProfileCompactLayout).Text
			charDelta += len(fragment.Content) - len(after)
			updated.RAGChunks[i].Fragments[j].Content = after
		}
	}

	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"removed_chars": fmt.Sprintf("%d", charDelta),
		},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("chars=%d", charDelta)),
			Retained:            appendNonEmpty(nil, "fragment_order", "all_sources"),
			Reasons:             appendNonEmpty(nil, "remove_redundant_blank_lines"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *SanitizeProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	sanitizedItems := 0
	removedChars := 0
	ruleHits := map[string]struct{}{}

	for i, msg := range updated.Messages {
		if isActiveTurnMessage(i, len(updated.Messages), msg.Role) {
			continue
		}
		result := p.sanitizer.Sanitize(msg.Content, components.TextSanitizerProfileRichText)
		if result.Text == msg.Content {
			continue
		}
		updated.Messages[i].Content = result.Text
		sanitizedItems++
		removedChars += result.Report.RemovedChars
		mergeRuleHits(ruleHits, result.Report.AppliedRules)
	}

	for i, chunk := range updated.RAGChunks {
		for j, fragment := range chunk.Fragments {
			if !sanitizeEligible(fragment.Type) {
				continue
			}
			result := p.sanitizer.Sanitize(fragment.Content, components.TextSanitizerProfileRichText)
			if result.Text == fragment.Content {
				continue
			}
			updated.RAGChunks[i].Fragments[j].Content = result.Text
			sanitizedItems++
			removedChars += result.Report.RemovedChars
			mergeRuleHits(ruleHits, result.Report.AppliedRules)
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

func (p *CanonicalizeProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	before := len(updated.RAGChunks)
	preprocessedChunks := 0
	changedFragments := 0
	promotedTitles := 0
	removedBoilerplate := 0
	createdSections := 0
	ruleHits := map[string]struct{}{}

	for i, chunk := range updated.RAGChunks {
		result := p.ragNormalizer.PreprocessChunk(toComponentChunk(chunk))
		updated.RAGChunks[i] = fromComponentChunk(result.Chunk)
		preprocessedChunks += result.Report.ProcessedChunks
		changedFragments += result.Report.ChangedFragments
		promotedTitles += result.Report.PromotedTitles
		removedBoilerplate += result.Report.RemovedBoilerplateLines
		createdSections += result.Report.CreatedSectionFragments
		for _, rule := range result.Report.AppliedRules {
			ruleHits[rule] = struct{}{}
		}
	}

	systemMessages, memoryMessages, activeMessages := p.promptComponent.StablePromptSegments(toPromptMessages(updated.Messages))
	updated.Messages = append(append(fromPromptMessages(systemMessages), fromPromptMessages(memoryMessages)...), fromPromptMessages(activeMessages)...)
	updated.RAGChunks = fromComponentChunks(p.ragNormalizer.StableChunks(toComponentChunks(updated.RAGChunks)))
	for i, chunk := range updated.RAGChunks {
		updated.RAGChunks[i].Sources = p.ragNormalizer.StableSources(chunk.Sources, chunk.Source)
	}
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]string)
	}
	updated.Metadata["canonicalized_rag"] = "true"
	updated.Metadata["canonicalized_messages"] = "true"
	updated.Metadata["normalization_version"] = "stable-prefix-v2"
	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, core.ProcessResult{
		Details: map[string]string{
			"canonicalized_chunks":      fmt.Sprintf("%d", before),
			"preprocessed_chunks":       fmt.Sprintf("%d", preprocessedChunks),
			"changed_fragments":         fmt.Sprintf("%d", changedFragments),
			"promoted_titles":           fmt.Sprintf("%d", promotedTitles),
			"removed_boilerplate":       fmt.Sprintf("%d", removedBoilerplate),
			"created_section_fragments": fmt.Sprintf("%d", createdSections),
			"rule_hits":                 strings.Join(sortedRuleHits(ruleHits), ","),
			"stable_system_messages":    fmt.Sprintf("%d", len(systemMessages)),
			"stable_memory_messages":    fmt.Sprintf("%d", len(memoryMessages)),
			"active_messages":           fmt.Sprintf("%d", len(activeMessages)),
		},
		Semantic: core.StepSemanticAudit{
			Retained:            appendNonEmpty(nil, "rag_ordering", "sources", "fragments", "message_roles", "message_content"),
			Reasons:             appendNonEmpty(nil, "stabilize_prompt_prefix_for_cache_hits", "normalize_high_churn_fields", "preserve_heading_boundaries", "remove_document_boilerplate"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func paginateChunk(counter core.TokenCounter, rag *components.RAGNormalizer, chunk core.RAGChunk, pageLimit int) [][]core.RAGFragment {
	pages := make([][]core.RAGFragment, 0)
	current := make([]core.RAGFragment, 0)
	currentTokens := 0

	for _, fragment := range chunk.Fragments {
		splitParts := splitFragmentByTokenBudget(counter, rag, fragment, pageLimit)
		for _, part := range splitParts {
			partTokens := counter.CountFragment(part)
			if currentTokens > 0 && currentTokens+partTokens > pageLimit {
				pages = append(pages, append([]core.RAGFragment(nil), current...))
				current = current[:0]
				currentTokens = 0
			}
			current = append(current, part)
			currentTokens += partTokens
		}
	}
	if len(current) > 0 {
		pages = append(pages, append([]core.RAGFragment(nil), current...))
	}
	if len(pages) == 0 {
		return [][]core.RAGFragment{{}}
	}
	return pages
}

func contentAddressedPageKey(chunk core.RAGChunk, contentHash string, pageIndex int) string {
	sourceKey := stableArtifactKeyParts(joinSources(chunk)...)
	if sourceKey == "" {
		sourceKey = "unknown"
	}
	return fmt.Sprintf(
		"artifact:v1:rag:sources:%s:hash:%s:page:%d",
		sourceKey,
		contentHash,
		pageIndex,
	)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
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

func mergeRuleHits(dst map[string]struct{}, hits []string) {
	for _, hit := range hits {
		if strings.TrimSpace(hit) == "" {
			continue
		}
		dst[hit] = struct{}{}
	}
}
