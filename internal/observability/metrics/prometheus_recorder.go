package observability

import (
	"net/http"
	"time"

	coreobs "context-refiner/internal/observability"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusRecorder struct {
	registry *prometheus.Registry

	refineRequests       *prometheus.CounterVec
	refineDuration       *prometheus.HistogramVec
	pageInRequests       *prometheus.CounterVec
	pageInDuration       *prometheus.HistogramVec
	pageInPages          *prometheus.CounterVec
	tokenTotals          *prometheus.CounterVec
	promptSegmentTotals  *prometheus.CounterVec
	pipelineStepDuration *prometheus.HistogramVec
	pipelineStepTokens   *prometheus.CounterVec
	prefixCacheLookups   *prometheus.CounterVec
	prefixCacheTokens    *prometheus.CounterVec
	pageArtifactWrites   *prometheus.CounterVec
	storeLoads           *prometheus.CounterVec
	summaryJobs          *prometheus.CounterVec
}

var _ coreobs.Recorder = (*PrometheusRecorder)(nil)

func NewPrometheusRecorder() (*PrometheusRecorder, error) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewBuildInfoCollector(),
	)

	recorder := &PrometheusRecorder{
		registry: registry,
		refineRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "refine_requests_total",
			Help:      "Total refine requests grouped by policy, status and budget result.",
		}, []string{"policy", "status", "budget_met"}),
		refineDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "context_refiner",
			Name:      "refine_duration_seconds",
			Help:      "Refine request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"policy", "status"}),
		pageInRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "pagein_requests_total",
			Help:      "Total page-in requests grouped by status.",
		}, []string{"status"}),
		pageInDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "context_refiner",
			Name:      "pagein_duration_seconds",
			Help:      "Page-in request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"status"}),
		pageInPages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "pagein_pages_total",
			Help:      "Page-in page totals grouped by requested, summary or page.",
		}, []string{"result"}),
		tokenTotals: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "tokens_total",
			Help:      "Aggregated token totals by kind.",
		}, []string{"kind"}),
		promptSegmentTotals: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "prompt_segments_total",
			Help:      "Aggregated prompt segment totals by kind.",
		}, []string{"kind"}),
		pipelineStepDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "context_refiner",
			Name:      "pipeline_step_duration_seconds",
			Help:      "Pipeline processor duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"step"}),
		pipelineStepTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "pipeline_step_tokens_total",
			Help:      "Aggregated pipeline tokens grouped by step and token kind.",
		}, []string{"step", "kind"}),
		prefixCacheLookups: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "prefix_cache_lookups_total",
			Help:      "Application-level prefix cache lookups grouped by result and miss reason.",
		}, []string{"result", "miss_reason"}),
		prefixCacheTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "prefix_cache_tokens_total",
			Help:      "Stable prefix tokens observed during prefix cache lookup.",
		}, []string{"kind"}),
		pageArtifactWrites: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "page_artifact_writes_total",
			Help:      "Page artifact write results grouped by created, reused or error.",
		}, []string{"result"}),
		storeLoads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "store_page_loads_total",
			Help:      "Store page load results grouped by summary, page, miss or error.",
		}, []string{"result"}),
		summaryJobs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "context_refiner",
			Name:      "summary_jobs_total",
			Help:      "Summary job lifecycle events grouped by action.",
		}, []string{"action"}),
	}

	collectorsToRegister := []prometheus.Collector{
		recorder.refineRequests,
		recorder.refineDuration,
		recorder.pageInRequests,
		recorder.pageInDuration,
		recorder.pageInPages,
		recorder.tokenTotals,
		recorder.promptSegmentTotals,
		recorder.pipelineStepDuration,
		recorder.pipelineStepTokens,
		recorder.prefixCacheLookups,
		recorder.prefixCacheTokens,
		recorder.pageArtifactWrites,
		recorder.storeLoads,
		recorder.summaryJobs,
	}
	for _, collector := range collectorsToRegister {
		if err := registry.Register(collector); err != nil {
			return nil, err
		}
	}
	return recorder, nil
}

func (r *PrometheusRecorder) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{
		Registry:            r.registry,
		EnableOpenMetrics:   true,
		MaxRequestsInFlight: 4,
	})
}

func (r *PrometheusRecorder) ObserveRefine(policy string, status string, budgetMet string, inputTokens int, outputTokens int, stablePrefixTokens int, stableChunks int, stableMessages int, dynamicMessages int, pagedChunks int, pendingSummaryJobs int, duration time.Duration) {
	r.refineRequests.WithLabelValues(policy, status, budgetMet).Inc()
	r.refineDuration.WithLabelValues(policy, status).Observe(duration.Seconds())
	r.tokenTotals.WithLabelValues("input").Add(float64(maxInt(inputTokens, 0)))
	r.tokenTotals.WithLabelValues("output").Add(float64(maxInt(outputTokens, 0)))
	r.tokenTotals.WithLabelValues("stable_prefix").Add(float64(maxInt(stablePrefixTokens, 0)))
	r.tokenTotals.WithLabelValues("reduced").Add(float64(maxInt(inputTokens-outputTokens, 0)))
	r.promptSegmentTotals.WithLabelValues("stable_rag_chunks").Add(float64(maxInt(stableChunks, 0)))
	r.promptSegmentTotals.WithLabelValues("stable_messages").Add(float64(maxInt(stableMessages, 0)))
	r.promptSegmentTotals.WithLabelValues("dynamic_messages").Add(float64(maxInt(dynamicMessages, 0)))
	r.promptSegmentTotals.WithLabelValues("paged_chunks").Add(float64(maxInt(pagedChunks, 0)))
	r.promptSegmentTotals.WithLabelValues("pending_summary_jobs").Add(float64(maxInt(pendingSummaryJobs, 0)))
}

func (r *PrometheusRecorder) ObservePageIn(status string, requested int, summaryHits int, pageHits int, duration time.Duration) {
	r.pageInRequests.WithLabelValues(status).Inc()
	r.pageInDuration.WithLabelValues(status).Observe(duration.Seconds())
	r.pageInPages.WithLabelValues("requested").Add(float64(maxInt(requested, 0)))
	r.pageInPages.WithLabelValues("summary").Add(float64(maxInt(summaryHits, 0)))
	r.pageInPages.WithLabelValues("page").Add(float64(maxInt(pageHits, 0)))
}

func (r *PrometheusRecorder) ObservePipelineStep(step string, beforeTokens int, afterTokens int, duration time.Duration) {
	r.pipelineStepDuration.WithLabelValues(step).Observe(duration.Seconds())
	r.pipelineStepTokens.WithLabelValues(step, "before").Add(float64(maxInt(beforeTokens, 0)))
	r.pipelineStepTokens.WithLabelValues(step, "after").Add(float64(maxInt(afterTokens, 0)))
	r.pipelineStepTokens.WithLabelValues(step, "reduced").Add(float64(maxInt(beforeTokens-afterTokens, 0)))
}

func (r *PrometheusRecorder) ObservePrefixCacheLookup(result string, missReason string, stablePrefixTokens int) {
	if result == "" {
		result = "unknown"
	}
	if missReason == "" {
		missReason = "none"
	}
	r.prefixCacheLookups.WithLabelValues(result, missReason).Inc()
	r.prefixCacheTokens.WithLabelValues("stable_prefix").Add(float64(maxInt(stablePrefixTokens, 0)))
}

func (r *PrometheusRecorder) ObservePageArtifactWrite(result string) {
	r.pageArtifactWrites.WithLabelValues(result).Inc()
}

func (r *PrometheusRecorder) ObserveStoreLoad(result string) {
	r.storeLoads.WithLabelValues(result).Inc()
}

func (r *PrometheusRecorder) ObserveSummaryJob(action string) {
	r.summaryJobs.WithLabelValues(action).Inc()
}

func maxInt(value int, floor int) int {
	if value < floor {
		return floor
	}
	return value
}
