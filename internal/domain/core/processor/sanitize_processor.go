package processor

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/components"
)

type SanitizeProcessor struct {
	counter   core.TokenCounter
	sanitizer *components.TextSanitizer
}

func NewSanitizeProcessor(counter core.TokenCounter) *SanitizeProcessor {
	return &SanitizeProcessor{
		counter:   counter,
		sanitizer: components.NewTextSanitizer(),
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

func sortedRuleHits(hits map[string]struct{}) []string {
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
