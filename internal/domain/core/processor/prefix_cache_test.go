package processor

import (
	"context"
	"strconv"
	"testing"

	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/repository"
)

type fakePrefixCacheRepository struct {
	registration repository.PrefixCacheRegistration
	calls        int
}

func (f *fakePrefixCacheRepository) RegisterPrefix(_ context.Context, entry repository.PrefixCacheEntry) (repository.PrefixCacheRegistration, error) {
	f.calls++
	registration := f.registration
	if registration.Entry.Key == "" {
		registration.Entry = entry
	}
	if registration.Entry.PrefixHash == "" {
		registration.Entry.PrefixHash = entry.PrefixHash
	}
	if registration.Entry.ModelID == "" {
		registration.Entry.ModelID = entry.ModelID
	}
	return registration, nil
}

func TestPrefixCacheProcessorSetsPredictionMetadataForSkippedPrefix(t *testing.T) {
	store := &fakePrefixCacheRepository{}
	processor := NewPrefixCacheProcessor(runeCounter{}, store, core.PrefixCachePolicy{
		MinStablePrefixTokens: 256,
		MinSegmentCount:       1,
		DefaultTenant:         "global",
	})
	req := &core.RefineRequest{
		SessionID: "session-1",
		RequestID: "request-1",
		Model:     core.ModelConfig{Name: "test-model"},
		Messages: []core.Message{
			{Role: "system", Content: "short"},
			{Role: "user", Content: "active turn"},
		},
		Metadata: map[string]string{
			"policy": "strict_coding_assistant",
		},
	}

	updated, _, err := processor.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("expected skipped prefix to avoid repository lookup")
	}
	if got := updated.Metadata["cache_observation_level"]; got != "application" {
		t.Fatalf("expected cache_observation_level=application, got %q", got)
	}
	if got := updated.Metadata["cache_prediction_result"]; got != "predicted_miss" {
		t.Fatalf("expected predicted_miss, got %q", got)
	}
	if got := updated.Metadata["predicted_reusable_tokens"]; got != "0" {
		t.Fatalf("expected predicted_reusable_tokens=0, got %q", got)
	}
	if got := updated.Metadata["segment_churn_reason"]; got != "system" {
		t.Fatalf("expected segment_churn_reason=system, got %q", got)
	}
}

func TestPrefixCacheProcessorSetsPredictionMetadataForCreatedPrefix(t *testing.T) {
	store := &fakePrefixCacheRepository{
		registration: repository.PrefixCacheRegistration{
			Entry: repository.PrefixCacheEntry{
				Key:      "prefix-key",
				ModelID:  "test-model",
				HitCount: 1,
			},
			Result:        "created",
			MissReason:    "created",
			SegmentReason: "cold_start",
		},
	}
	processor := NewPrefixCacheProcessor(runeCounter{}, store, core.PrefixCachePolicy{
		MinStablePrefixTokens: 1,
		MinSegmentCount:       1,
		DefaultTenant:         "global",
	})
	req := &core.RefineRequest{
		SessionID: "session-2",
		RequestID: "request-2",
		Model:     core.ModelConfig{Name: "test-model"},
		Messages: []core.Message{
			{Role: "system", Content: "stable prefix body for cache prediction"},
			{Role: "user", Content: "active turn"},
		},
		Metadata: map[string]string{
			"policy": "strict_coding_assistant",
		},
	}

	identity := core.BuildPrefixCacheIdentity(req, runeCounter{})
	updated, _, err := processor.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if store.calls != 1 {
		t.Fatalf("expected repository lookup once, got %d", store.calls)
	}
	if got := updated.Metadata["cache_prediction_result"]; got != "partial_reusable" {
		t.Fatalf("expected partial_reusable, got %q", got)
	}
	if got := updated.Metadata["predicted_reusable_tokens"]; got != strconv.Itoa(identity.StablePrefixTokens) {
		t.Fatalf("expected predicted_reusable_tokens=%d, got %q", identity.StablePrefixTokens, got)
	}
	if got := updated.Metadata["segment_churn_reason"]; got != "cold_start" {
		t.Fatalf("expected segment_churn_reason=cold_start, got %q", got)
	}
}

func TestPrefixCacheProcessorSetsPredictionMetadataForHitPrefix(t *testing.T) {
	store := &fakePrefixCacheRepository{
		registration: repository.PrefixCacheRegistration{
			Entry: repository.PrefixCacheEntry{
				Key:      "prefix-key",
				ModelID:  "test-model",
				HitCount: 3,
			},
			Result:        "hit",
			MissReason:    "none",
			SegmentReason: "none",
		},
	}
	processor := NewPrefixCacheProcessor(runeCounter{}, store, core.PrefixCachePolicy{
		MinStablePrefixTokens: 1,
		MinSegmentCount:       1,
		DefaultTenant:         "global",
	})
	req := &core.RefineRequest{
		SessionID: "session-3",
		RequestID: "request-3",
		Model:     core.ModelConfig{Name: "test-model"},
		Messages: []core.Message{
			{Role: "system", Content: "stable prefix body for cache prediction"},
			{Role: "user", Content: "active turn"},
		},
		Metadata: map[string]string{
			"policy": "strict_coding_assistant",
		},
	}

	updated, _, err := processor.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if got := updated.Metadata["cache_prediction_result"]; got != "predicted_hit" {
		t.Fatalf("expected predicted_hit, got %q", got)
	}
	if got := updated.Metadata["segment_churn_reason"]; got != "none" {
		t.Fatalf("expected segment_churn_reason=none, got %q", got)
	}
}
