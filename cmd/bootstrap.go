package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/config"
	"context-refiner/internal/engine"
	"context-refiner/internal/processor"
	"context-refiner/internal/server"
	"context-refiner/internal/store"
	"context-refiner/internal/summary"
	"context-refiner/internal/tokenizer"

	"google.golang.org/grpc"
)

type appRuntime struct {
	cfg        *config.AppConfig
	policies   map[string]engine.RuntimePolicy
	counter    engine.TokenCounter
	pageStore  *store.RedisStore
	registry   *engine.Registry
	grpcServer *grpc.Server
}

func loadRuntime(ctx context.Context, configPath string) (*appRuntime, error) {
	cfg, err := config.LoadAppConfig(configPath)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
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
	pageStore, err := newPageStore(ctx, cfg)
	if err != nil {
		return nil, err
	}

	registry := buildRegistry(counter, pageStore, pageStore, cfg.Pipeline.PagingTokenThreshold)
	grpcServer := grpc.NewServer()
	refinerv1.RegisterRefinerServiceServer(grpcServer, server.NewRefinerServer(
		registry,
		counter,
		pageStore,
		policies,
		cfg.Pipeline.DefaultPolicy,
	))

	return &appRuntime{
		cfg:        cfg,
		policies:   policies,
		counter:    counter,
		pageStore:  pageStore,
		registry:   registry,
		grpcServer: grpcServer,
	}, nil
}

func newPageStore(ctx context.Context, cfg *config.AppConfig) (*store.RedisStore, error) {
	pageStore, err := store.NewRedisStore(ctx, store.RedisConfig{
		Addr:          cfg.Redis.Addr,
		Username:      cfg.Redis.Username,
		Password:      cfg.Redis.Password,
		DB:            cfg.Redis.DB,
		KeyPrefix:     cfg.Redis.KeyPrefix,
		PageTTL:       cfg.Redis.PageTTL,
		SummaryStream: cfg.Redis.SummaryStream,
	})
	if err != nil {
		return nil, fmt.Errorf("init redis store failed: %w", err)
	}
	return pageStore, nil
}

func buildRegistry(counter engine.TokenCounter, pageStore store.PageStore, summaryQueue store.SummaryJobQueue, pagingLimit int) *engine.Registry {
	registry := engine.NewRegistry()
	for _, item := range []engine.Processor{
		processor.NewPagingProcessor(counter, pageStore, pagingLimit),
		processor.NewCollapseProcessor(counter),
		processor.NewCompactProcessor(counter),
		processor.NewJSONTrimProcessor(counter),
		processor.NewTableReduceProcessor(counter),
		processor.NewCodeOutlineProcessor(counter),
		processor.NewErrorStackFocusProcessor(counter),
		processor.NewSnipProcessor(counter),
		processor.NewAutoCompactSyncProcessor(counter),
		processor.NewAutoCompactAsyncProcessor(counter, summaryQueue),
		processor.NewAssembleProcessor(counter),
	} {
		registry.MustRegister(item)
	}
	return registry
}

func startSummaryWorker(ctx context.Context, runtime *appRuntime) {
	if !runtime.cfg.SummaryWorker.Enabled {
		return
	}
	worker := summary.NewWorker(
		runtime.pageStore,
		runtime.pageStore,
		runtime.cfg.SummaryWorker.ConsumerGroup,
		runtime.cfg.SummaryWorker.ConsumerName,
		runtime.cfg.SummaryWorker.BatchSize,
		runtime.cfg.SummaryWorker.BlockTimeout,
	)
	go func() {
		if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("summary worker stopped with error: %v", err)
		}
	}()
}
