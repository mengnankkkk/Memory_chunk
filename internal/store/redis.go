package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type PageStore interface {
	SavePage(ctx context.Context, key string, content string) error
	LoadPage(ctx context.Context, key string) (string, error)
	LoadResolvedPage(ctx context.Context, key string) (ResolvedPage, error)
	SaveSummary(ctx context.Context, key string, result SummaryResult) error
	Close() error
}

type SummaryJobQueue interface {
	EnqueueSummaryJob(ctx context.Context, job SummaryJob) error
}

type SummaryJobConsumer interface {
	EnsureSummaryGroup(ctx context.Context, group string) error
	ConsumeSummaryJobs(ctx context.Context, group string, consumer string, count int64, block time.Duration) ([]SummaryJobMessage, error)
	AckSummaryJob(ctx context.Context, group string, messageID string) error
}

type SummaryFragment struct {
	Type     string `json:"type"`
	Content  string `json:"content"`
	Language string `json:"language,omitempty"`
}

type SummaryJob struct {
	JobID         string            `json:"job_id"`
	SessionID     string            `json:"session_id"`
	RequestID     string            `json:"request_id"`
	Policy        string            `json:"policy"`
	ChunkID       string            `json:"chunk_id"`
	Source        string            `json:"source"`
	ContentHash   string            `json:"content_hash"`
	PageRefs      []string          `json:"page_refs"`
	Fragments     []SummaryFragment `json:"fragments"`
	Content       string            `json:"content"`
	TargetTokens  int               `json:"target_tokens"`
	CurrentTokens int               `json:"current_tokens"`
	CreatedAt     time.Time         `json:"created_at"`
}

type SummaryJobMessage struct {
	ID  string
	Job SummaryJob
}

type SummaryResult struct {
	JobID     string    `json:"job_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ResolvedPage struct {
	Key          string
	Content      string
	IsSummary    bool
	SummaryJobID string
}

type RedisConfig struct {
	Addr          string
	Username      string
	Password      string
	DB            int
	KeyPrefix     string
	PageTTL       time.Duration
	SummaryStream string
}

type RedisStore struct {
	client        *redis.Client
	keyPrefix     string
	pageTTL       time.Duration
	summaryStream string
}

func NewRedisStore(ctx context.Context, cfg RedisConfig) (*RedisStore, error) {
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

	return &RedisStore{
		client:        client,
		keyPrefix:     strings.TrimSuffix(cfg.KeyPrefix, ":"),
		pageTTL:       cfg.PageTTL,
		summaryStream: cfg.SummaryStream,
	}, nil
}

func (s *RedisStore) SavePage(ctx context.Context, key string, content string) error {
	return s.client.Set(ctx, s.prefixed(key), content, s.pageTTL).Err()
}

func (s *RedisStore) LoadPage(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, s.prefixed(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", fmt.Errorf("page not found: %s", key)
		}
		return "", err
	}
	return value, nil
}

func (s *RedisStore) LoadResolvedPage(ctx context.Context, key string) (ResolvedPage, error) {
	if result, err := s.loadSummary(ctx, key); err == nil {
		return ResolvedPage{
			Key:          key,
			Content:      result.Content,
			IsSummary:    true,
			SummaryJobID: result.JobID,
		}, nil
	}
	value, err := s.LoadPage(ctx, key)
	if err != nil {
		return ResolvedPage{}, err
	}
	return ResolvedPage{
		Key:       key,
		Content:   value,
		IsSummary: false,
	}, nil
}

func (s *RedisStore) SaveSummary(ctx context.Context, key string, result SummaryResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal summary result failed: %w", err)
	}
	return s.client.Set(ctx, s.summaryKey(key), payload, s.pageTTL).Err()
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}

func (s *RedisStore) EnqueueSummaryJob(ctx context.Context, job SummaryJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal summary job failed: %w", err)
	}
	return s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.summaryStream,
		Values: map[string]any{
			"job_id":     job.JobID,
			"request_id": job.RequestID,
			"chunk_id":   job.ChunkID,
			"payload":    string(payload),
		},
	}).Err()
}

func (s *RedisStore) EnsureSummaryGroup(ctx context.Context, group string) error {
	err := s.client.XGroupCreateMkStream(ctx, s.summaryStream, group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (s *RedisStore) ConsumeSummaryJobs(ctx context.Context, group string, consumer string, count int64, block time.Duration) ([]SummaryJobMessage, error) {
	streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{s.summaryStream, ">"},
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
	out := make([]SummaryJobMessage, 0)
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			payload, _ := msg.Values["payload"].(string)
			if strings.TrimSpace(payload) == "" {
				continue
			}
			var job SummaryJob
			if err := json.Unmarshal([]byte(payload), &job); err != nil {
				return nil, fmt.Errorf("unmarshal summary job failed: %w", err)
			}
			out = append(out, SummaryJobMessage{
				ID:  msg.ID,
				Job: job,
			})
		}
	}
	return out, nil
}

func (s *RedisStore) AckSummaryJob(ctx context.Context, group string, messageID string) error {
	return s.client.XAck(ctx, s.summaryStream, group, messageID).Err()
}

func (s *RedisStore) prefixed(key string) string {
	return s.keyPrefix + ":" + key
}

func (s *RedisStore) summaryKey(key string) string {
	return s.keyPrefix + ":summary:" + key
}

func (s *RedisStore) loadSummary(ctx context.Context, key string) (SummaryResult, error) {
	value, err := s.client.Get(ctx, s.summaryKey(key)).Result()
	if err != nil {
		return SummaryResult{}, err
	}
	var result SummaryResult
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return SummaryResult{}, err
	}
	return result, nil
}
