package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"

	grpcadapter "context-refiner/internal/adapter/grpc"
	"context-refiner/internal/core/repository"
	"context-refiner/internal/infra/config"
	redisstore "context-refiner/internal/infra/store/redis"
	"context-refiner/internal/infra/summary"
	"context-refiner/internal/infra/tokenizer"
	"context-refiner/internal/service"

	"google.golang.org/grpc"
)

type AppRuntime struct {
	Cfg             *config.AppConfig
	PageRepository  repository.PageRepository
	RedisRepository *redisstore.RedisRepository
	GRPCServer      *grpc.Server
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
	redisRepository, err := newPageRepository(ctx, cfg)
	if err != nil {
		return nil, err
	}

	registry := buildRegistry(counter, redisRepository, redisRepository, cfg.Pipeline.PagingTokenThreshold)
	grpcServer := grpc.NewServer()
	refinerService := service.NewRefinerApplicationService(
		registry,
		counter,
		redisRepository,
		policies,
		cfg.Pipeline.DefaultPolicy,
	)
	grpcadapter.RegisterRefinerService(grpcServer, refinerService)

	return &AppRuntime{
		Cfg:             cfg,
		PageRepository:  redisRepository,
		RedisRepository: redisRepository,
		GRPCServer:      grpcServer,
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

func newPageRepository(ctx context.Context, cfg *config.AppConfig) (*redisstore.RedisRepository, error) {
	pageStore, err := redisstore.NewRedisRepository(ctx, redisstore.Config{
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

func StartSummaryWorker(ctx context.Context, runtime *AppRuntime) {
	if !runtime.Cfg.SummaryWorker.Enabled {
		return
	}
	worker := summary.NewWorker(
		runtime.RedisRepository,
		runtime.PageRepository,
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
