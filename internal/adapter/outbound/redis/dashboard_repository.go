package redisstore

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"context-refiner/internal/domain/core/repository"

	"github.com/redis/go-redis/v9"
)

var _ repository.DashboardRepository = (*RedisRepository)(nil)

const (
	defaultDashboardLimit = 8
	maxDashboardLimit     = 50
)

func (r *RedisRepository) LoadDashboardSnapshot(ctx context.Context, query repository.DashboardQuery) (repository.DashboardSnapshot, error) {
	limit := normalizeDashboardLimit(query.Limit)

	pageArtifacts, pageCount, err := r.loadDashboardPageArtifacts(ctx, limit)
	if err != nil {
		return repository.DashboardSnapshot{}, err
	}

	summaryArtifacts, summaryCount, err := r.loadDashboardSummaryArtifacts(ctx, limit)
	if err != nil {
		return repository.DashboardSnapshot{}, err
	}

	recentPrefixes, prefixCount, err := r.loadDashboardRecentPrefixes(ctx, limit)
	if err != nil {
		return repository.DashboardSnapshot{}, err
	}

	hotPrefixes, err := r.loadDashboardHotPrefixes(ctx, limit)
	if err != nil {
		return repository.DashboardSnapshot{}, err
	}

	queue, err := r.loadDashboardQueueSnapshot(ctx, query.SummaryConsumerGroup)
	if err != nil {
		return repository.DashboardSnapshot{}, err
	}

	return repository.DashboardSnapshot{
		GeneratedAt:          time.Now().UTC(),
		PageArtifactCount:    pageCount,
		SummaryArtifactCount: summaryCount,
		PrefixEntryCount:     prefixCount,
		PageArtifacts:        pageArtifacts,
		SummaryArtifacts:     summaryArtifacts,
		RecentPrefixes:       recentPrefixes,
		HotPrefixes:          hotPrefixes,
		SummaryQueue:         queue,
	}, nil
}

func (r *RedisRepository) LoadPageDetail(ctx context.Context, key string) (repository.DashboardPageDetail, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return repository.DashboardPageDetail{}, fmt.Errorf("page key is required")
	}

	detail := repository.DashboardPageDetail{Key: key}

	rawContent, rawErr := r.LoadPage(ctx, key)
	if rawErr == nil {
		detail.RawContent = rawContent
		detail.RawTTLSeconds = ttlSeconds(r.client.TTL(ctx, r.prefixed(key)).Val())
	}

	resolved, err := r.LoadResolvedPage(ctx, key)
	if err != nil {
		if rawErr != nil {
			return repository.DashboardPageDetail{}, err
		}
		return detail, nil
	}

	detail.ResolvedContent = resolved.Content
	detail.ResolvedIsSummary = resolved.IsSummary
	detail.SummaryArtifact = resolved.SummaryArtifact
	if resolved.SummaryArtifact != nil {
		detail.SummaryTTLSeconds = ttlSeconds(r.client.TTL(ctx, r.summaryKey(key)).Val())
	}

	return detail, nil
}

func (r *RedisRepository) loadDashboardPageArtifacts(ctx context.Context, limit int) ([]repository.DashboardPageArtifact, int, error) {
	keys, err := r.scanKeys(ctx, r.keyPrefix+":*")
	if err != nil {
		return nil, 0, err
	}

	pageKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if !r.isPageStorageKey(key) {
			continue
		}
		pageKeys = append(pageKeys, key)
	}
	sort.Strings(pageKeys)

	records := make([]repository.DashboardPageArtifact, 0, min(limit, len(pageKeys)))
	for _, storageKey := range pageKeys {
		if len(records) >= limit {
			break
		}
		pageKey := strings.TrimPrefix(storageKey, r.keyPrefix+":")
		content, err := r.client.Get(ctx, storageKey).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			return nil, 0, err
		}

		record := repository.DashboardPageArtifact{
			Key:           pageKey,
			ContentLength: len(content),
			TTLSeconds:    ttlSeconds(r.client.TTL(ctx, storageKey).Val()),
			Preview:       previewText(content, 220),
		}

		if artifact, err := r.loadSummaryArtifact(ctx, pageKey); err == nil {
			record.SummaryArtifact = &artifact
		}

		records = append(records, record)
	}

	return records, len(pageKeys), nil
}

func (r *RedisRepository) loadDashboardSummaryArtifacts(ctx context.Context, limit int) ([]repository.DashboardSummaryRecord, int, error) {
	keys, err := r.scanKeys(ctx, r.summaryKey("*"))
	if err != nil {
		return nil, 0, err
	}

	records := make([]repository.DashboardSummaryRecord, 0, min(limit, len(keys)))
	all := make([]repository.DashboardSummaryRecord, 0, len(keys))

	for _, storageKey := range keys {
		pageKey := strings.TrimPrefix(storageKey, r.summaryKey(""))
		artifact, err := r.loadSummaryArtifact(ctx, pageKey)
		if err != nil {
			continue
		}
		all = append(all, repository.DashboardSummaryRecord{
			PageKey:         pageKey,
			ArtifactID:      artifact.ArtifactID,
			JobID:           artifact.JobID,
			Provider:        artifact.Provider,
			ProviderVersion: artifact.ProviderVersion,
			ContentHash:     artifact.ContentHash,
			FragmentTypes:   append([]string(nil), artifact.FragmentTypes...),
			CreatedAt:       artifact.CreatedAt,
			ExpiresAt:       artifact.ExpiresAt,
			TTLSeconds:      ttlSeconds(r.client.TTL(ctx, storageKey).Val()),
			SummaryPreview:  previewText(artifact.SummaryText, 220),
		})
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	if len(all) > limit {
		records = append(records, all[:limit]...)
	} else {
		records = append(records, all...)
	}

	return records, len(all), nil
}

func (r *RedisRepository) loadDashboardRecentPrefixes(ctx context.Context, limit int) ([]repository.DashboardPrefixRecord, int, error) {
	keys, err := r.scanKeys(ctx, r.prefixKey("*"))
	if err != nil {
		return nil, 0, err
	}

	all := make([]repository.DashboardPrefixRecord, 0, len(keys))
	for _, key := range keys {
		entry, found, err := r.loadPrefixEntry(ctx, key)
		if err != nil {
			return nil, 0, err
		}
		if !found {
			continue
		}
		all = append(all, prefixRecordFromEntry(key, entry))
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].LastSeenAt.Equal(all[j].LastSeenAt) {
			return all[i].CreatedAt.After(all[j].CreatedAt)
		}
		return all[i].LastSeenAt.After(all[j].LastSeenAt)
	})

	if len(all) > limit {
		all = all[:limit]
	}
	return all, len(keys), nil
}

func (r *RedisRepository) loadDashboardHotPrefixes(ctx context.Context, limit int) ([]repository.DashboardPrefixRecord, error) {
	keys, err := r.scanKeys(ctx, r.keyPrefix+":prefix-hot:*")
	if err != nil {
		return nil, err
	}

	records := make([]repository.DashboardPrefixRecord, 0, limit)
	seen := make(map[string]struct{}, limit)

	for _, key := range keys {
		namespace := strings.TrimPrefix(key, r.keyPrefix+":prefix-hot:")
		items, err := r.client.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		for _, item := range items {
			prefixHash, ok := item.Member.(string)
			if !ok || prefixHash == "" {
				continue
			}
			entryKey := r.prefixKey(namespace, prefixHash)
			entry, found, err := r.loadPrefixEntry(ctx, entryKey)
			if err != nil {
				return nil, err
			}
			if !found && namespace == "global" {
				entryKey = r.prefixKey(prefixHash)
				entry, found, err = r.loadPrefixEntry(ctx, entryKey)
				if err != nil {
					return nil, err
				}
			}

			record := repository.DashboardPrefixRecord{
				StorageKey: entryKey,
				Namespace:  namespace,
				PrefixHash: prefixHash,
				HotScore:   item.Score,
				Hot:        true,
				CacheTier:  "hot",
			}
			if found {
				record = prefixRecordFromEntry(entryKey, entry)
				record.HotScore = item.Score
				record.Hot = true
			}

			dedupeKey := record.Namespace + "|" + record.PrefixHash
			if _, ok := seen[dedupeKey]; ok {
				continue
			}
			seen[dedupeKey] = struct{}{}
			records = append(records, record)
			if len(records) >= limit {
				break
			}
		}
		if len(records) >= limit {
			break
		}
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].HotScore == records[j].HotScore {
			return records[i].LastSeenAt.After(records[j].LastSeenAt)
		}
		return records[i].HotScore > records[j].HotScore
	})
	return records, nil
}

func (r *RedisRepository) loadDashboardQueueSnapshot(ctx context.Context, consumerGroup string) (repository.DashboardQueueSnapshot, error) {
	queue := repository.DashboardQueueSnapshot{
		Stream:        r.summaryStream,
		ConsumerGroup: strings.TrimSpace(consumerGroup),
	}

	info, err := r.client.XInfoStream(ctx, r.summaryStream).Result()
	if err == nil {
		queue.StreamLength = info.Length
		queue.LastGeneratedID = info.LastGeneratedID
	} else if !errors.Is(err, redis.Nil) && !strings.Contains(err.Error(), "ERR no such key") {
		return repository.DashboardQueueSnapshot{}, err
	}

	if queue.ConsumerGroup == "" {
		return queue, nil
	}

	pending, err := r.client.XPending(ctx, r.summaryStream, queue.ConsumerGroup).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "NOGROUP") {
			return queue, nil
		}
		return repository.DashboardQueueSnapshot{}, err
	}

	queue.PendingCount = pending.Count
	queue.ConsumerCount = len(pending.Consumers)
	queue.OldestPendingID = pending.Lower
	queue.NewestPendingID = pending.Higher
	queue.Consumers = pending.Consumers
	return queue, nil
}

func (r *RedisRepository) scanKeys(ctx context.Context, match string) ([]string, error) {
	keys := make([]string, 0)
	var cursor uint64

	for {
		batch, nextCursor, err := r.client.Scan(ctx, cursor, match, 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	sort.Strings(keys)
	return keys, nil
}

func (r *RedisRepository) isPageStorageKey(key string) bool {
	switch {
	case !strings.HasPrefix(key, r.keyPrefix+":"):
		return false
	case strings.HasPrefix(key, r.summaryKey("")):
		return false
	case strings.HasPrefix(key, r.keyPrefix+":prefix:"):
		return false
	case strings.HasPrefix(key, r.keyPrefix+":prefix-session:"):
		return false
	case strings.HasPrefix(key, r.keyPrefix+":prefix-hot:"):
		return false
	default:
		return true
	}
}

func prefixRecordFromEntry(storageKey string, entry repository.PrefixCacheEntry) repository.DashboardPrefixRecord {
	return repository.DashboardPrefixRecord{
		StorageKey:           storageKey,
		Namespace:            entry.Namespace,
		PrefixHash:           entry.PrefixHash,
		ModelID:              entry.ModelID,
		StablePrefixTokens:   entry.StablePrefixTokens,
		CacheTier:            entry.CacheTier,
		Hot:                  entry.Hot,
		HotScore:             entry.HotScore,
		HitCount:             entry.HitCount,
		AdmissionDecision:    entry.AdmissionDecision,
		NormalizationVersion: entry.NormalizationVersion,
		CreatedAt:            entry.CreatedAt,
		LastSeenAt:           entry.LastSeenAt,
	}
}

func previewText(value string, limit int) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func ttlSeconds(ttl time.Duration) int64 {
	if ttl <= 0 {
		return 0
	}
	return int64(ttl.Seconds())
}

func normalizeDashboardLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultDashboardLimit
	case limit > maxDashboardLimit:
		return maxDashboardLimit
	default:
		return limit
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
