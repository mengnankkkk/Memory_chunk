package processor

import "context-refiner/internal/domain/core"

func cloneRequest(req *core.RefineRequest) *core.RefineRequest {
	updated := *req
	updated.Messages = append([]core.Message(nil), req.Messages...)
	updated.RAGChunks = make([]core.RAGChunk, 0, len(req.RAGChunks))
	for _, chunk := range req.RAGChunks {
		cloned := chunk
		cloned.Sources = append([]string(nil), chunk.Sources...)
		cloned.PageRefs = append([]string(nil), chunk.PageRefs...)
		cloned.Fragments = append([]core.RAGFragment(nil), chunk.Fragments...)
		updated.RAGChunks = append(updated.RAGChunks, cloned)
	}
	updated.Audits = append([]core.StepAudit(nil), req.Audits...)
	updated.PendingSummaryJobIDs = append([]string(nil), req.PendingSummaryJobIDs...)
	if req.Metadata != nil {
		updated.Metadata = make(map[string]string, len(req.Metadata))
		for k, v := range req.Metadata {
			updated.Metadata[k] = v
		}
	}
	return &updated
}
