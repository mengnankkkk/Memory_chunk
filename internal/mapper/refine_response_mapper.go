package mapper

import (
	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/domain/core"
	"context-refiner/internal/dto"
)

func MapRefineDomainResponseToDTO(resp *core.RefineResponse) *dto.RefineResponse {
	return &dto.RefineResponse{
		OptimizedPrompt:      resp.OptimizedPrompt,
		InputTokens:          resp.InputTokens,
		OutputTokens:         resp.OutputTokens,
		Audits:               mapDomainAudits(resp.Audits),
		PagedChunks:          mapDomainPagedChunks(resp.PagedChunks),
		Metadata:             cloneMetadata(resp.Metadata),
		BudgetMet:            resp.BudgetMet,
		PendingSummaryJobIDs: append([]string(nil), resp.PendingSummaryJobIDs...),
	}
}

func MapRefineDTOToProtoResponse(resp *dto.RefineResponse) *refinerv1.RefineResponse {
	return &refinerv1.RefineResponse{
		OptimizedPrompt:      resp.OptimizedPrompt,
		InputTokens:          int32(resp.InputTokens),
		OutputTokens:         int32(resp.OutputTokens),
		Audits:               mapDTOAudits(resp.Audits),
		PagedChunks:          mapDTOPagedChunks(resp.PagedChunks),
		Metadata:             cloneMetadata(resp.Metadata),
		BudgetMet:            resp.BudgetMet,
		PendingSummaryJobIds: append([]string(nil), resp.PendingSummaryJobIDs...),
	}
}

func MapPageInDTOToProtoResponse(resp *dto.PageInResponse) *refinerv1.PageInResponse {
	pages := make([]*refinerv1.StoredPage, 0, len(resp.Pages))
	for _, item := range resp.Pages {
		pages = append(pages, &refinerv1.StoredPage{
			Key:          item.Key,
			Content:      item.Content,
			IsSummary:    item.IsSummary,
			SummaryJobId: item.SummaryJobID,
		})
	}
	return &refinerv1.PageInResponse{Pages: pages}
}

func mapDomainAudits(items []core.StepAudit) []dto.StepAudit {
	audits := make([]dto.StepAudit, 0, len(items))
	for _, item := range items {
		audits = append(audits, dto.StepAudit{
			Name:         item.Name,
			BeforeTokens: item.BeforeTokens,
			AfterTokens:  item.AfterTokens,
			DurationMS:   item.DurationMS,
			Details:      cloneMetadata(item.Details),
			Capabilities: dto.ProcessorCapabilities{
				Aggressive:          item.Capabilities.Aggressive,
				Lossy:               item.Capabilities.Lossy,
				StructuredInputOnly: item.Capabilities.StructuredInputOnly,
				MinTriggerTokens:    item.Capabilities.MinTriggerTokens,
				PreserveCitation:    item.Capabilities.PreserveCitation,
			},
			Semantic: dto.StepSemanticAudit{
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

func mapDTOAudits(items []dto.StepAudit) []*refinerv1.StepAudit {
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

func mapDomainPagedChunks(items []core.PagedChunk) []dto.PagedChunk {
	paged := make([]dto.PagedChunk, 0, len(items))
	for _, item := range items {
		paged = append(paged, dto.PagedChunk{
			ChunkID:   item.ChunkID,
			PageKeys:  append([]string(nil), item.PageKeys...),
			SessionID: item.SessionID,
			RequestID: item.RequestID,
		})
	}
	return paged
}

func mapDTOPagedChunks(items []dto.PagedChunk) []*refinerv1.PagedChunk {
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

func cloneMetadata(items map[string]string) map[string]string {
	if items == nil {
		return nil
	}
	cloned := make(map[string]string, len(items))
	for key, value := range items {
		cloned[key] = value
	}
	return cloned
}
