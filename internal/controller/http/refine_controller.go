package httpcontroller

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"context-refiner/internal/dto"
	"context-refiner/internal/service"
)

type RefineController struct {
	service *service.RefinerApplicationService
}

func NewRefineController(svc *service.RefinerApplicationService) *RefineController {
	return &RefineController{service: svc}
}

// 简化的对外接入入参：system + messages + rag 三件套，其余都是可选项。
type simpleRefineRequest struct {
	System    string            `json:"system"`
	Messages  []simpleMessage   `json:"messages"`
	RAG       json.RawMessage   `json:"rag"`
	Budget    int               `json:"budget"`
	Policy    string            `json:"policy"`
	Model     json.RawMessage   `json:"model"`
	SessionID string            `json:"session_id"`
	RequestID string            `json:"request_id"`
	Metadata  map[string]string `json:"metadata"`
}

type simpleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type simpleRagItem struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

type simpleModel struct {
	Name             string `json:"name"`
	MaxContextTokens int    `json:"max_context_tokens"`
}

type simpleRefineResponse struct {
	Prompt           string  `json:"prompt"`
	TraceID          string  `json:"trace_id"`
	RequestID        string  `json:"request_id"`
	SessionID        string  `json:"session_id"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	SavedTokens      int     `json:"saved_tokens"`
	CompressionRatio float64 `json:"compression_ratio"`
	BudgetMet        bool    `json:"budget_met"`
	CacheHit         bool    `json:"cache_hit"`
}

func (c *RefineController) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "只支持 POST 请求")
		return
	}
	defer r.Body.Close()

	var in simpleRefineRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("解析 JSON 失败：%v", err))
		return
	}
	if len(in.Messages) == 0 && strings.TrimSpace(in.System) == "" {
		writeJSONError(w, http.StatusBadRequest, "messages 与 system 至少要提供一个")
		return
	}

	req, err := buildRefineDTO(in)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := c.service.RefineDTO(r.Context(), req)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildSimpleResponse(req, resp))
}

func buildRefineDTO(in simpleRefineRequest) (*dto.RefineRequest, error) {
	messages := make([]dto.Message, 0, len(in.Messages)+1)
	if sys := strings.TrimSpace(in.System); sys != "" {
		messages = append(messages, dto.Message{Role: "system", Content: sys})
	}
	for _, m := range in.Messages {
		role := strings.TrimSpace(m.Role)
		if role == "" {
			role = "user"
		}
		messages = append(messages, dto.Message{Role: role, Content: m.Content})
	}

	rag, err := decodeRAG(in.RAG)
	if err != nil {
		return nil, err
	}

	model, err := decodeModel(in.Model)
	if err != nil {
		return nil, err
	}

	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		sessionID = "ext-" + randomID(8)
	}
	requestID := strings.TrimSpace(in.RequestID)
	if requestID == "" {
		requestID = "req-" + randomID(12)
	}

	return &dto.RefineRequest{
		SessionID:   sessionID,
		RequestID:   requestID,
		Messages:    messages,
		RAGChunks:   rag,
		Model:       model,
		TokenBudget: in.Budget,
		Policy:      strings.TrimSpace(in.Policy),
	}, nil
}

func decodeRAG(raw json.RawMessage) ([]dto.RAGChunk, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	// 优先尝试字符串数组：["片段1","片段2"]
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err == nil {
		out := make([]dto.RAGChunk, 0, len(asStrings))
		for i, s := range asStrings {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, dto.RAGChunk{
				ID:     fmt.Sprintf("ext-rag-%d", i+1),
				Source: "external",
				Fragments: []dto.RAGFragment{
					{Type: "FRAGMENT_TYPE_BODY", Content: s},
				},
			})
		}
		return out, nil
	}
	// 再尝试对象数组：[{id,source,content,type}]
	var asItems []simpleRagItem
	if err := json.Unmarshal(raw, &asItems); err != nil {
		return nil, fmt.Errorf("rag 字段需要是字符串数组或对象数组：%v", err)
	}
	out := make([]dto.RAGChunk, 0, len(asItems))
	for i, item := range asItems {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		fragType := strings.TrimSpace(item.Type)
		if fragType == "" {
			fragType = "FRAGMENT_TYPE_BODY"
		}
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("ext-rag-%d", i+1)
		}
		source := strings.TrimSpace(item.Source)
		if source == "" {
			source = "external"
		}
		out = append(out, dto.RAGChunk{
			ID:        id,
			Source:    source,
			Fragments: []dto.RAGFragment{{Type: fragType, Content: content}},
		})
	}
	return out, nil
}

func decodeModel(raw json.RawMessage) (dto.Model, error) {
	if len(raw) == 0 {
		return dto.Model{}, nil
	}
	// 字符串形式："gpt-4o-mini"
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return dto.Model{Name: strings.TrimSpace(asString)}, nil
	}
	// 对象形式：{name, max_context_tokens}
	var asObj simpleModel
	if err := json.Unmarshal(raw, &asObj); err != nil {
		return dto.Model{}, fmt.Errorf("model 字段需要是字符串或对象：%v", err)
	}
	return dto.Model{
		Name:             strings.TrimSpace(asObj.Name),
		MaxContextTokens: asObj.MaxContextTokens,
	}, nil
}

func buildSimpleResponse(req *dto.RefineRequest, resp *dto.RefineResponse) simpleRefineResponse {
	saved := resp.InputTokens - resp.OutputTokens
	if saved < 0 {
		saved = 0
	}
	ratio := 0.0
	if resp.InputTokens > 0 {
		ratio = float64(resp.OutputTokens) / float64(resp.InputTokens)
	}
	cacheHit := false
	traceID := ""
	if resp.Metadata != nil {
		cacheHit = strings.EqualFold(resp.Metadata["prefix_cache_lookup"], "hit")
		traceID = resp.Metadata["trace_id"]
	}
	return simpleRefineResponse{
		Prompt:           resp.OptimizedPrompt,
		TraceID:          traceID,
		RequestID:        req.RequestID,
		SessionID:        req.SessionID,
		InputTokens:      resp.InputTokens,
		OutputTokens:     resp.OutputTokens,
		SavedTokens:      saved,
		CompressionRatio: ratio,
		BudgetMet:        resp.BudgetMet,
		CacheHit:         cacheHit,
	}
}

func randomID(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(buf)
}
