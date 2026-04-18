package httpcontroller

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"context-refiner/internal/domain/core/repository"
	"context-refiner/internal/service"
)

//go:embed assets/*
var dashboardAssets embed.FS

type DashboardController struct {
	service  *service.DashboardService
	refine   *RefineController
	staticFS http.FileSystem
}

func NewDashboardController(svc *service.DashboardService, refiner *service.RefinerApplicationService) *DashboardController {
	sub, err := fs.Sub(dashboardAssets, "assets")
	if err != nil {
		panic(err)
	}
	var refineCtl *RefineController
	if refiner != nil {
		refineCtl = NewRefineController(refiner)
	}
	return &DashboardController{
		service:  svc,
		refine:   refineCtl,
		staticFS: http.FS(sub),
	}
}

func (c *DashboardController) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/snapshot", c.handleSnapshot)
	mux.HandleFunc("/api/page", c.handlePageDetail)
	mux.HandleFunc("/api/traces/search", c.handleTraceSearch)
	mux.HandleFunc("/api/traces/detail", c.handleTraceDetail)
	mux.HandleFunc("/api/traces/evaluation", c.handleTraceEvaluation)
	if c.refine != nil {
		mux.HandleFunc("/api/refine", c.refine.Handle)
	}
	mux.Handle("/", http.FileServer(c.staticFS))
	return mux
}

func (c *DashboardController) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "只支持 GET 请求")
		return
	}

	snapshot, err := c.service.Snapshot(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (c *DashboardController) handlePageDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "只支持 GET 请求")
		return
	}

	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		writeJSONError(w, http.StatusBadRequest, "必须提供 page key")
		return
	}

	detail, err := c.service.PageDetail(r.Context(), key)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (c *DashboardController) handleTraceSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "只支持 GET 请求")
		return
	}

	query, err := buildTraceSearchQuery(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := c.service.SearchTraces(r.Context(), query)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (c *DashboardController) handleTraceDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "只支持 GET 请求")
		return
	}

	traceID := strings.TrimSpace(r.URL.Query().Get("id"))
	if traceID == "" {
		writeJSONError(w, http.StatusBadRequest, "必须提供 trace id")
		return
	}

	start, err := parseOptionalInt64Query(r, "start")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	end, err := parseOptionalInt64Query(r, "end")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	detail, err := c.service.TraceDetail(r.Context(), traceID, start, end)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (c *DashboardController) handleTraceEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "只支持 GET 请求")
		return
	}

	traceID := strings.TrimSpace(r.URL.Query().Get("id"))
	if traceID == "" {
		writeJSONError(w, http.StatusBadRequest, "必须提供 trace id")
		return
	}

	evaluation, err := c.service.TraceEvaluation(r.Context(), traceID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, evaluation)
}

func buildTraceSearchQuery(r *http.Request) (repository.TraceSearchQuery, error) {
	limit, err := parseOptionalIntQuery(r, "limit")
	if err != nil {
		return repository.TraceSearchQuery{}, err
	}
	start, err := parseOptionalInt64Query(r, "start")
	if err != nil {
		return repository.TraceSearchQuery{}, err
	}
	end, err := parseOptionalInt64Query(r, "end")
	if err != nil {
		return repository.TraceSearchQuery{}, err
	}
	spss, err := parseOptionalIntQuery(r, "spss")
	if err != nil {
		return repository.TraceSearchQuery{}, err
	}

	return repository.TraceSearchQuery{
		Query:           strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:           limit,
		Start:           start,
		End:             end,
		SpansPerSpanSet: spss,
	}, nil
}

func parseOptionalIntQuery(r *http.Request, key string) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return 0, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s 参数格式不正确", key)
	}
	return number, nil
}

func parseOptionalInt64Query(r *http.Request, key string) (int64, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return 0, nil
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s 参数格式不正确", key)
	}
	return number, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{
		"error": message,
	})
}
