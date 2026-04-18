package mapper

import (
	"strings"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/domain/core"
	"context-refiner/internal/dto"

	"github.com/google/uuid"
)

func MapRefineProtoRequestToDTO(req *refinerv1.RefineRequest) *dto.RefineRequest {
	sessionID, requestID := normalizeRefineIDs(req.GetSessionId(), req.GetRequestId())
	return &dto.RefineRequest{
		SessionID:   sessionID,
		RequestID:   requestID,
		System:      mapProtoSystem(req),
		Messages:    mapProtoMessages(req),
		Memory:      dto.Memory{RAGChunks: mapProtoChunks(req)},
		Model:       mapProtoModel(req),
		TokenBudget: int(req.GetTokenBudget()),
		Policy:      strings.TrimSpace(req.GetPolicy()),
	}
}

func MapRefineDTOToDomainRequest(req *dto.RefineRequest, policy core.RuntimePolicy) *core.RefineRequest {
	model := mapDTOModel(req)
	return &core.RefineRequest{
		SessionID:     req.SessionID,
		RequestID:     req.RequestID,
		Messages:      mapDTOMessages(req),
		RAGChunks:     mapDTOChunks(req),
		Model:         model,
		Budget:        resolveBudget(req, model, policy),
		Policy:        firstNonEmpty(req.Policy, policy.Name),
		RuntimePolicy: policy,
		Metadata: map[string]string{
			"policy":                    policy.Name,
			"session_id":                req.SessionID,
			"request_id":                req.RequestID,
			"prompt_layout_version":     "stable-prefix-v2",
			"normalization_version":     "stable-prefix-v2",
			"artifact_key_version":      "content-addressed-v1",
			"cache_optimization_target": "prefix-reuse-stability",
		},
	}
}

func MapPageInProtoRequestToDTO(req *refinerv1.PageInRequest) *dto.PageInRequest {
	return &dto.PageInRequest{
		PageKeys: append([]string(nil), req.GetPageKeys()...),
	}
}

func mapProtoMessages(req *refinerv1.RefineRequest) []dto.Message {
	if req == nil {
		return nil
	}
	messages := make([]dto.Message, 0, len(req.GetMessages()))
	for _, item := range req.GetMessages() {
		messages = append(messages, dto.Message{
			Role:    item.GetRole(),
			Content: item.GetContent(),
		})
	}
	return messages
}

func mapDTOMessages(req *dto.RefineRequest) []core.Message {
	messages := make([]core.Message, 0, len(req.Messages)+1)
	if system := strings.TrimSpace(req.System); system != "" {
		messages = append(messages, core.Message{
			Role:    "system",
			Content: system,
		})
	}
	for _, item := range req.Messages {
		messages = append(messages, core.Message{
			Role:    item.Role,
			Content: item.Content,
		})
	}
	return messages
}

func mapProtoChunks(req *refinerv1.RefineRequest) []dto.RAGChunk {
	protoChunks := protoMemoryChunks(req)
	chunks := make([]dto.RAGChunk, 0, len(protoChunks))
	for _, item := range protoChunks {
		chunks = append(chunks, dto.RAGChunk{
			ID:        item.GetId(),
			Source:    item.GetSource(),
			Sources:   mapProtoSources(item),
			Fragments: mapProtoFragments(item),
		})
	}
	return chunks
}

func mapDTOChunks(req *dto.RefineRequest) []core.RAGChunk {
	chunks := make([]core.RAGChunk, 0, len(req.Memory.RAGChunks))
	for _, item := range req.Memory.RAGChunks {
		chunks = append(chunks, core.RAGChunk{
			ID:        item.ID,
			Source:    item.Source,
			Sources:   core.StableSources(append([]string(nil), item.Sources...), item.Source),
			Fragments: mapDTOFragments(item),
		})
	}
	return core.StableRAGChunks(chunks)
}

func mapProtoSources(chunk *refinerv1.RagChunk) []string {
	sources := append([]string(nil), chunk.GetSources()...)
	if len(sources) == 0 && strings.TrimSpace(chunk.GetSource()) != "" {
		return []string{chunk.GetSource()}
	}
	return sources
}

func mapProtoFragments(chunk *refinerv1.RagChunk) []dto.RAGFragment {
	fragments := make([]dto.RAGFragment, 0, len(chunk.GetFragments()))
	for _, fragment := range chunk.GetFragments() {
		fragments = append(fragments, dto.RAGFragment{
			Type:     mapFragmentTypeFromProto(fragment.GetType()),
			Content:  fragment.GetContent(),
			Language: fragment.GetLanguage(),
		})
	}
	if len(fragments) == 0 && strings.TrimSpace(chunk.GetContent()) != "" {
		return []dto.RAGFragment{{
			Type:    string(core.FragmentTypeBody),
			Content: chunk.GetContent(),
		}}
	}
	return fragments
}

func mapDTOFragments(chunk dto.RAGChunk) []core.RAGFragment {
	fragments := make([]core.RAGFragment, 0, len(chunk.Fragments))
	for _, fragment := range chunk.Fragments {
		fragments = append(fragments, core.RAGFragment{
			Type:     mapFragmentTypeToCore(fragment.Type),
			Content:  fragment.Content,
			Language: fragment.Language,
		})
	}
	return fragments
}

func mapProtoModel(req *refinerv1.RefineRequest) dto.Model {
	modelCfg := req.GetModel()
	if modelCfg == nil {
		return dto.Model{}
	}
	return dto.Model{
		Name:             modelCfg.GetModel(),
		MaxContextTokens: int(modelCfg.GetMaxContextTokens()),
	}
}

func mapDTOModel(req *dto.RefineRequest) core.ModelConfig {
	return core.ModelConfig{
		Name:             req.Model.Name,
		MaxContextTokens: req.Model.MaxContextTokens,
	}
}

func resolveBudget(req *dto.RefineRequest, model core.ModelConfig, policy core.RuntimePolicy) int {
	budget := req.TokenBudget
	if budget <= 0 && model.MaxContextTokens > 0 && policy.BudgetRatio > 0 {
		return int(float64(model.MaxContextTokens) * policy.BudgetRatio)
	}
	return budget
}

func normalizeRefineIDs(session string, request string) (sessionID string, requestID string) {
	sessionID = strings.TrimSpace(session)
	requestID = strings.TrimSpace(request)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	if sessionID == "" {
		sessionID = "session-" + requestID
	}
	return sessionID, requestID
}

func mapProtoSystem(req *refinerv1.RefineRequest) string {
	if req == nil {
		return ""
	}
	return strings.TrimSpace(req.GetSystem())
}

func protoMemoryChunks(req *refinerv1.RefineRequest) []*refinerv1.RagChunk {
	if req == nil {
		return nil
	}
	if memory := req.GetMemory(); memory != nil {
		return memory.GetRagChunks()
	}
	return nil
}
