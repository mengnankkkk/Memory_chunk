package processor

import (
	"context"
	"fmt"
	"strings"

	"context-refiner/internal/engine"
	"context-refiner/internal/store"
)

type PagingProcessor struct {
	counter   engine.TokenCounter
	store     store.PageStore
	pageLimit int
}

func NewPagingProcessor(counter engine.TokenCounter, pageStore store.PageStore, pageLimit int) *PagingProcessor {
	return &PagingProcessor{
		counter:   counter,
		store:     pageStore,
		pageLimit: pageLimit,
	}
}

func (p *PagingProcessor) Descriptor() engine.ProcessorDescriptor {
	return engine.ProcessorDescriptor{
		Name: "paging",
		Capabilities: engine.ProcessorCapabilities{
			Aggressive:          false,
			Lossy:               false,
			StructuredInputOnly: true,
			MinTriggerTokens:    p.pageLimit + 1,
			PreserveCitation:    true,
		},
	}
}

func (p *PagingProcessor) Process(ctx context.Context, req *engine.RefineRequest) (*engine.RefineRequest, engine.ProcessResult, error) {
	paged := 0
	updated := cloneRequest(req)
	pageCount := 0

	for i, chunk := range updated.RAGChunks {
		if p.counter.CountChunk(chunk) <= p.pageLimit {
			continue
		}
		pages := paginateChunk(p.counter, chunk, p.pageLimit)
		pageRefs := make([]string, 0, len(pages))
		chunkHash := hashText(engine.ChunkText(chunk))
		for idx, page := range pages {
			key := scopedPageKey(updated.SessionID, updated.RequestID, chunk.ID, chunkHash, idx+1)
			if err := p.store.SavePage(ctx, key, engine.FragmentsText(page)); err != nil {
				return nil, engine.ProcessResult{}, err
			}
			pageRefs = append(pageRefs, key)
			pageCount++
		}
		updated.RAGChunks[i].Fragments = append([]engine.RAGFragment(nil), pages[0]...)
		updated.RAGChunks[i].PageRefs = pageRefs
		paged++
	}

	updated.CurrentTokens = p.counter.CountRequest(updated)
	return updated, engine.ProcessResult{
		Details: map[string]string{
			"paged_chunks": fmt.Sprintf("%d", paged),
			"page_count":   fmt.Sprintf("%d", pageCount),
		},
		Semantic: engine.StepSemanticAudit{
			Removed:             appendNonEmpty(nil, fmt.Sprintf("paged_chunks=%d", paged)),
			Retained:            appendNonEmpty(nil, "first_page_fragments", "page_refs", "all_sources"),
			Reasons:             appendNonEmpty(nil, "page_out_large_chunks"),
			SourcePreserved:     true,
			CodeFencePreserved:  true,
			ErrorStackPreserved: true,
		},
	}, nil
}

func paginateChunk(counter engine.TokenCounter, chunk engine.RAGChunk, pageLimit int) [][]engine.RAGFragment {
	pages := make([][]engine.RAGFragment, 0)
	current := make([]engine.RAGFragment, 0)
	currentTokens := 0

	for _, fragment := range chunk.Fragments {
		splitParts := splitFragmentByTokenBudget(counter, fragment, pageLimit)
		for _, part := range splitParts {
			partTokens := counter.CountFragment(part)
			if currentTokens > 0 && currentTokens+partTokens > pageLimit {
				pages = append(pages, append([]engine.RAGFragment(nil), current...))
				current = current[:0]
				currentTokens = 0
			}
			current = append(current, part)
			currentTokens += partTokens
		}
	}
	if len(current) > 0 {
		pages = append(pages, append([]engine.RAGFragment(nil), current...))
	}
	if len(pages) == 0 {
		return [][]engine.RAGFragment{{}}
	}
	return pages
}

func scopedPageKey(sessionID, requestID, chunkID, contentHash string, pageIndex int) string {
	return fmt.Sprintf(
		"session:%s:request:%s:chunk:%s:hash:%s:page:%d",
		sanitizeKeyPart(sessionID),
		sanitizeKeyPart(requestID),
		sanitizeKeyPart(chunkID),
		contentHash,
		pageIndex,
	)
}

func sanitizeKeyPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(" ", "-", ":", "-", "/", "-", "\\", "-", "|", "-")
	return replacer.Replace(value)
}
