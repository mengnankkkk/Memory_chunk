package service

import (
	"strings"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/core"

	"github.com/google/uuid"
)

func mapRequest(req *refinerv1.RefineRequest, policy core.RuntimePolicy) *core.RefineRequest {
	sessionID, requestID := normalizeIDs(req)
	model := mapModel(req)
	return &core.RefineRequest{
		SessionID:     sessionID,
		RequestID:     requestID,
		Messages:      mapMessages(req),
		RAGChunks:     mapChunks(req),
		Model:         model,
		Budget:        resolveBudget(req, model, policy),
		Policy:        firstNonEmpty(req.GetPolicy(), policy.Name),
		RuntimePolicy: policy,
		Metadata: map[string]string{
			"policy":                    policy.Name,
			"session_id":                sessionID,
			"request_id":                requestID,
			"prompt_layout_version":     "stable-prefix-v2",
			"normalization_version":     "stable-prefix-v2",
			"artifact_key_version":      "content-addressed-v1",
			"cache_optimization_target": "prefix-reuse-stability",
		},
	}
}

func mapMessages(req *refinerv1.RefineRequest) []core.Message {
	messages := make([]core.Message, 0, len(req.GetMessages()))
	for _, item := range req.GetMessages() {
		messages = append(messages, core.Message{
			Role:    item.GetRole(),
			Content: item.GetContent(),
		})
	}
	return messages
}

func mapChunks(req *refinerv1.RefineRequest) []core.RAGChunk {
	chunks := make([]core.RAGChunk, 0, len(req.GetRagChunks()))
	for _, item := range req.GetRagChunks() {
		chunks = append(chunks, core.RAGChunk{
			ID:        item.GetId(),
			Source:    item.GetSource(),
			Sources:   core.StableSources(mapSources(item), item.GetSource()),
			Fragments: mapFragments(item),
		})
	}
	return core.StableRAGChunks(chunks)
}

func mapSources(chunk *refinerv1.RagChunk) []string {
	sources := append([]string(nil), chunk.GetSources()...)
	if len(sources) == 0 && strings.TrimSpace(chunk.GetSource()) != "" {
		return []string{chunk.GetSource()}
	}
	return sources
}

func mapFragments(chunk *refinerv1.RagChunk) []core.RAGFragment {
	fragments := make([]core.RAGFragment, 0, len(chunk.GetFragments()))
	for _, fragment := range chunk.GetFragments() {
		fragments = append(fragments, core.RAGFragment{
			Type:     mapFragmentType(fragment.GetType()),
			Content:  fragment.GetContent(),
			Language: fragment.GetLanguage(),
		})
	}
	if len(fragments) == 0 && strings.TrimSpace(chunk.GetContent()) != "" {
		return []core.RAGFragment{{
			Type:    core.FragmentTypeBody,
			Content: chunk.GetContent(),
		}}
	}
	return fragments
}

func mapModel(req *refinerv1.RefineRequest) core.ModelConfig {
	modelCfg := req.GetModel()
	if modelCfg == nil {
		return core.ModelConfig{}
	}
	return core.ModelConfig{
		Name:             modelCfg.GetModel(),
		MaxContextTokens: int(modelCfg.GetMaxContextTokens()),
	}
}

func resolveBudget(req *refinerv1.RefineRequest, model core.ModelConfig, policy core.RuntimePolicy) int {
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
