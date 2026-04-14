package summary

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"context-refiner/internal/domain/core/repository"
)

type Provider interface {
	Name() string
	Version() string
	Summarize(ctx context.Context, job repository.SummaryJob) (repository.SummaryArtifact, error)
}

type HeuristicProvider struct{}

func NewHeuristicProvider() *HeuristicProvider {
	return &HeuristicProvider{}
}

func (p *HeuristicProvider) Name() string {
	return repository.SummaryProviderHeuristic
}

func (p *HeuristicProvider) Version() string {
	return repository.SummaryProviderVersionHeuristicV1
}

func (p *HeuristicProvider) Summarize(_ context.Context, job repository.SummaryJob) (repository.SummaryArtifact, error) {
	summaryText := strings.TrimSpace(summarizeJob(job))
	if summaryText == "" {
		return repository.SummaryArtifact{}, fmt.Errorf("empty summary text")
	}
	createdAt := time.Now().UTC()
	return repository.SummaryArtifact{
		ArtifactID:      repository.BuildSummaryArtifactID(job.ContentHash, p.Name(), p.Version()),
		JobID:           job.JobID,
		SessionID:       job.SessionID,
		RequestID:       job.RequestID,
		Policy:          job.Policy,
		ChunkID:         job.ChunkID,
		Source:          job.Source,
		PageRefs:        append([]string(nil), job.PageRefs...),
		ContentHash:     strings.TrimSpace(job.ContentHash),
		SummaryText:     summaryText,
		FragmentTypes:   summaryFragmentTypes(job.Fragments),
		Provider:        p.Name(),
		ProviderVersion: p.Version(),
		SchemaVersion:   repository.SummaryArtifactSchemaVersionV1,
		CreatedAt:       createdAt,
	}, nil
}

func summaryFragmentTypes(fragments []repository.SummaryFragment) []string {
	unique := make(map[string]struct{}, len(fragments))
	for _, fragment := range fragments {
		fragmentType := strings.TrimSpace(fragment.Type)
		if fragmentType == "" {
			continue
		}
		unique[fragmentType] = struct{}{}
	}
	out := make([]string, 0, len(unique))
	for fragmentType := range unique {
		out = append(out, fragmentType)
	}
	sort.Strings(out)
	return out
}
