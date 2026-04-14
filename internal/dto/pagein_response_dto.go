package dto

import "time"

type PageInResponse struct {
	Pages []StoredPage
}

type SummaryArtifact struct {
	ArtifactID      string
	JobID           string
	SessionID       string
	RequestID       string
	Policy          string
	ChunkID         string
	Source          string
	PageRefs        []string
	ContentHash     string
	SummaryText     string
	FragmentTypes   []string
	Provider        string
	ProviderVersion string
	SchemaVersion   string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

type StoredPage struct {
	Key             string
	Content         string
	IsSummary       bool
	SummaryJobID    string
	SummaryArtifact *SummaryArtifact
}
