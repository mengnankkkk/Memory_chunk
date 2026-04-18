package mapper

import (
	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/domain/core"
	"context-refiner/internal/dto"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func MapRefineDomainResponseToDTO(resp *core.RefineResponse) *dto.RefineResponse {
	return &dto.RefineResponse{
		OptimizedPrompt:      resp.OptimizedPrompt,
		System:               mapDomainResponseSystem(resp),
		Messages:             mapDomainResponseMessages(resp),
		Memory:               dto.Memory{RAGChunks: mapDomainResponseChunks(resp)},
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
		System:               resp.System,
		Messages:             mapDTOResponseMessages(resp.Messages),
		Memory:               mapDTOResponseMemory(resp.Memory),
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
			Key:             item.Key,
			Content:         item.Content,
			IsSummary:       item.IsSummary,
			SummaryJobId:    item.SummaryJobID,
			SummaryArtifact: mapDTOSummaryArtifact(item.SummaryArtifact),
		})
	}
	return &refinerv1.PageInResponse{Pages: pages}
}

func mapDTOSummaryArtifact(item *dto.SummaryArtifact) *refinerv1.SummaryArtifact {
	if item == nil {
		return nil
	}
	out := &refinerv1.SummaryArtifact{
		ArtifactId:      item.ArtifactID,
		JobId:           item.JobID,
		SessionId:       item.SessionID,
		RequestId:       item.RequestID,
		Policy:          item.Policy,
		ChunkId:         item.ChunkID,
		Source:          item.Source,
		PageRefs:        append([]string(nil), item.PageRefs...),
		ContentHash:     item.ContentHash,
		SummaryText:     item.SummaryText,
		FragmentTypes:   append([]string(nil), item.FragmentTypes...),
		Provider:        item.Provider,
		ProviderVersion: item.ProviderVersion,
		SchemaVersion:   item.SchemaVersion,
	}
	if !item.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(item.CreatedAt)
	}
	if !item.ExpiresAt.IsZero() {
		out.ExpiresAt = timestamppb.New(item.ExpiresAt)
	}
	return out
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

func mapDomainResponseSystem(resp *core.RefineResponse) string {
	if resp == nil {
		return ""
	}
	parts := make([]string, 0)
	for _, item := range resp.Messages {
		if normalizeMessageRole(item.Role) != "system" {
			continue
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		parts = append(parts, content)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func mapDomainResponseMessages(resp *core.RefineResponse) []dto.Message {
	if resp == nil {
		return nil
	}
	messages := make([]dto.Message, 0, len(resp.Messages))
	for _, item := range resp.Messages {
		if normalizeMessageRole(item.Role) == "system" {
			continue
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		messages = append(messages, dto.Message{
			Role:    strings.TrimSpace(item.Role),
			Content: content,
		})
	}
	return messages
}

func mapDomainResponseChunks(resp *core.RefineResponse) []dto.RAGChunk {
	if resp == nil {
		return nil
	}
	chunks := make([]dto.RAGChunk, 0, len(resp.RAGChunks))
	for _, item := range resp.RAGChunks {
		chunks = append(chunks, dto.RAGChunk{
			ID:        strings.TrimSpace(item.ID),
			Source:    strings.TrimSpace(item.Source),
			Sources:   append([]string(nil), item.Sources...),
			Fragments: mapDomainResponseFragments(item.Fragments),
		})
	}
	return chunks
}

func mapDomainResponseFragments(items []core.RAGFragment) []dto.RAGFragment {
	if len(items) == 0 {
		return nil
	}
	fragments := make([]dto.RAGFragment, 0, len(items))
	for _, item := range items {
		fragments = append(fragments, dto.RAGFragment{
			Type:     string(item.Type),
			Content:  strings.TrimSpace(item.Content),
			Language: strings.TrimSpace(item.Language),
		})
	}
	return fragments
}

func mapDTOResponseMessages(items []dto.Message) []*refinerv1.Message {
	if len(items) == 0 {
		return nil
	}
	messages := make([]*refinerv1.Message, 0, len(items))
	for _, item := range items {
		messages = append(messages, &refinerv1.Message{
			Role:    item.Role,
			Content: item.Content,
		})
	}
	return messages
}

func mapDTOResponseMemory(memory dto.Memory) *refinerv1.Memory {
	if len(memory.RAGChunks) == 0 {
		return nil
	}
	return &refinerv1.Memory{RagChunks: mapDTOResponseChunks(memory.RAGChunks)}
}

func mapDTOResponseChunks(items []dto.RAGChunk) []*refinerv1.RagChunk {
	if len(items) == 0 {
		return nil
	}
	chunks := make([]*refinerv1.RagChunk, 0, len(items))
	for _, item := range items {
		chunks = append(chunks, &refinerv1.RagChunk{
			Id:        item.ID,
			Source:    item.Source,
			Fragments: mapDTOResponseFragments(item.Fragments),
			Sources:   append([]string(nil), item.Sources...),
		})
	}
	return chunks
}

func mapDTOResponseFragments(items []dto.RAGFragment) []*refinerv1.RagFragment {
	if len(items) == 0 {
		return nil
	}
	fragments := make([]*refinerv1.RagFragment, 0, len(items))
	for _, item := range items {
		fragments = append(fragments, &refinerv1.RagFragment{
			Type:     mapFragmentTypeToProto(item.Type),
			Content:  item.Content,
			Language: item.Language,
		})
	}
	return fragments
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
