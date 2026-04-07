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
