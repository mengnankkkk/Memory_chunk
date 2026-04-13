package mapper_test

import (
	"strings"
	"testing"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/domain/core"
	"context-refiner/internal/dto"
	"context-refiner/internal/mapper"
)

func TestMapRefineProtoRequestToDTOAndDomain(t *testing.T) {
	req := &refinerv1.RefineRequest{
		Policy: "default",
		Messages: []*refinerv1.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hello"},
		},
		RagChunks: []*refinerv1.RagChunk{
			{
				Id:      "chunk-1",
				Source:  "kb-a",
				Content: "plain body",
			},
		},
		Model: &refinerv1.ModelConfig{
			Model:            "gpt-test",
			MaxContextTokens: 1000,
		},
	}

	dtoReq := mapper.MapRefineProtoRequestToDTO(req)
	if dtoReq.RequestID == "" {
		t.Fatal("expected request id to be generated")
	}
	if dtoReq.SessionID != "session-"+dtoReq.RequestID {
		t.Fatalf("expected derived session id, got %q", dtoReq.SessionID)
	}
	if len(dtoReq.RAGChunks) != 1 || len(dtoReq.RAGChunks[0].Fragments) != 1 {
		t.Fatal("expected fallback fragment to be generated")
	}
	if dtoReq.RAGChunks[0].Fragments[0].Type != string(core.FragmentTypeBody) {
		t.Fatalf("unexpected fragment type: %q", dtoReq.RAGChunks[0].Fragments[0].Type)
	}

	policy := core.RuntimePolicy{
		Name:        "default",
		BudgetRatio: 0.5,
	}
	domainReq := mapper.MapRefineDTOToDomainRequest(dtoReq, policy)
	if domainReq.Budget != 500 {
		t.Fatalf("expected derived budget 500, got %d", domainReq.Budget)
	}
	if domainReq.Policy != "default" {
		t.Fatalf("expected policy default, got %q", domainReq.Policy)
	}
	if domainReq.Metadata["session_id"] != dtoReq.SessionID {
		t.Fatalf("expected session_id metadata to match dto session id, got %q", domainReq.Metadata["session_id"])
	}
	if domainReq.Metadata["cache_optimization_target"] != "prefix-reuse-stability" {
		t.Fatalf("unexpected cache optimization target: %q", domainReq.Metadata["cache_optimization_target"])
	}
	if len(domainReq.RAGChunks[0].Sources) != 1 || domainReq.RAGChunks[0].Sources[0] != "kb-a" {
		t.Fatalf("expected stable sources to be preserved, got %#v", domainReq.RAGChunks[0].Sources)
	}
}

func TestMapRefineDomainResponseToDTOAndProto(t *testing.T) {
	resp := &core.RefineResponse{
		OptimizedPrompt: "stable prompt",
		InputTokens:     120,
		OutputTokens:    80,
		BudgetMet:       true,
		Audits: []core.StepAudit{
			{
				Name:         "canonicalize",
				BeforeTokens: 120,
				AfterTokens:  100,
				DurationMS:   5,
				Details: map[string]string{
					"normalized": "true",
				},
				Capabilities: core.ProcessorCapabilities{
					Aggressive: true,
				},
				Semantic: core.StepSemanticAudit{
					Removed: []string{"volatile_fields"},
				},
			},
		},
		PagedChunks: []core.PagedChunk{
			{
				SessionID: "session-1",
				RequestID: "req-1",
				ChunkID:   "chunk-1",
				PageKeys:  []string{"page-1"},
			},
		},
		Metadata: map[string]string{
			"prefix_cache_lookup": "hit",
		},
		PendingSummaryJobIDs: []string{"job-1"},
	}

	dtoResp := mapper.MapRefineDomainResponseToDTO(resp)
	if dtoResp.OptimizedPrompt != "stable prompt" {
		t.Fatalf("unexpected dto prompt: %q", dtoResp.OptimizedPrompt)
	}
	dtoResp.Metadata["prefix_cache_lookup"] = "mutated"
	if resp.Metadata["prefix_cache_lookup"] != "hit" {
		t.Fatal("expected metadata to be cloned when mapping domain response to dto")
	}

	protoResp := mapper.MapRefineDTOToProtoResponse(dtoResp)
	if protoResp.GetPendingSummaryJobIds()[0] != "job-1" {
		t.Fatalf("unexpected pending summary job ids: %#v", protoResp.GetPendingSummaryJobIds())
	}
	if protoResp.GetAudits()[0].GetName() != "canonicalize" {
		t.Fatalf("unexpected audit name: %q", protoResp.GetAudits()[0].GetName())
	}
	if !strings.EqualFold(protoResp.GetPagedChunks()[0].GetChunkId(), "chunk-1") {
		t.Fatalf("unexpected paged chunk id: %q", protoResp.GetPagedChunks()[0].GetChunkId())
	}
}

func TestMapPageInProtoRequestAndDTOResponse(t *testing.T) {
	dtoReq := mapper.MapPageInProtoRequestToDTO(&refinerv1.PageInRequest{
		PageKeys: []string{"page-1", "page-2"},
	})
	if len(dtoReq.PageKeys) != 2 {
		t.Fatalf("expected 2 page keys, got %d", len(dtoReq.PageKeys))
	}

	protoResp := mapper.MapPageInDTOToProtoResponse(&dto.PageInResponse{
		Pages: []dto.StoredPage{
			{
				Key:          "page-1",
				Content:      "summary",
				IsSummary:    true,
				SummaryJobID: "job-1",
			},
		},
	})
	if len(protoResp.GetPages()) != 1 {
		t.Fatalf("expected 1 page, got %d", len(protoResp.GetPages()))
	}
	if !protoResp.GetPages()[0].GetIsSummary() {
		t.Fatal("expected stored page to be marked as summary")
	}
}
