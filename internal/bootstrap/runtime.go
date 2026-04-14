package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	redisstore "context-refiner/internal/adapter/outbound/redis"
	"context-refiner/internal/adapter/outbound/summary"
	grpccontroller "context-refiner/internal/controller/grpc"
	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/repository"
	"context-refiner/internal/infra/config"
	"context-refiner/internal/observability"
	metricsobs "context-refiner/internal/observability/metrics"
	apptracing "context-refiner/internal/observability/tracing"
	"context-refiner/internal/service"
	"context-refiner/internal/support/tokenizer"

	"google.golang.org/grpc"
)

type AppRuntime struct {
	Cfg             *config.AppConfig
	PageRepository  repository.PageRepository
	RedisRepository *redisstore.RedisRepository
	GRPCServer      *grpc.Server
	MetricsRecorder observability.Recorder
	MetricsServer   *http.Server
	TraceShutdown   func(context.Context) error
}

func LoadRuntime(ctx context.Context, configPath string) (*AppRuntime, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}
	policies, err := config.LoadPolicies(cfg.Pipeline.PolicyFile)
	if err != nil {
		return nil, err
	}
	counter, err := tokenizer.NewCounter(cfg.Tokenizer.Model, cfg.Tokenizer.FallbackEncoding)
	if err != nil {
		return nil, err
	}
	metricsRecorder, metricsServer, err := newMetricsRuntime(cfg)
	if err != nil {
		return nil, err
	}
	traceShutdown, err := newTracingRuntime(ctx, cfg)
	if err != nil {
		return nil, err
	}
	redisRepository, err := newPageRepository(ctx, cfg, metricsRecorder)
	if err != nil {
		return nil, err
	}
	if err := prewarmPrefixCache(ctx, cfg, counter, redisRepository); err != nil {
		return nil, err
	}

	registry := buildRegistry(counter, redisRepository, redisRepository, redisRepository, buildPrefixCachePolicy(cfg), cfg.Pipeline.PagingTokenThreshold)
	grpcServer := grpc.NewServer()
	refinerService := service.NewRefinerApplicationService(
		registry,
		counter,
		redisRepository,
		metricsRecorder,
		policies,
		cfg.Pipeline.DefaultPolicy,
	)
	grpccontroller.RegisterRefinerService(grpcServer, refinerService)

	return &AppRuntime{
		Cfg:             cfg,
		PageRepository:  redisRepository,
		RedisRepository: redisRepository,
		GRPCServer:      grpcServer,
		MetricsRecorder: metricsRecorder,
		MetricsServer:   metricsServer,
		TraceShutdown:   traceShutdown,
	}, nil
}

func loadConfig(path string) (*config.AppConfig, error) {
	cfg, err := config.LoadAppConfig(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func newPageRepository(ctx context.Context, cfg *config.AppConfig, metricsRecorder observability.Recorder) (*redisstore.RedisRepository, error) {
	pageStore, err := redisstore.NewRedisRepository(ctx, redisstore.Config{
		Addr:           cfg.Redis.Addr,
		Username:       cfg.Redis.Username,
		Password:       cfg.Redis.Password,
		DB:             cfg.Redis.DB,
		KeyPrefix:      cfg.Redis.KeyPrefix,
		PageTTL:        cfg.Redis.PageTTL,
		PrefixCacheTTL: cfg.Redis.PrefixCacheTTL,
		HotThreshold:   cfg.PrefixCache.HotThreshold,
		HotTTL:         cfg.PrefixCache.HotTTL,
		SummaryStream:  cfg.Redis.SummaryStream,
	}, metricsRecorder)
	if err != nil {
		return nil, fmt.Errorf("init redis store failed: %w", err)
	}
	return pageStore, nil
}

func newMetricsRuntime(cfg *config.AppConfig) (observability.Recorder, *http.Server, error) {
	if !cfg.Observability.MetricsEnabled {
		return observability.NewNopRecorder(), nil, nil
	}
	recorder, err := metricsobs.NewPrometheusRecorder()
	if err != nil {
		return nil, nil, fmt.Errorf("init prometheus recorder failed: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle(cfg.Observability.MetricsPath, recorder.Handler())
	server := &http.Server{
		Addr:    cfg.Observability.MetricsListenAddr,
		Handler: mux,
	}
	return recorder, server, nil
}

func newTracingRuntime(ctx context.Context, cfg *config.AppConfig) (func(context.Context) error, error) {
	shutdown, err := apptracing.NewProvider(ctx, apptracing.Config{
		Enabled:     cfg.Observability.TracingEnabled,
		ServiceName: cfg.Observability.ServiceName,
		Endpoint:    cfg.Observability.TracingEndpoint,
		Insecure:    cfg.Observability.TracingInsecure,
		SampleRate:  cfg.Observability.TracingSampleRate,
	})
	if err != nil {
		return nil, fmt.Errorf("init tracing provider failed: %w", err)
	}
	return shutdown, nil
}

func buildPrefixCachePolicy(cfg *config.AppConfig) core.PrefixCachePolicy {
	return core.PrefixCachePolicy{
		MinStablePrefixTokens: cfg.PrefixCache.MinStablePrefixTokens,
		MinSegmentCount:       cfg.PrefixCache.MinSegmentCount,
		DefaultTenant:         cfg.PrefixCache.DefaultTenant,
		Namespace: core.PrefixNamespaceConfig{
			IncludePolicy: cfg.PrefixCache.Namespace.IncludePolicy,
			IncludeModel:  cfg.PrefixCache.Namespace.IncludeModel,
			IncludeTenant: cfg.PrefixCache.Namespace.IncludeTenant,
		},
	}
}

func prewarmPrefixCache(ctx context.Context, cfg *config.AppConfig, counter core.TokenCounter, store *redisstore.RedisRepository) error {
	for _, item := range cfg.PrefixCache.Prewarm {
		identity := core.BuildPrefixCacheIdentityFromSegments(item.ModelID, item.SystemPrompt, item.MemoryPrompt, item.RAGPrompt, counter)
		if identity.CombinedPrefixHash == "" {
			continue
		}
		namespace := core.BuildPrefixNamespace(item.Policy, firstNonBlank(item.Tenant, cfg.PrefixCache.DefaultTenant), identity.ModelID, core.PrefixNamespaceConfig{
			IncludePolicy: cfg.PrefixCache.Namespace.IncludePolicy,
			IncludeModel:  cfg.PrefixCache.Namespace.IncludeModel,
			IncludeTenant: cfg.PrefixCache.Namespace.IncludeTenant,
		})
		if _, err := store.RegisterPrefix(ctx, repository.PrefixCacheEntry{
			Key:                  identity.CombinedPrefixHash,
			SessionScope:         "prewarm|" + item.Name,
			Namespace:            namespace,
			ModelID:              identity.ModelID,
			PrefixHash:           identity.CombinedPrefixHash,
			SystemPrefixHash:     identity.SystemPrefixHash,
			MemoryPrefixHash:     identity.MemoryPrefixHash,
			RAGPrefixHash:        identity.RAGPrefixHash,
			StablePrefixTokens:   identity.StablePrefixTokens,
			SystemPrefixTokens:   identity.SystemPrefixTokens,
			MemoryPrefixTokens:   identity.MemoryPrefixTokens,
			RAGPrefixTokens:      identity.RAGPrefixTokens,
			PromptLayoutVersion:  "stable-prefix-v2",
			ArtifactKeyVersion:   "content-addressed-v1",
			CacheOptimizationAim: "prefix-reuse-stability",
			NormalizationVersion: identity.NormalizationVersion,
		}); err != nil {
			return fmt.Errorf("prewarm prefix cache %s failed: %w", item.Name, err)
		}
	}
	return nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func StartSummaryWorker(ctx context.Context, runtime *AppRuntime) {
	if !runtime.Cfg.SummaryWorker.Enabled {
		return
	}
	worker := summary.NewWorker(
		runtime.RedisRepository,
		runtime.PageRepository,
		summary.NewHeuristicProvider(),
		runtime.MetricsRecorder,
		runtime.Cfg.SummaryWorker.ConsumerGroup,
		runtime.Cfg.SummaryWorker.ConsumerName,
		runtime.Cfg.SummaryWorker.BatchSize,
		runtime.Cfg.SummaryWorker.BlockTimeout,
	)
	go func() {
		if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("summary worker stopped with error: %v", err)
		}
	}()
}

func StartMetricsServer(ctx context.Context, runtime *AppRuntime) {
	if runtime.MetricsServer == nil {
		return
	}
	go func() {
		<-ctx.Done()
		if err := runtime.MetricsServer.Shutdown(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("metrics server shutdown failed: %v", err)
		}
	}()
	go func() {
		log.Printf("metrics HTTP server listening on %s%s", runtime.Cfg.Observability.MetricsListenAddr, runtime.Cfg.Observability.MetricsPath)
		if err := runtime.MetricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("metrics server stopped with error: %v", err)
		}
	}()
}
