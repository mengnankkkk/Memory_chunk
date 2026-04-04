package tokenizer

import (
	"fmt"
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"

	"context-refiner/internal/engine"
)

type Counter struct {
	encoding *tiktoken.Tiktoken
}

func NewCounter(model, fallbackEncoding string) (*Counter, error) {
	var (
		encoding *tiktoken.Tiktoken
		err      error
	)

	if strings.TrimSpace(model) != "" {
		encoding, err = tiktoken.EncodingForModel(model)
		if err == nil {
			return &Counter{encoding: encoding}, nil
		}
	}

	if strings.TrimSpace(fallbackEncoding) == "" {
		fallbackEncoding = "cl100k_base"
	}
	encoding, err = tiktoken.GetEncoding(fallbackEncoding)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer encoding failed: %w", err)
	}
	return &Counter{encoding: encoding}, nil
}

func (c *Counter) CountText(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return len(c.encoding.Encode(text, nil, nil))
}

func (c *Counter) CountFragment(fragment engine.RAGFragment) int {
	return c.CountText(engine.FragmentText(fragment))
}

func (c *Counter) CountChunk(chunk engine.RAGChunk) int {
	return c.CountText(engine.ChunkText(chunk))
}

func (c *Counter) CountRequest(req *engine.RefineRequest) int {
	if req.OptimizedPrompt != "" {
		return c.CountText(req.OptimizedPrompt)
	}
	return c.CountText(engine.AssemblePrompt(req))
}
