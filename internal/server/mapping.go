package server

import (
	"strings"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/engine"

	"github.com/google/uuid"
)

func mapRequest(req *refinerv1.RefineRequest, policy engine.RuntimePolicy) *engine.RefineRequest {
	sessionID, requestID := normalizeIDs(req)
	model := mapModel(req)
	return &engine.RefineRequest{
		SessionID:     sessionID,
		RequestID:     requestID,
		Messages:      mapMessages(req),
		RAGChunks:     mapChunks(req),
		Model:         model,
		Budget:        resolveBudget(req, model, policy),
		Policy:        firstNonEmpty(req.GetPolicy(), policy.Name),
		RuntimePolicy: policy,
		Metadata: map[string]string{
			"policy":     policy.Name,
			"session_id": sessionID,
			"request_id": requestID,
		},
	}
}

func mapResponse(resp *engine.RefineResponse) *refinerv1.RefineResponse {
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

func mapMessages(req *refinerv1.RefineRequest) []engine.Message {
	messages := make([]engine.Message, 0, len(req.GetMessages()))
	for _, item := range req.GetMessages() {
		messages = append(messages, engine.Message{
			Role:    item.GetRole(),
			Content: item.GetContent(),
		})
	}
	return messages
}

func mapChunks(req *refinerv1.RefineRequest) []engine.RAGChunk {
	chunks := make([]engine.RAGChunk, 0, len(req.GetRagChunks()))
	for _, item := range req.GetRagChunks() {
		chunks = append(chunks, engine.RAGChunk{
			ID:        item.GetId(),
			Source:    item.GetSource(),
			Sources:   mapSources(item),
			Fragments: mapFragments(item),
		})
	}
	return chunks
}

func mapSources(chunk *refinerv1.RagChunk) []string {
	sources := append([]string(nil), chunk.GetSources()...)
	if len(sources) == 0 && strings.TrimSpace(chunk.GetSource()) != "" {
		return []string{chunk.GetSource()}
	}
	return sources
}

func mapFragments(chunk *refinerv1.RagChunk) []engine.RAGFragment {
	fragments := make([]engine.RAGFragment, 0, len(chunk.GetFragments()))
	for _, fragment := range chunk.GetFragments() {
		fragments = append(fragments, engine.RAGFragment{
			Type:     mapFragmentType(fragment.GetType()),
			Content:  fragment.GetContent(),
			Language: fragment.GetLanguage(),
		})
	}
	if len(fragments) == 0 && strings.TrimSpace(chunk.GetContent()) != "" {
		return []engine.RAGFragment{{
			Type:    engine.FragmentTypeBody,
			Content: chunk.GetContent(),
		}}
	}
	return fragments
}

func mapModel(req *refinerv1.RefineRequest) engine.ModelConfig {
	modelCfg := req.GetModel()
	if modelCfg == nil {
		return engine.ModelConfig{}
	}
	return engine.ModelConfig{
		Name:             modelCfg.GetModel(),
		MaxContextTokens: int(modelCfg.GetMaxContextTokens()),
	}
}

func resolveBudget(req *refinerv1.RefineRequest, model engine.ModelConfig, policy engine.RuntimePolicy) int {
	budget := int(req.GetTokenBudget())
	if budget <= 0 && model.MaxContextTokens > 0 && policy.BudgetRatio > 0 {
		return int(float64(model.MaxContextTokens) * policy.BudgetRatio)
	}
	return budget
}

func normalizeIDs(req *refinerv1.RefineRequest) (sessionID string, requestID string) {
	sessionID = strings.TrimSpace(req.GetSessionId())
	requestID = strings.TrimSpace(req.GetRequestId())
	if requestID == "" {
		requestID = uuid.NewString()
	}
	if sessionID == "" {
		sessionID = "session-" + requestID
	}
	return sessionID, requestID
}

func mapAudits(items []engine.StepAudit) []*refinerv1.StepAudit {
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

func mapPagedChunks(items []engine.PagedChunk) []*refinerv1.PagedChunk {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapFragmentType(fragmentType refinerv1.FragmentType) engine.FragmentType {
	switch fragmentType {
	case refinerv1.FragmentType_FRAGMENT_TYPE_TITLE:
		return engine.FragmentTypeTitle
	case refinerv1.FragmentType_FRAGMENT_TYPE_CODE:
		return engine.FragmentTypeCode
	case refinerv1.FragmentType_FRAGMENT_TYPE_TABLE:
		return engine.FragmentTypeTable
	case refinerv1.FragmentType_FRAGMENT_TYPE_JSON:
		return engine.FragmentTypeJSON
	case refinerv1.FragmentType_FRAGMENT_TYPE_TOOL_OUTPUT:
		return engine.FragmentTypeToolOutput
	case refinerv1.FragmentType_FRAGMENT_TYPE_LOG:
		return engine.FragmentTypeLog
	case refinerv1.FragmentType_FRAGMENT_TYPE_ERROR_STACK:
		return engine.FragmentTypeErrorStack
	default:
		return engine.FragmentTypeBody
	}
}
