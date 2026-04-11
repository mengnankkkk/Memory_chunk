package client

import (
	"context"

	refinerv1 "context-refiner/api/refinerv1"
	serviceapi "context-refiner/pkg/service"
)

type Client struct {
	refiner serviceapi.RefinerService
}

func New(refiner serviceapi.RefinerService) *Client {
	return &Client{refiner: refiner}
}

func (c *Client) Refine(ctx context.Context, req *refinerv1.RefineRequest) (*refinerv1.RefineResponse, error) {
	return c.refiner.Refine(ctx, req)
}

func (c *Client) PageIn(ctx context.Context, req *refinerv1.PageInRequest) (*refinerv1.PageInResponse, error) {
	return c.refiner.PageIn(ctx, req)
}
