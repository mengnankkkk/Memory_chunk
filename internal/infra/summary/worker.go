package summary

import (
	"context"
	"fmt"
	"strings"
	"time"

	"context-refiner/internal/core/repository"
)

type Worker struct {
	consumer     repository.SummaryJobConsumer
	pageStore    repository.PageRepository
	group        string
	consumerName string
	batchSize    int64
	blockTimeout time.Duration
}

func NewWorker(consumer repository.SummaryJobConsumer, pageStore repository.PageRepository, group string, consumerName string, batchSize int64, blockTimeout time.Duration) *Worker {
	return &Worker{
		consumer:     consumer,
		pageStore:    pageStore,
		group:        defaultSummaryGroup(group),
		consumerName: defaultConsumerName(consumerName),
		batchSize:    defaultBatchSize(batchSize),
		blockTimeout: defaultBlockTimeout(blockTimeout),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.consumer.EnsureSummaryGroup(ctx, w.group); err != nil {
		return fmt.Errorf("ensure summary group failed: %w", err)
	}

	for {
		if ctx.Err() != nil {
			return nil
		}
		messages, err := w.consumer.ConsumeSummaryJobs(ctx, w.group, w.consumerName, w.batchSize, w.blockTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("consume summary jobs failed: %w", err)
		}
		if err := w.processMessages(ctx, messages); err != nil {
			return err
		}
	}
}

func (w *Worker) processMessages(ctx context.Context, messages []repository.SummaryJobMessage) error {
	for _, message := range messages {
		if err := w.handleJob(ctx, message); err != nil {
			return err
		}
		if err := w.consumer.AckSummaryJob(ctx, w.group, message.ID); err != nil {
			return fmt.Errorf("ack summary job failed: %w", err)
		}
	}
	return nil
}

func (w *Worker) handleJob(ctx context.Context, message repository.SummaryJobMessage) error {
	result := repository.SummaryResult{
		JobID:     message.Job.JobID,
		Content:   summarizeJob(message.Job),
		CreatedAt: time.Now().UTC(),
	}
	for _, pageRef := range message.Job.PageRefs {
		if err := w.pageStore.SaveSummary(ctx, pageRef, result); err != nil {
			return fmt.Errorf("save summary result failed: %w", err)
		}
	}
	return nil
}

func defaultSummaryGroup(group string) string {
	if strings.TrimSpace(group) == "" {
		return "context-refiner-summary"
	}
	return group
}

func defaultConsumerName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "worker-1"
	}
	return name
}

func defaultBatchSize(batchSize int64) int64 {
	if batchSize <= 0 {
		return 8
	}
	return batchSize
}

func defaultBlockTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return 2 * time.Second
	}
	return timeout
}
