package service

import (
	"context"

	refinerv1 "context-refiner/api/refinerv1"
)

// RefinerService defines the public in-process API exposed by the application.
// gRPC is one transport adapter over this service, not the service itself.
type RefinerService interface {
	Refine(ctx context.Context, req *refinerv1.RefineRequest) (*refinerv1.RefineResponse, error)
	PageIn(ctx context.Context, req *refinerv1.PageInRequest) (*refinerv1.PageInResponse, error)
}
