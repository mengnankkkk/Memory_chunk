package observability

import "time"

type Recorder interface {
	ObserveRefine(policy string, status string, budgetMet string, inputTokens int, outputTokens int, stablePrefixTokens int, stableChunks int, stableMessages int, dynamicMessages int, pagedChunks int, pendingSummaryJobs int, duration time.Duration)
	ObservePageIn(status string, requested int, summaryHits int, pageHits int, duration time.Duration)
	ObservePipelineStep(step string, beforeTokens int, afterTokens int, duration time.Duration)
	ObservePrefixCacheLookup(result string, missReason string, stablePrefixTokens int)
	ObservePageArtifactWrite(result string)
	ObserveStoreLoad(result string)
	ObserveSummaryJob(action string)
}

type NopRecorder struct{}

func NewNopRecorder() Recorder {
	return NopRecorder{}
}

func (NopRecorder) ObserveRefine(string, string, string, int, int, int, int, int, int, int, int, time.Duration) {
}

func (NopRecorder) ObservePageIn(string, int, int, int, time.Duration) {}

func (NopRecorder) ObservePipelineStep(string, int, int, time.Duration) {}

func (NopRecorder) ObservePrefixCacheLookup(string, string, int) {}

func (NopRecorder) ObservePageArtifactWrite(string) {}

func (NopRecorder) ObserveStoreLoad(string) {}

func (NopRecorder) ObserveSummaryJob(string) {}
