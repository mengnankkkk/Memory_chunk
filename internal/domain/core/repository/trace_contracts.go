package repository

import (
	"context"
	"time"
)

type TraceSearchQuery struct {
	Query           string `json:"query"`
	Limit           int    `json:"limit,omitempty"`
	Start           int64  `json:"start,omitempty"`
	End             int64  `json:"end,omitempty"`
	SpansPerSpanSet int    `json:"spans_per_span_set,omitempty"`
}

type TraceRepository interface {
	SearchTraces(ctx context.Context, query TraceSearchQuery) (TraceSearchResult, error)
	LoadTraceDetail(ctx context.Context, traceID string, start int64, end int64) (TraceDetail, error)
}

type TraceSearchResult struct {
	Query      TraceSearchQuery  `json:"query"`
	TraceCount int               `json:"trace_count"`
	Traces     []TraceSearchItem `json:"traces"`
	Metrics    map[string]any    `json:"metrics,omitempty"`
}

type TraceSearchItem struct {
	TraceID         string              `json:"trace_id"`
	RootServiceName string              `json:"root_service_name"`
	RootTraceName   string              `json:"root_trace_name"`
	StartTime       time.Time           `json:"start_time"`
	DurationMs      int64               `json:"duration_ms"`
	MatchedSpans    int                 `json:"matched_spans"`
	SpanCount       int                 `json:"span_count"`
	ServiceCount    int                 `json:"service_count"`
	ErrorCount      int                 `json:"error_count"`
	SpanSets        []TraceMatchSpanSet `json:"span_sets,omitempty"`
}

type TraceMatchSpanSet struct {
	Matched int              `json:"matched"`
	Spans   []TraceMatchSpan `json:"spans,omitempty"`
}

type TraceMatchSpan struct {
	SpanID     string           `json:"span_id"`
	Name       string           `json:"name"`
	StartTime  time.Time        `json:"start_time"`
	DurationMs int64            `json:"duration_ms"`
	Attributes []TraceAttribute `json:"attributes,omitempty"`
}

type TraceDetail struct {
	TraceID         string             `json:"trace_id"`
	RootServiceName string             `json:"root_service_name"`
	RootSpanName    string             `json:"root_span_name"`
	StartTime       time.Time          `json:"start_time"`
	EndTime         time.Time          `json:"end_time"`
	DurationMs      int64              `json:"duration_ms"`
	SpanCount       int                `json:"span_count"`
	ServiceCount    int                `json:"service_count"`
	ErrorCount      int                `json:"error_count"`
	Services        []TraceServiceStat `json:"services,omitempty"`
	Spans           []TraceSpanRecord  `json:"spans,omitempty"`
}

type TraceServiceStat struct {
	ServiceName string `json:"service_name"`
	SpanCount   int    `json:"span_count"`
	ErrorCount  int    `json:"error_count"`
}

type TraceSpanRecord struct {
	TraceID       string           `json:"trace_id"`
	SpanID        string           `json:"span_id"`
	ParentSpanID  string           `json:"parent_span_id,omitempty"`
	Name          string           `json:"name"`
	ServiceName   string           `json:"service_name"`
	ScopeName     string           `json:"scope_name,omitempty"`
	ScopeVersion  string           `json:"scope_version,omitempty"`
	Kind          string           `json:"kind,omitempty"`
	StatusCode    string           `json:"status_code,omitempty"`
	StatusMessage string           `json:"status_message,omitempty"`
	StartTime     time.Time        `json:"start_time"`
	EndTime       time.Time        `json:"end_time"`
	DurationMs    int64            `json:"duration_ms"`
	Depth         int              `json:"depth"`
	Attributes    []TraceAttribute `json:"attributes,omitempty"`
	Events        []TraceEvent     `json:"events,omitempty"`
}

type TraceAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type TraceEvent struct {
	Name       string           `json:"name"`
	Time       time.Time        `json:"time"`
	Attributes []TraceAttribute `json:"attributes,omitempty"`
}
