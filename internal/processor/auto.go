package processor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"context-refiner/internal/engine"
	"context-refiner/internal/store"
)

type AutoCompactSyncProcessor struct {
	counter engine.TokenCounter
}

type AutoCompactAsyncProcessor struct {
	counter engine.TokenCounter
	queue   store.SummaryJobQueue
}

func NewAutoCompactSyncProcessor(counter engine.TokenCounter) *AutoCompactSyncProcessor {
	return &AutoCompactSyncProcessor{counter: counter}
}

func NewAutoCompactAsyncProcessor(counter engine.TokenCounter, queue store.SummaryJobQueue) *AutoCompactAsyncProcessor {
	return &AutoCompactAsyncProcessor{
		counter: counter,
		queue:   queue,
	}
}

func (p *AutoCompactSyncProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "auto_compact_sync",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          false,
			Lossy:               false,
			StructuredInputOnly: true,
			PreserveCitation:    true,
		},
	}
}

func (p *AutoCompactAsyncProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "auto_compact_async",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          true,
			Lossy:               true,
			StructuredInputOnly: true,
			PreserveCitation:    true,
		},
	}
}

func (p *AutoCompactSyncProcessor) Process(_ context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	threshold := updated.RuntimePolicy.AutoCompactThreshold
	if threshold <= 0 {
		threshold = updated.Budget
	}
	if updated.CurrentTokens <= updated.Budget || updated.CurrentTokens <= threshold {
		return updated, engine.ProcessResult{Details: map[string]string{"safe_compacted_chunks": "0"}}, nil
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

	return updated, engine.ProcessResult{
		Details: map[string]string{
			"safe_compacted_chunks": fmt.Sprintf("%d", compacted),
			"threshold":             fmt.Sprintf("%d", threshold),
		},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("safe_compacted_chunks=%d", compacted)),
			Retained:            appendNonEmpty(nil, "all_sources", "code_fences", "error_stack_frames"),
			Reasons:             appendNonEmpty(nil, "safe_log_and_tool_output_compaction"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *AutoCompactAsyncProcessor) Process(ctx context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	updated := cloneRequest(req)
	threshold := updated.RuntimePolicy.AutoCompactThreshold
	if threshold <= 0 {
		threshold = updated.Budget
	}
	if updated.CurrentTokens <= updated.Budget || updated.CurrentTokens <= threshold {
		return updated, engine.ProcessResult{Details: map[string]string{"queued_jobs": "0"}}, nil
	}

	jobCount := 0
	for _, chunk := range updated.RAGChunks {
		if p.counter.CountChunk(chunk) < threshold/2 {
			continue
		}
		fragments := make([]store.SummaryFragment, 0, len(chunk.Fragments))
		for _, fragment := range chunk.Fragments {
			fragments = append(fragments, store.SummaryFragment{
				Type:     string(fragment.Type),
				Content:  fragment.Content,
				Language: fragment.Language,
			})
		}
		job := store.SummaryJob{
			JobID:         fmt.Sprintf("%s-%s", updated.RequestID, hashText(engine.ChunkText(chunk))),
			SessionID:     updated.SessionID,
			RequestID:     updated.RequestID,
			Policy:        updated.Policy,
			ChunkID:       chunk.ID,
			Source:        strings.Join(joinSources(chunk), ","),
			ContentHash:   hashText(engine.ChunkText(chunk)),
			PageRefs:      append([]string(nil), chunk.PageRefs...),
			Fragments:     fragments,
			Content:       engine.ChunkText(chunk),
			TargetTokens:  updated.Budget,
			CurrentTokens: updated.CurrentTokens,
			CreatedAt:     time.Now().UTC(),
		}
		if err := p.queue.EnqueueSummaryJob(ctx, job); err != nil {
			return nil, engine.ProcessResult{}, err
		}
		updated.PendingSummaryJobIDs = append(updated.PendingSummaryJobIDs, job.JobID)
		jobCount++
	}
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]string)
	}
	updated.Metadata["pending_summary_jobs"] = fmt.Sprintf("%d", len(updated.PendingSummaryJobIDs))

	return updated, engine.ProcessResult{
		Details: map[string]string{
			"queued_jobs": fmt.Sprintf("%d", jobCount),
			"threshold":   fmt.Sprintf("%d", threshold),
		},
		Semantic: engine.StepSemanticAudit{
			Retained:            appendNonEmpty(nil, "current_prompt_state", "page_refs", "citations"),
			Reasons:             appendNonEmpty(nil, "enqueue_async_summary_for_follow_up_page_in"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func (p *AutoCompactSyncProcessor) safeCompactChunk(chunk engine.RAGChunk) (engine.RAGChunk, bool) {
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

func safeCompactEligible(fragmentType engine.FragmentType) bool {
	switch fragmentType {
	case engine.FragmentTypeToolOutput, engine.FragmentTypeLog, engine.FragmentTypeErrorStack:
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
