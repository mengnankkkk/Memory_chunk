package service

import (
	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/core"
)

func mapResponse(resp *core.RefineResponse) *refinerv1.RefineResponse {
	return &refinerv1.RefineResponse{
		OptimizedPrompt:      resp.OptimizedPrompt,
		InputTokens:          int32(resp.InputTokens),
		OutputTokens:         int32(resp.OutputTokens),
		Audits:               mapAudits(resp.Audits),
		PagedChunks:          mapPagedChunks(resp.PagedChunks),
		Metadata:             resp.Metadata,
		BudgetMet:            resp.BudgetMet,
		PendingSummaryJobIds: append([]string(nil), resp.PendingSummaryJobIDs...),
	}
}

func mapAudits(items []core.StepAudit) []*refinerv1.StepAudit {
	audits := make([]*refinerv1.StepAudit, 0, len(items))
	for _, item := range items {
		audits = append(audits, &refinerv1.StepAudit{
			Name:         item.Name,
			BeforeTokens: int32(item.BeforeTokens),
			AfterTokens:  int32(item.AfterTokens),
			DurationMs:   item.DurationMS,
			Details:      item.Details,
			Capabilities: &refinerv1.ProcessorCapabilities{
				Aggressive:          item.Capabilities.Aggressive,
				Lossy:               item.Capabilities.Lossy,
				StructuredInputOnly: item.Capabilities.StructuredInputOnly,
				MinTriggerTokens:    int32(item.Capabilities.MinTriggerTokens),
				PreserveCitation:    item.Capabilities.PreserveCitation,
			},
			Semantic: &refinerv1.StepSemanticAudit{
				Removed:             append([]string(nil), item.Semantic.Removed...),
				Retained:            append([]string(nil), item.Semantic.Retained...),
				Reasons:             append([]string(nil), item.Semantic.Reasons...),
				SourcePreserved:     item.Semantic.SourcePreserved,
				CodeFencePreserved:  item.Semantic.CodeFencePreserved,
				ErrorStackPreserved: item.Semantic.ErrorStackPreserved,
				DroppedCitations:    item.Semantic.DroppedCitations,
			},
		})
	}
	return audits
}

func mapPagedChunks(items []core.PagedChunk) []*refinerv1.PagedChunk {
	paged := make([]*refinerv1.PagedChunk, 0, len(items))
	for _, item := range items {
		paged = append(paged, &refinerv1.PagedChunk{
			ChunkId:   item.ChunkID,
			PageKeys:  append([]string(nil), item.PageKeys...),
			SessionId: item.SessionID,
			RequestId: item.RequestID,
		})
	}
	return paged
}
