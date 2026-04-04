package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadAppConfig("config/service.yaml")
	if err != nil {
		log.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	policies, err := config.LoadPolicies(cfg.Pipeline.PolicyFile)
	if err != nil {
		log.Fatal(err)
	}

	counter, err := tokenizer.NewCounter(cfg.Tokenizer.Model, cfg.Tokenizer.FallbackEncoding)
	if err != nil {
		log.Fatal(err)
	}

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
		log.Fatal(err)
	}
	defer func() {
		if closeErr := pageStore.Close(); closeErr != nil {
			log.Printf("close redis store failed: %v", closeErr)
		}
	}()

	registry := buildRegistry(counter, pageStore, pageStore, cfg.Pipeline.PagingTokenThreshold)
	grpcServer := grpc.NewServer()
	refinerv1.RegisterRefinerServiceServer(grpcServer, server.NewRefinerServer(
		registry,
		counter,
		pageStore,
		policies,
		cfg.Pipeline.DefaultPolicy,
	))

	lis, err := net.Listen("tcp", cfg.GRPC.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	if cfg.SummaryWorker.Enabled {
		worker := summary.NewWorker(
			pageStore,
			pageStore,
			cfg.SummaryWorker.ConsumerGroup,
			cfg.SummaryWorker.ConsumerName,
			cfg.SummaryWorker.BatchSize,
			cfg.SummaryWorker.BlockTimeout,
		)
		go func() {
			if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("summary worker stopped with error: %v", err)
			}
		}()
	}

	log.Printf("refiner gRPC server listening on %s", cfg.GRPC.ListenAddr)
	if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		log.Fatal(err)
	}
}

func buildRegistry(counter engine.TokenCounter, pageStore store.PageStore, summaryQueue store.SummaryJobQueue, pagingLimit int) *engine.Registry {
	registry := engine.NewRegistry()
	processors := []engine.Processor{
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
	}
	for _, item := range processors {
		registry.MustRegister(item)
	}
	return registry
}
