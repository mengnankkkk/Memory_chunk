package processor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/repository"
)

type AutoCompactSyncProcessor struct {
	counter core.TokenCounter
}

type AutoCompactAsyncProcessor struct {
	counter core.TokenCounter
	queue   repository.SummaryJobRepository
}

func NewAutoCompactSyncProcessor(counter core.TokenCounter) *AutoCompactSyncProcessor {
	return &AutoCompactSyncProcessor{counter: counter}
}

func NewAutoCompactAsyncProcessor(counter core.TokenCounter, queue repository.SummaryJobRepository) *AutoCompactAsyncProcessor {
	return &AutoCompactAsyncProcessor{
		counter: counter,
		queue:   queue,
	}
}

func (p *AutoCompactSyncProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "auto_compact_sync",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:          false,
			Lossy:               false,
			StructuredInputOnly: true,
			PreserveCitation:    true,
		},
	}
}

func (p *AutoCompactAsyncProcessor) Descriptor() core.ProcessorDescriptor {
	return core.ProcessorDescriptor{
		Name: "auto_compact_async",
		Capabilities: core.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			PreserveCitation:    true,
		},
	}
}

func (p *AutoCompactSyncProcessor) Process(_ context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	threshold := updated.RuntimePolicy.AutoCompactThreshold
	if threshold <= 0 {
		threshold = updated.Budget
	}
	if updated.CurrentTokens <= updated.Budget || updated.CurrentTokens <= threshold {
		return updated, core.ProcessResult{Details: map[string]string{"safe_compacted_chunks": "0"}}, nil
	}

	compacted := 0
	for i, chunk := range updated.RAGChunks {
		nextChunk, changed := p.safeCompactChunk(chunk)
		if !changed {
			continue
		}
		updated.RAGChunks[i] = nextChunk
		compacted++
	}
	updated.CurrentTokens = p.counter.CountRequest(updated)

	return updated, core.ProcessResult{
		Details: map[string]string{
			"safe_compacted_chunks": fmt.Sprintf("%d", compacted),
			"threshold":             fmt.Sprintf("%d", threshold),
		},
		Semantic: core.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("safe_compacted_chunks=%d", compacted)),
			Retained:            appendNonEmpty(nil, "all_sources", "code_fences", "error_stack_frames"),
			Reasons:             appendNonEmpty(nil, "safe_log_and_tool_output_compaction"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *AutoCompactAsyncProcessor) Process(ctx context.Context, req *core.RefineRequest) (*core.RefineRequest, core.ProcessResult, error) {
	updated := cloneRequest(req)
	threshold := updated.RuntimePolicy.AutoCompactThreshold
	if threshold <= 0 {
		threshold = updated.Budget
	}
	if updated.CurrentTokens <= updated.Budget || updated.CurrentTokens <= threshold {
		return updated, core.ProcessResult{Details: map[string]string{"queued_jobs": "0"}}, nil
	}

	jobCount := 0
	for _, chunk := range updated.RAGChunks {
		if p.counter.CountChunk(chunk) < threshold/2 {
			continue
		}
		stableChunk := core.StableRAGChunks([]core.RAGChunk{chunk})[0]
		fragments := make([]repository.SummaryFragment, 0, len(chunk.Fragments))
		for _, fragment := range stableChunk.Fragments {
			fragments = append(fragments, repository.SummaryFragment{
				Type:     string(fragment.Type),
				Content:  fragment.Content,
				Language: fragment.Language,
			})
		}
		job := repository.SummaryJob{
			JobID:         fmt.Sprintf("summary-%s", hashText(core.ChunkText(stableChunk))),
			SessionID:     updated.SessionID,
			RequestID:     updated.RequestID,
			Policy:        updated.Policy,
			ChunkID:       stableChunk.ID,
			Source:        strings.Join(joinSources(stableChunk), ","),
			ContentHash:   hashText(core.ChunkText(stableChunk)),
			PageRefs:      append([]string(nil), stableChunk.PageRefs...),
			Fragments:     fragments,
			Content:       core.ChunkText(stableChunk),
			TargetTokens:  updated.Budget,
			CurrentTokens: updated.CurrentTokens,
			CreatedAt:     time.Now().UTC(),
		}
		if err := p.queue.EnqueueSummaryJob(ctx, job); err != nil {
			return nil, core.ProcessResult{}, err
		}
		updated.PendingSummaryJobIDs = append(updated.PendingSummaryJobIDs, job.JobID)
		jobCount++
	}
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]string)
	}
	updated.Metadata["pending_summary_jobs"] = fmt.Sprintf("%d", len(updated.PendingSummaryJobIDs))

	return updated, core.ProcessResult{
		Details: map[string]string{
			"queued_jobs": fmt.Sprintf("%d", jobCount),
			"threshold":   fmt.Sprintf("%d", threshold),
		},
		Semantic: core.StepSemanticAudit{
			Retained:            appendNonEmpty(nil, "current_prompt_state", "page_refs", "citations"),
			Reasons:             appendNonEmpty(nil, "enqueue_async_summary_for_follow_up_page_in"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *AutoCompactSyncProcessor) safeCompactChunk(chunk core.RAGChunk) (core.RAGChunk, bool) {
	updated := chunk
	changed := false
	for i, fragment := range updated.Fragments {
		if !safeCompactEligible(fragment.Type) {
			continue
		}
		nextContent := safeCompactContent(fragment.Content)
		if nextContent == fragment.Content {
			continue
		}
		updated.Fragments[i].Content = nextContent
		changed = true
	}
	return updated, changed
}

func safeCompactEligible(fragmentType core.FragmentType) bool {
	switch fragmentType {
	case core.FragmentTypeToolOutput, core.FragmentTypeLog, core.FragmentTypeErrorStack:
		return true
	default:
		return false
	}
}

func safeCompactContent(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	var previous string
	blankCount := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if strings.TrimSpace(trimmed) == "" {
			blankCount++
			if blankCount > 1 {
				continue
			}
		} else {
			blankCount = 0
		}
		if trimmed == previous && strings.TrimSpace(trimmed) != "" {
			continue
		}
		out = append(out, trimmed)
		previous = trimmed
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
