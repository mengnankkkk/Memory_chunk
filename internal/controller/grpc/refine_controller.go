package grpc

import (
	"context"

	refinerv1 "context-refiner/api/refinerv1"
	serviceapi "context-refiner/pkg/service"

	"google.golang.org/grpc"
)

type RefinerHandler struct {
	refinerv1.UnimplementedRefinerServiceServer
	service serviceapi.RefinerService
}

func NewRefinerHandler(service serviceapi.RefinerService) *RefinerHandler {
	return &RefinerHandler{service: service}
}

func RegisterRefinerService(server grpc.ServiceRegistrar, service serviceapi.RefinerService) {
	refinerv1.RegisterRefinerServiceServer(server, NewRefinerHandler(service))
}

func (h *RefinerHandler) Refine(ctx context.Context, req *refinerv1.RefineRequest) (*refinerv1.RefineResponse, error) {
	return h.service.Refine(ctx, req)
}

func (h *RefinerHandler) PageIn(ctx context.Context, req *refinerv1.PageInRequest) (*refinerv1.PageInResponse, error) {
	return h.service.PageIn(ctx, req)
}
