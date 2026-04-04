package client

import (
	"context"

	"context-refiner/internal/engine"
)

type Refiner interface {
	Run(ctx context.Context, req *engine.RefineRequest) (*engine.RefineResponse, error)
}

type Client struct {
	refiner Refiner
}

func New(refiner Refiner) *Client {
	return &Client{refiner: refiner}
}

func (c *Client) Refine(ctx context.Context, req *engine.RefineRequest) (*engine.RefineResponse, error) {
	return c.refiner.Run(ctx, req)
}
