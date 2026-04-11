package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"context-refiner/internal/core/repository"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr          string
	Username      string
	Password      string
	DB            int
	KeyPrefix     string
	PageTTL       time.Duration
	SummaryStream string
}

type RedisRepository struct {
	client        *redis.Client
	keyPrefix     string
	pageTTL       time.Duration
	summaryStream string
}

var _ repository.PageRepository = (*RedisRepository)(nil)
var _ repository.SummaryJobRepository = (*RedisRepository)(nil)
var _ repository.SummaryJobConsumer = (*RedisRepository)(nil)

func NewRedisRepository(ctx context.Context, cfg Config) (*RedisRepository, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, errors.New("redis.addr is required")
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		cfg.KeyPrefix = "context-refiner:page"
	}
	if cfg.PageTTL <= 0 {
		cfg.PageTTL = 24 * time.Hour
	}
	if strings.TrimSpace(cfg.SummaryStream) == "" {
		cfg.SummaryStream = "context-refiner:summary-jobs"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis failed: %w", err)
	}

	return &RedisRepository{
		client:        client,
		keyPrefix:     strings.TrimSuffix(cfg.KeyPrefix, ":"),
		pageTTL:       cfg.PageTTL,
		summaryStream: cfg.SummaryStream,
	}, nil
}

func (r *RedisRepository) SavePage(ctx context.Context, key string, content string) error {
	return r.client.Set(ctx, r.prefixed(key), content, r.pageTTL).Err()
}

func (r *RedisRepository) LoadPage(ctx context.Context, key string) (string, error) {
	value, err := r.client.Get(ctx, r.prefixed(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", fmt.Errorf("page not found: %s", key)
		}
		return "", err
	}
	return value, nil
}

func (r *RedisRepository) LoadResolvedPage(ctx context.Context, key string) (repository.ResolvedPage, error) {
	if result, err := r.loadSummary(ctx, key); err == nil {
		return repository.ResolvedPage{
			Key:          key,
			Content:      result.Content,
			IsSummary:    true,
			SummaryJobID: result.JobID,
		}, nil
	}
	value, err := r.LoadPage(ctx, key)
	if err != nil {
		return repository.ResolvedPage{}, err
	}
	return repository.ResolvedPage{
		Key:       key,
		Content:   value,
		IsSummary: false,
	}, nil
}

func (r *RedisRepository) SaveSummary(ctx context.Context, key string, result repository.SummaryResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal summary result failed: %w", err)
	}
	return r.client.Set(ctx, r.summaryKey(key), payload, r.pageTTL).Err()
}

func (r *RedisRepository) EnqueueSummaryJob(ctx context.Context, job repository.SummaryJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal summary job failed: %w", err)
	}
	return r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: r.summaryStream,
		Values: map[string]any{
			"job_id":     job.JobID,
			"request_id": job.RequestID,
			"chunk_id":   job.ChunkID,
			"payload":    string(payload),
		},
	}).Err()
}

func (r *RedisRepository) EnsureSummaryGroup(ctx context.Context, group string) error {
	err := r.client.XGroupCreateMkStream(ctx, r.summaryStream, group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (r *RedisRepository) ConsumeSummaryJobs(ctx context.Context, group string, consumer string, count int64, block time.Duration) ([]repository.SummaryJobMessage, error) {
	streams, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{r.summaryStream, ">"},
		Count:    count,
		Block:    block,
		NoAck:    false,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]repository.SummaryJobMessage, 0)
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			payload, _ := msg.Values["payload"].(string)
			if strings.TrimSpace(payload) == "" {
				continue
			}
			var job repository.SummaryJob
			if err := json.Unmarshal([]byte(payload), &job); err != nil {
				return nil, fmt.Errorf("unmarshal summary job failed: %w", err)
			}
			out = append(out, repository.SummaryJobMessage{
				ID:  msg.ID,
				Job: job,
			})
		}
	}
	return out, nil
}

func (r *RedisRepository) AckSummaryJob(ctx context.Context, group string, messageID string) error {
	return r.client.XAck(ctx, r.summaryStream, group, messageID).Err()
}

func (r *RedisRepository) Close() error {
	return r.client.Close()
}

func (r *RedisRepository) prefixed(key string) string {
	return r.keyPrefix + ":" + key
}

func (r *RedisRepository) summaryKey(key string) string {
	return r.keyPrefix + ":summary:" + key
}

func (r *RedisRepository) loadSummary(ctx context.Context, key string) (repository.SummaryResult, error) {
	value, err := r.client.Get(ctx, r.summaryKey(key)).Result()
	if err != nil {
		return repository.SummaryResult{}, err
	}
	var result repository.SummaryResult
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return repository.SummaryResult{}, err
	}
	return result, nil
}
