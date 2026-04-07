package processor

import "context-refiner/internal/engine"

func cloneRequest(req *engine.RefineRequest) *engine.RefineRequest {
	updated := *req
	updated.Messages = append([]engine.Message(nil), req.Messages...)
	updated.RAGChunks = make([]engine.RAGChunk, 0, len(req.RAGChunks))
	for _, chunk := range req.RAGChunks {
		cloned := chunk
		cloned.Sources = append([]string(nil), chunk.Sources...)
		cloned.PageRefs = append([]string(nil), chunk.PageRefs...)
		cloned.Fragments = append([]engine.RAGFragment(nil), chunk.Fragments...)
		updated.RAGChunks = append(updated.RAGChunks, cloned)
	}
	updated.Audits = append([]engine.StepAudit(nil), req.Audits...)
	updated.PendingSummaryJobIDs = append([]string(nil), req.PendingSummaryJobIDs...)
	if req.Metadata != nil {
		updated.Metadata = make(map[string]string, len(req.Metadata))
		for k, v := range req.Metadata {
			updated.Metadata[k] = v
		}
	}
	return &updated
}
