package tempo

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"context-refiner/internal/domain/core/repository"
)

type Config struct {
	BaseURL string
	Timeout time.Duration
}

type Repository struct {
	baseURL string
	client  *http.Client
}

func NewRepository(cfg Config) (*Repository, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://localhost:3200"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 8 * time.Second
	}
	return &Repository{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (r *Repository) SearchTraces(ctx context.Context, query repository.TraceSearchQuery) (repository.TraceSearchResult, error) {
	traceQL := strings.TrimSpace(query.Query)
	if traceQL == "" {
		traceQL = `{ resource.service.name = "context-refiner" }`
	}

	params := url.Values{}
	params.Set("q", traceQL)
	if query.Limit > 0 {
		params.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Start > 0 {
		params.Set("start", strconv.FormatInt(query.Start, 10))
	}
	if query.End > 0 {
		params.Set("end", strconv.FormatInt(query.End, 10))
	}
	if query.SpansPerSpanSet > 0 {
		params.Set("spss", strconv.Itoa(query.SpansPerSpanSet))
	}

	var payload rawSearchResponse
	if err := r.getJSON(ctx, "/api/search?"+params.Encode(), &payload); err != nil {
		return repository.TraceSearchResult{}, err
	}

	items := payload.Traces
	if len(items) == 0 && len(payload.Results) > 0 {
		items = payload.Results
	}

	result := repository.TraceSearchResult{
		Query: repository.TraceSearchQuery{
			Query:           traceQL,
			Limit:           query.Limit,
			Start:           query.Start,
			End:             query.End,
			SpansPerSpanSet: query.SpansPerSpanSet,
		},
		Metrics: payload.Metrics,
		Traces:  make([]repository.TraceSearchItem, 0, len(items)),
	}

	for _, item := range items {
		serviceNames := make(map[string]struct{})
		rootServiceName := normalizeSearchServiceName(item.RootServiceName)
		rootTraceName := strings.TrimSpace(item.RootTraceName)
		if rootServiceName != "" {
			serviceNames[rootServiceName] = struct{}{}
		}

		spanCount := 0
		matchedSpans := 0
		errorCount := 0
		spanSets := make([]repository.TraceMatchSpanSet, 0, len(item.SpanSets))

		for _, set := range item.SpanSets {
			matchedSpans += set.Matched
			matchSpans := make([]repository.TraceMatchSpan, 0, len(set.Spans))
			for _, span := range set.Spans {
				attributes := convertAttributes(span.Attributes)
				spanCount++
				if serviceName := firstAttributeValue(attributes, "service.name"); serviceName != "" {
					serviceNames[serviceName] = struct{}{}
					if rootServiceName == "" {
						rootServiceName = serviceName
					}
				}
				if rootTraceName == "" && strings.TrimSpace(span.Name) != "" {
					rootTraceName = strings.TrimSpace(span.Name)
				}
				if isErrorAttributes(attributes) {
					errorCount++
				}
				matchSpans = append(matchSpans, repository.TraceMatchSpan{
					SpanID:     normalizeSpanID(span.SpanID),
					Name:       strings.TrimSpace(span.Name),
					StartTime:  parseUnixNano(span.StartTimeUnixNano),
					DurationMs: nanosToMilliseconds(parseInt64(span.DurationNanos)),
					Attributes: attributes,
				})
			}
			spanSets = append(spanSets, repository.TraceMatchSpanSet{
				Matched: set.Matched,
				Spans:   matchSpans,
			})
		}

		result.Traces = append(result.Traces, repository.TraceSearchItem{
			TraceID:         normalizeTraceID(item.TraceID),
			RootServiceName: rootServiceName,
			RootTraceName:   rootTraceName,
			StartTime:       parseUnixNano(item.StartTimeUnixNano),
			DurationMs:      parseInt64(item.DurationMs),
			MatchedSpans:    matchedSpans,
			SpanCount:       maxInt(spanCount, matchedSpans),
			ServiceCount:    len(serviceNames),
			ErrorCount:      errorCount,
			SpanSets:        spanSets,
		})
	}

	slices.SortFunc(result.Traces, func(a, b repository.TraceSearchItem) int {
		if a.StartTime.Equal(b.StartTime) {
			return strings.Compare(a.TraceID, b.TraceID)
		}
		if a.StartTime.After(b.StartTime) {
			return -1
		}
		return 1
	})
	result.TraceCount = len(result.Traces)
	return result, nil
}

func (r *Repository) LoadTraceDetail(ctx context.Context, traceID string, start int64, end int64) (repository.TraceDetail, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return repository.TraceDetail{}, fmt.Errorf("trace id is required")
	}

	params := url.Values{}
	if start > 0 {
		params.Set("start", strconv.FormatInt(start, 10))
	}
	if end > 0 {
		params.Set("end", strconv.FormatInt(end, 10))
	}

	path := "/api/v2/traces/" + url.PathEscape(traceID)
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var payload rawTraceDetailResponse
	if err := r.getJSON(ctx, path, &payload); err != nil {
		return repository.TraceDetail{}, err
	}

	resourceSpans := payload.Trace.ResourceSpans
	if len(resourceSpans) == 0 {
		resourceSpans = payload.ResourceSpans
	}
	if len(resourceSpans) == 0 {
		return repository.TraceDetail{}, fmt.Errorf("trace %s has no spans", traceID)
	}

	serviceStatsMap := make(map[string]*repository.TraceServiceStat)
	records := make([]repository.TraceSpanRecord, 0, 32)
	var minStart time.Time
	var maxEnd time.Time
	errorCount := 0

	for _, rs := range resourceSpans {
		resourceAttributes := convertAttributes(rs.Resource.Attributes)
		resourceService := firstAttributeValue(resourceAttributes, "service.name")

		for _, scope := range rs.ScopeSpans {
			scopeName := strings.TrimSpace(scope.Scope.Name)
			scopeVersion := strings.TrimSpace(scope.Scope.Version)
			for _, span := range scope.Spans {
				attributes := append(
					convertAttributes(span.Attributes),
					remainingResourceAttributes(resourceAttributes, span.Attributes)...,
				)
				serviceName := firstNonBlank(firstAttributeValue(attributes, "service.name"), resourceService)
				startTime := parseUnixNano(span.StartTimeUnixNano)
				endTime := parseUnixNano(span.EndTimeUnixNano)
				if minStart.IsZero() || (!startTime.IsZero() && startTime.Before(minStart)) {
					minStart = startTime
				}
				if maxEnd.IsZero() || endTime.After(maxEnd) {
					maxEnd = endTime
				}

				record := repository.TraceSpanRecord{
					TraceID:       firstNonBlank(normalizeTraceID(span.TraceID), traceID),
					SpanID:        normalizeSpanID(span.SpanID),
					ParentSpanID:  normalizeSpanID(span.ParentSpanID),
					Name:          strings.TrimSpace(span.Name),
					ServiceName:   serviceName,
					ScopeName:     scopeName,
					ScopeVersion:  scopeVersion,
					Kind:          normalizeEnumLabel(span.Kind, "SPAN_KIND_"),
					StatusCode:    normalizeEnumLabel(span.Status.Code, "STATUS_CODE_"),
					StatusMessage: strings.TrimSpace(span.Status.Message),
					StartTime:     startTime,
					EndTime:       endTime,
					DurationMs:    nanosToMilliseconds(parseDurationNanos(span.StartTimeUnixNano, span.EndTimeUnixNano)),
					Attributes:    attributes,
					Events:        convertEvents(span.Events),
				}

				spanHasError := isErrorStatus(span.Status, attributes)
				if spanHasError {
					errorCount++
					if record.StatusCode == "" {
						record.StatusCode = "ERROR"
					}
				}

				if serviceName != "" {
					stat := serviceStatsMap[serviceName]
					if stat == nil {
						stat = &repository.TraceServiceStat{ServiceName: serviceName}
						serviceStatsMap[serviceName] = stat
					}
					stat.SpanCount++
					if spanHasError {
						stat.ErrorCount++
					}
				}

				records = append(records, record)
			}
		}
	}

	if len(records) == 0 {
		return repository.TraceDetail{}, fmt.Errorf("trace %s has no spans", traceID)
	}

	slices.SortFunc(records, func(a, b repository.TraceSpanRecord) int {
		if a.StartTime.Equal(b.StartTime) {
			return strings.Compare(a.SpanID, b.SpanID)
		}
		if a.StartTime.Before(b.StartTime) {
			return -1
		}
		return 1
	})

	depthIndex := buildDepthIndex(records)
	for i := range records {
		records[i].Depth = depthIndex[records[i].SpanID]
	}

	serviceStats := make([]repository.TraceServiceStat, 0, len(serviceStatsMap))
	for _, stat := range serviceStatsMap {
		serviceStats = append(serviceStats, *stat)
	}
	slices.SortFunc(serviceStats, func(a, b repository.TraceServiceStat) int {
		if a.ErrorCount == b.ErrorCount {
			if a.SpanCount == b.SpanCount {
				return strings.Compare(a.ServiceName, b.ServiceName)
			}
			return b.SpanCount - a.SpanCount
		}
		return b.ErrorCount - a.ErrorCount
	})

	rootSpan := chooseRootSpan(records)
	durationMs := int64(0)
	if !minStart.IsZero() && !maxEnd.IsZero() && maxEnd.After(minStart) {
		durationMs = maxEnd.Sub(minStart).Milliseconds()
	}
	if durationMs <= 0 {
		durationMs = rootSpan.DurationMs
	}

	return repository.TraceDetail{
		TraceID:         traceID,
		RootServiceName: firstNonBlank(rootSpan.ServiceName, topServiceName(serviceStats)),
		RootSpanName:    rootSpan.Name,
		StartTime:       minStart,
		EndTime:         maxEnd,
		DurationMs:      durationMs,
		SpanCount:       len(records),
		ServiceCount:    len(serviceStats),
		ErrorCount:      errorCount,
		Services:        serviceStats,
		Spans:           records,
	}, nil
}

func (r *Repository) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build tempo request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("request tempo failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read tempo response failed: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("tempo query failed: %s", strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode tempo response failed: %w", err)
	}
	return nil
}

type rawSearchResponse struct {
	Traces  []rawSearchTrace `json:"traces"`
	Results []rawSearchTrace `json:"results"`
	Metrics map[string]any   `json:"metrics"`
}

type rawSearchTrace struct {
	TraceID           string             `json:"traceID"`
	RootServiceName   string             `json:"rootServiceName"`
	RootTraceName     string             `json:"rootTraceName"`
	StartTimeUnixNano string             `json:"startTimeUnixNano"`
	DurationMs        any                `json:"durationMs"`
	SpanSets          []rawSearchSpanSet `json:"spanSets"`
}

type rawSearchSpanSet struct {
	Matched int             `json:"matched"`
	Spans   []rawSearchSpan `json:"spans"`
}

type rawSearchSpan struct {
	SpanID            string         `json:"spanID"`
	Name              string         `json:"name"`
	StartTimeUnixNano string         `json:"startTimeUnixNano"`
	DurationNanos     string         `json:"durationNanos"`
	Attributes        []rawAttribute `json:"attributes"`
}

type rawTraceDetailResponse struct {
	Trace struct {
		ResourceSpans []rawResourceSpan `json:"resourceSpans"`
	} `json:"trace"`
	ResourceSpans []rawResourceSpan `json:"resourceSpans"`
}

type rawResourceSpan struct {
	Resource struct {
		Attributes []rawAttribute `json:"attributes"`
	} `json:"resource"`
	ScopeSpans []rawScopeSpan `json:"scopeSpans"`
}

type rawScopeSpan struct {
	Scope struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"scope"`
	Spans []rawSpan `json:"spans"`
}

type rawSpan struct {
	TraceID           string         `json:"traceId"`
	SpanID            string         `json:"spanId"`
	ParentSpanID      string         `json:"parentSpanId"`
	Name              string         `json:"name"`
	Kind              string         `json:"kind"`
	StartTimeUnixNano string         `json:"startTimeUnixNano"`
	EndTimeUnixNano   string         `json:"endTimeUnixNano"`
	Attributes        []rawAttribute `json:"attributes"`
	Events            []rawEvent     `json:"events"`
	Status            rawStatus      `json:"status"`
}

type rawStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type rawEvent struct {
	Name         string         `json:"name"`
	TimeUnixNano string         `json:"timeUnixNano"`
	Attributes   []rawAttribute `json:"attributes"`
}

type rawAttribute struct {
	Key   string         `json:"key"`
	Value map[string]any `json:"value"`
}

func convertAttributes(items []rawAttribute) []repository.TraceAttribute {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceAttribute, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		out = append(out, repository.TraceAttribute{
			Key:   key,
			Value: normalizeAttributeValue(item.Value),
		})
	}
	slices.SortFunc(out, func(a, b repository.TraceAttribute) int {
		return strings.Compare(a.Key, b.Key)
	})
	return out
}

func remainingResourceAttributes(resourceAttrs []repository.TraceAttribute, spanAttrs []rawAttribute) []repository.TraceAttribute {
	if len(resourceAttrs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(spanAttrs))
	for _, item := range spanAttrs {
		if key := strings.TrimSpace(item.Key); key != "" {
			seen[key] = struct{}{}
		}
	}
	out := make([]repository.TraceAttribute, 0, len(resourceAttrs))
	for _, item := range resourceAttrs {
		if _, exists := seen[item.Key]; exists {
			continue
		}
		out = append(out, item)
	}
	return out
}

func convertEvents(items []rawEvent) []repository.TraceEvent {
	if len(items) == 0 {
		return nil
	}
	out := make([]repository.TraceEvent, 0, len(items))
	for _, item := range items {
		out = append(out, repository.TraceEvent{
			Name:       strings.TrimSpace(item.Name),
			Time:       parseUnixNano(item.TimeUnixNano),
			Attributes: convertAttributes(item.Attributes),
		})
	}
	slices.SortFunc(out, func(a, b repository.TraceEvent) int {
		if a.Time.Equal(b.Time) {
			return strings.Compare(a.Name, b.Name)
		}
		if a.Time.Before(b.Time) {
			return -1
		}
		return 1
	})
	return out
}

func normalizeAttributeValue(value map[string]any) string {
	if len(value) == 0 {
		return ""
	}
	for _, key := range []string{"stringValue", "boolValue", "intValue", "doubleValue", "bytesValue"} {
		if raw, ok := value[key]; ok {
			return fmt.Sprint(raw)
		}
	}
	if raw, ok := value["arrayValue"]; ok {
		return fmt.Sprint(raw)
	}
	if raw, ok := value["kvlistValue"]; ok {
		return fmt.Sprint(raw)
	}
	return fmt.Sprint(value)
}

func firstAttributeValue(items []repository.TraceAttribute, key string) string {
	for _, item := range items {
		if item.Key == key {
			return item.Value
		}
	}
	return ""
}

func buildDepthIndex(records []repository.TraceSpanRecord) map[string]int {
	byID := make(map[string]repository.TraceSpanRecord, len(records))
	for _, record := range records {
		if record.SpanID != "" {
			byID[record.SpanID] = record
		}
	}

	depths := make(map[string]int, len(records))
	var resolve func(string) int
	resolve = func(id string) int {
		if id == "" {
			return 0
		}
		if depth, ok := depths[id]; ok {
			return depth
		}
		record, ok := byID[id]
		if !ok || record.ParentSpanID == "" {
			depths[id] = 0
			return 0
		}
		depth := resolve(record.ParentSpanID) + 1
		depths[id] = depth
		return depth
	}

	for _, record := range records {
		resolve(record.SpanID)
	}
	return depths
}

func chooseRootSpan(records []repository.TraceSpanRecord) repository.TraceSpanRecord {
	if len(records) == 0 {
		return repository.TraceSpanRecord{}
	}
	root := records[0]
	for _, record := range records {
		if record.ParentSpanID == "" && (root.ParentSpanID != "" || record.StartTime.Before(root.StartTime)) {
			root = record
		}
	}
	return root
}

func isErrorStatus(status rawStatus, attributes []repository.TraceAttribute) bool {
	code := strings.ToUpper(strings.TrimSpace(status.Code))
	if code != "" && code != "OK" && code != "UNSET" && code != "STATUS_CODE_OK" && code != "STATUS_CODE_UNSET" {
		return true
	}
	return isErrorAttributes(attributes)
}

func isErrorAttributes(attributes []repository.TraceAttribute) bool {
	for _, item := range attributes {
		switch item.Key {
		case "status":
			if strings.EqualFold(item.Value, "error") {
				return true
			}
		case "http.status_code":
			if number, err := strconv.Atoi(item.Value); err == nil && number >= 500 {
				return true
			}
		case "error":
			if strings.EqualFold(item.Value, "true") {
				return true
			}
		}
	}
	return false
}

func normalizeSearchServiceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "<root span") {
		return ""
	}
	return value
}

func normalizeEnumLabel(value string, prefix string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.TrimPrefix(value, prefix)
}

func normalizeTraceID(value string) string {
	return normalizeTelemetryIDWithSize(value, 16)
}

func normalizeSpanID(value string) string {
	return normalizeTelemetryIDWithSize(value, 8)
}

func normalizeTelemetryIDWithSize(value string, expectedBytes int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if _, err := hex.DecodeString(value); err == nil {
		return normalizeHexLength(strings.ToLower(value), expectedBytes)
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return normalizeHexLength(strings.ToLower(value), expectedBytes)
	}
	return hex.EncodeToString(decoded)
}

func normalizeHexLength(value string, expectedBytes int) string {
	expectedLength := expectedBytes * 2
	if expectedLength <= 0 || len(value) >= expectedLength {
		return value
	}
	return strings.Repeat("0", expectedLength-len(value)) + value
}

func parseUnixNano(value string) time.Time {
	nanos := parseInt64(value)
	if nanos <= 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos).UTC()
}

func parseDurationNanos(start string, end string) int64 {
	startNanos := parseInt64(start)
	endNanos := parseInt64(end)
	if endNanos <= startNanos {
		return 0
	}
	return endNanos - startNanos
}

func parseInt64(value any) int64 {
	switch typed := value.(type) {
	case nil:
		return 0
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return 0
		}
		number, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return number
		}
		floatNumber, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0
		}
		return int64(floatNumber)
	case float64:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		number, err := typed.Int64()
		if err == nil {
			return number
		}
		floatNumber, err := typed.Float64()
		if err != nil {
			return 0
		}
		return int64(floatNumber)
	default:
		return 0
	}
}

func nanosToMilliseconds(nanos int64) int64 {
	if nanos <= 0 {
		return 0
	}
	ms := nanos / int64(time.Millisecond)
	if ms == 0 {
		return 1
	}
	return ms
}

func topServiceName(items []repository.TraceServiceStat) string {
	if len(items) == 0 {
		return ""
	}
	return items[0].ServiceName
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
