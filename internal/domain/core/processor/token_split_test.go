package processor

import (
	"testing"

	"context-refiner/internal/domain/core"
)

type runeCounter struct{}

func (r runeCounter) CountText(text string) int {
	return len([]rune(text))
}

func (r runeCounter) CountFragment(fragment core.RAGFragment) int {
	return r.CountText(fragment.Content)
}

func (r runeCounter) CountChunk(chunk core.RAGChunk) int {
	return r.CountText(core.ChunkText(chunk))
}

func (r runeCounter) CountRequest(req *core.RefineRequest) int {
	return r.CountText(req.OptimizedPrompt)
}

func TestSplitFragmentByTokenBudgetPreservesMetadata(t *testing.T) {
	counter := runeCounter{}
	fragment := core.RAGFragment{
		Type:     core.FragmentTypeCode,
		Content:  "abcd\nefgh\nijkl",
		Language: "go",
	}

	parts := splitFragmentByTokenBudget(counter, fragment, 5)
	if len(parts) < 2 {
		t.Fatalf("expected fragment to be split, got %d part(s)", len(parts))
	}
	for _, part := range parts {
		if part.Type != fragment.Type {
			t.Fatalf("expected fragment type to be preserved")
		}
		if part.Language != fragment.Language {
			t.Fatalf("expected fragment language to be preserved")
		}
		if counter.CountFragment(part) > 5 {
			t.Fatalf("expected part to stay within token budget: %#v", part)
		}
	}
}

func TestSplitTextByTokenBudgetKeepsLineBoundariesWhenPossible(t *testing.T) {
	counter := runeCounter{}
	parts := splitTextByTokenBudget(counter, "ab\ncd\nef", 4)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %#v", parts)
	}
	if parts[0] != "ab" || parts[1] != "cd" || parts[2] != "ef" {
		t.Fatalf("unexpected split result: %#v", parts)
	}
}
