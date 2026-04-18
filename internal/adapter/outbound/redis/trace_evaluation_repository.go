package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"context-refiner/internal/domain/core/repository"

	"github.com/redis/go-redis/v9"
)

var _ repository.TraceEvaluationRepository = (*RedisRepository)(nil)

func (r *RedisRepository) SaveTraceEvaluation(ctx context.Context, snapshot repository.TraceEvaluation) error {
	traceID := strings.TrimSpace(snapshot.TraceID)
	if traceID == "" {
		return fmt.Errorf("trace id is required")
	}
	snapshot.TraceID = traceID
	if snapshot.SavedTokens == 0 && snapshot.InputTokens > snapshot.OutputTokens {
		snapshot.SavedTokens = snapshot.InputTokens - snapshot.OutputTokens
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal trace evaluation failed: %w", err)
	}
	if err := r.client.Set(ctx, r.traceEvaluationKey(traceID), payload, r.pageTTL).Err(); err != nil {
		return fmt.Errorf("save trace evaluation failed: %w", err)
	}
	return nil
}

func (r *RedisRepository) LoadTraceEvaluation(ctx context.Context, traceID string) (repository.TraceEvaluation, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return repository.TraceEvaluation{}, fmt.Errorf("trace id is required")
	}

	value, err := r.client.Get(ctx, r.traceEvaluationKey(traceID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return repository.TraceEvaluation{}, fmt.Errorf("trace evaluation not found: %s", traceID)
		}
		return repository.TraceEvaluation{}, fmt.Errorf("load trace evaluation failed: %w", err)
	}

	var snapshot repository.TraceEvaluation
	if err := json.Unmarshal([]byte(value), &snapshot); err != nil {
		return repository.TraceEvaluation{}, fmt.Errorf("decode trace evaluation failed: %w", err)
	}
	return snapshot, nil
}
