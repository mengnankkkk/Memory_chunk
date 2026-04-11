package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"context-refiner/internal/core/repository"
	"context-refiner/internal/observability"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
)

type Config struct {
	Addr           string
	Username       string
	Password       string
	DB             int
	KeyPrefix      string
	PageTTL        time.Duration
	PrefixCacheTTL time.Duration
	HotThreshold   int64
	HotTTL         time.Duration
	SummaryStream  string
}

type RedisRepository struct {
	client         *redis.Client
	keyPrefix      string
	pageTTL        time.Duration
	prefixCacheTTL time.Duration
	hotThreshold   int64
	hotTTL         time.Duration
	summaryStream  string
	metrics        observability.Recorder
}

var _ repository.PageRepository = (*RedisRepository)(nil)
var _ repository.SummaryJobRepository = (*RedisRepository)(nil)
var _ repository.SummaryJobConsumer = (*RedisRepository)(nil)
var _ repository.PrefixCacheRepository = (*RedisRepository)(nil)

func NewRedisRepository(ctx context.Context, cfg Config, metrics observability.Recorder) (*RedisRepository, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, errors.New("redis.addr is required")
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		cfg.KeyPrefix = "context-refiner:page"
	}
	if cfg.PageTTL <= 0 {
		cfg.PageTTL = 24 * time.Hour
	}
	if cfg.PrefixCacheTTL <= 0 {
		cfg.PrefixCacheTTL = 24 * time.Hour
	}
	if strings.TrimSpace(cfg.SummaryStream) == "" {
		cfg.SummaryStream = "context-refiner:summary-jobs"
	}
	if cfg.HotThreshold <= 0 {
		cfg.HotThreshold = 5
	}
	if cfg.HotTTL <= 0 {
		cfg.HotTTL = 72 * time.Hour
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
		client:         client,
		keyPrefix:      strings.TrimSuffix(cfg.KeyPrefix, ":"),
		pageTTL:        cfg.PageTTL,
		prefixCacheTTL: cfg.PrefixCacheTTL,
		hotThreshold:   cfg.HotThreshold,
		hotTTL:         cfg.HotTTL,
		summaryStream:  cfg.SummaryStream,
		metrics:        defaultMetrics(metrics),
	}, nil
}

func (r *RedisRepository) SavePage(ctx context.Context, key string, content string) error {
	ctx, span := otel.Tracer("context-refiner/infra/store/redis").Start(ctx, "redis.save_page")
	defer span.End()
	span.SetAttributes(
		attribute.String("page.key", key),
		attribute.Int("page.content_length", len(content)),
	)

	created, err := r.client.SetNX(ctx, r.prefixed(key), content, r.pageTTL).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		r.metrics.ObservePageArtifactWrite("error")
		return err
	}
	if created {
		span.SetAttributes(attribute.String("page.write_result", "created"))
		r.metrics.ObservePageArtifactWrite("created")
		return nil
	}
	if err := r.client.Expire(ctx, r.prefixed(key), r.pageTTL).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		r.metrics.ObservePageArtifactWrite("error")
		return err
	}
	span.SetAttributes(attribute.String("page.write_result", "reused"))
	r.metrics.ObservePageArtifactWrite("reused")
	return nil
}

func (r *RedisRepository) LoadPage(ctx context.Context, key string) (string, error) {
	ctx, span := otel.Tracer("context-refiner/infra/store/redis").Start(ctx, "redis.load_page")
	defer span.End()
	span.SetAttributes(attribute.String("page.key", key))

	value, err := r.client.Get(ctx, r.prefixed(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			err = fmt.Errorf("page not found: %s", key)
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			return "", err
		}
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return "", err
	}
	span.SetAttributes(attribute.Int("page.content_length", len(value)))
	return value, nil
}

func (r *RedisRepository) LoadResolvedPage(ctx context.Context, key string) (repository.ResolvedPage, error) {
	ctx, span := otel.Tracer("context-refiner/infra/store/redis").Start(ctx, "redis.load_resolved_page")
	defer span.End()
	span.SetAttributes(attribute.String("page.key", key))

	if result, err := r.loadSummary(ctx, key); err == nil {
		span.SetAttributes(
			attribute.String("page.resolve_result", "summary"),
			attribute.String("summary.job_id", result.JobID),
		)
		r.metrics.ObserveStoreLoad("summary")
		return repository.ResolvedPage{
			Key:          key,
			Content:      result.Content,
			IsSummary:    true,
			SummaryJobID: result.JobID,
		}, nil
	}
	value, err := r.client.Get(ctx, r.prefixed(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			err = fmt.Errorf("page not found: %s", key)
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			span.SetAttributes(attribute.String("page.resolve_result", "miss"))
			r.metrics.ObserveStoreLoad("miss")
			return repository.ResolvedPage{}, err
		}
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		span.SetAttributes(attribute.String("page.resolve_result", "error"))
		r.metrics.ObserveStoreLoad("error")
		return repository.ResolvedPage{}, err
	}
	span.SetAttributes(
		attribute.String("page.resolve_result", "page"),
		attribute.Int("page.content_length", len(value)),
	)
	r.metrics.ObserveStoreLoad("page")
	return repository.ResolvedPage{
		Key:       key,
		Content:   value,
		IsSummary: false,
	}, nil
}

func (r *RedisRepository) SaveSummary(ctx context.Context, key string, result repository.SummaryResult) error {
	ctx, span := otel.Tracer("context-refiner/infra/store/redis").Start(ctx, "redis.save_summary")
	defer span.End()
	span.SetAttributes(
		attribute.String("page.key", key),
		attribute.String("summary.job_id", result.JobID),
	)

	payload, err := json.Marshal(result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return fmt.Errorf("marshal summary result failed: %w", err)
	}
	if err := r.client.Set(ctx, r.summaryKey(key), payload, r.pageTTL).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return err
	}
	return nil
}

func (r *RedisRepository) EnqueueSummaryJob(ctx context.Context, job repository.SummaryJob) error {
	ctx, span := otel.Tracer("context-refiner/infra/store/redis").Start(ctx, "redis.enqueue_summary_job")
	defer span.End()
	span.SetAttributes(
		attribute.String("summary.job_id", job.JobID),
		attribute.String("summary.chunk_id", job.ChunkID),
		attribute.Int("summary.page_ref_count", len(job.PageRefs)),
	)

	payload, err := json.Marshal(job)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return fmt.Errorf("marshal summary job failed: %w", err)
	}
	err = r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: r.summaryStream,
		Values: map[string]any{
			"job_id":     job.JobID,
			"request_id": job.RequestID,
			"chunk_id":   job.ChunkID,
			"payload":    string(payload),
		},
	}).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return err
	}
	r.metrics.ObserveSummaryJob("enqueued")
	return nil
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

func (r *RedisRepository) RegisterPrefix(ctx context.Context, entry repository.PrefixCacheEntry) (repository.PrefixCacheRegistration, error) {
	ctx, span := otel.Tracer("context-refiner/infra/store/redis").Start(ctx, "redis.register_prefix")
	defer span.End()

	if strings.TrimSpace(entry.PrefixHash) == "" {
		err := fmt.Errorf("prefix hash is required")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return repository.PrefixCacheRegistration{}, err
	}

	now := time.Now().UTC()
	if strings.TrimSpace(entry.Key) == "" {
		entry.Key = entry.PrefixHash
	}
	span.SetAttributes(
		attribute.String("prefix.hash", entry.PrefixHash),
		attribute.String("llm.model_id", entry.ModelID),
		attribute.Int("prefix.stable_tokens", entry.StablePrefixTokens),
	)

	storageKey := r.prefixKey(entry.PrefixHash)
	if strings.TrimSpace(entry.Namespace) != "" {
		storageKey = r.prefixKey(entry.Namespace, entry.PrefixHash)
	}
	current, found, err := r.loadPrefixEntry(ctx, storageKey)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return repository.PrefixCacheRegistration{}, err
	}
	previousState, _, err := r.loadPrefixEntry(ctx, r.prefixSessionKey(entry.SessionScope))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return repository.PrefixCacheRegistration{}, err
	}

	result := "created"
	missReason := "created"
	segmentReason := "cold_start"
	if found {
		result = "hit"
		missReason = "none"
		segmentReason = "none"
		entry.CreatedAt = current.CreatedAt
		entry.HitCount = current.HitCount + 1
		if entry.PromptLayoutVersion == "" {
			entry.PromptLayoutVersion = current.PromptLayoutVersion
		}
		if entry.ArtifactKeyVersion == "" {
			entry.ArtifactKeyVersion = current.ArtifactKeyVersion
		}
		if entry.CacheOptimizationAim == "" {
			entry.CacheOptimizationAim = current.CacheOptimizationAim
		}
	} else {
		entry.CreatedAt = now
		entry.HitCount = 1
		missReason, segmentReason = diagnoseMiss(previousState, entry)
	}
	entry.LastSeenAt = now
	appliedTTL, cacheTier := r.resolvePrefixTTL(entry.HitCount)
	entry.CacheTier = cacheTier
	entry.AppliedTTLSeconds = int64(appliedTTL.Seconds())
	entry.AdmissionDecision = "admitted"
	entry.Hot = cacheTier == "hot"

	payload, err := json.Marshal(entry)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return repository.PrefixCacheRegistration{}, fmt.Errorf("marshal prefix cache entry failed: %w", err)
	}
	if err := r.client.Set(ctx, storageKey, payload, appliedTTL).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return repository.PrefixCacheRegistration{}, err
	}
	if strings.TrimSpace(entry.SessionScope) != "" {
		if err := r.client.Set(ctx, r.prefixSessionKey(entry.SessionScope), payload, appliedTTL).Err(); err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			return repository.PrefixCacheRegistration{}, err
		}
	}
	hotScore, err := r.recordHotPrefix(ctx, entry.Namespace, entry.PrefixHash, appliedTTL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return repository.PrefixCacheRegistration{}, err
	}
	entry.HotScore = hotScore
	span.SetAttributes(
		attribute.String("prefix.namespace", entry.Namespace),
		attribute.String("prefix.lookup_result", result),
		attribute.String("prefix.miss_reason", missReason),
		attribute.String("prefix.segment_reason", segmentReason),
		attribute.String("prefix.cache_tier", cacheTier),
		attribute.Float64("prefix.hot_score", hotScore),
		attribute.Int64("prefix.hit_count", entry.HitCount),
	)
	return repository.PrefixCacheRegistration{
		Entry:         entry,
		PreviousEntry: previousState,
		Result:        result,
		MissReason:    missReason,
		SegmentReason: segmentReason,
	}, nil
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

func (r *RedisRepository) prefixKey(parts ...string) string {
	filtered := make([]string, 0, len(parts)+1)
	filtered = append(filtered, r.keyPrefix, "prefix")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, ":")
}

func (r *RedisRepository) prefixSessionKey(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return ""
	}
	return r.keyPrefix + ":prefix-session:" + scope
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

func (r *RedisRepository) loadPrefixEntry(ctx context.Context, key string) (repository.PrefixCacheEntry, bool, error) {
	if strings.TrimSpace(key) == "" {
		return repository.PrefixCacheEntry{}, false, nil
	}
	value, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return repository.PrefixCacheEntry{}, false, nil
		}
		return repository.PrefixCacheEntry{}, false, err
	}
	var entry repository.PrefixCacheEntry
	if err := json.Unmarshal([]byte(value), &entry); err != nil {
		return repository.PrefixCacheEntry{}, false, fmt.Errorf("unmarshal prefix cache entry failed: %w", err)
	}
	return entry, true, nil
}

func diagnoseMiss(previous repository.PrefixCacheEntry, current repository.PrefixCacheEntry) (string, string) {
	if previous.ModelID == "" {
		return "created", "cold_start"
	}
	if previous.ModelID != current.ModelID {
		return "model_changed", "model_changed"
	}
	if previous.PrefixHash == current.PrefixHash {
		return "ttl_expired", "ttl_expired"
	}
	if previous.SystemPrefixHash != current.SystemPrefixHash {
		return "hash_changed", "system_changed"
	}
	if previous.MemoryPrefixHash != current.MemoryPrefixHash {
		return "hash_changed", "memory_changed"
	}
	if previous.RAGPrefixHash != current.RAGPrefixHash {
		return "hash_changed", "rag_changed"
	}
	if previous.NormalizationVersion != current.NormalizationVersion {
		return "hash_changed", "normalization_changed"
	}
	return "hash_changed", "combined_changed"
}

func (r *RedisRepository) resolvePrefixTTL(hitCount int64) (time.Duration, string) {
	if hitCount >= r.hotThreshold {
		return r.hotTTL, "hot"
	}
	return r.prefixCacheTTL, "default"
}

func (r *RedisRepository) prefixHotKey(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		return r.keyPrefix + ":prefix-hot:global"
	}
	return r.keyPrefix + ":prefix-hot:" + namespace
}

func (r *RedisRepository) recordHotPrefix(ctx context.Context, namespace string, prefixHash string, ttl time.Duration) (float64, error) {
	score, err := r.client.ZIncrBy(ctx, r.prefixHotKey(namespace), 1, prefixHash).Result()
	if err != nil {
		return 0, err
	}
	if err := r.client.Expire(ctx, r.prefixHotKey(namespace), ttl).Err(); err != nil {
		return 0, err
	}
	return score, nil
}

func defaultMetrics(recorder observability.Recorder) observability.Recorder {
	if recorder == nil {
		return observability.NewNopRecorder()
	}
	return recorder
}
