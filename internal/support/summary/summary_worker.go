package summary

import (
	"context"
	"fmt"
	"strings"
	"time"

	"context-refiner/internal/domain/core/repository"
	"context-refiner/internal/observability"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
)

type Worker struct {
	consumer     repository.SummaryJobConsumer
	pageStore    repository.PageRepository
	provider     Provider
	metrics      observability.Recorder
	group        string
	consumerName string
	batchSize    int64
	blockTimeout time.Duration
}

func NewWorker(consumer repository.SummaryJobConsumer, pageStore repository.PageRepository, provider Provider, metrics observability.Recorder, group string, consumerName string, batchSize int64, blockTimeout time.Duration) *Worker {
	if provider == nil {
		provider = NewHeuristicProvider()
	}
	return &Worker{
		consumer:     consumer,
		pageStore:    pageStore,
		provider:     provider,
		metrics:      defaultMetrics(metrics),
		group:        defaultSummaryGroup(group),
		consumerName: defaultConsumerName(consumerName),
		batchSize:    defaultBatchSize(batchSize),
		blockTimeout: defaultBlockTimeout(blockTimeout),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	ctx, span := otel.Tracer("context-refiner/infra/summary").Start(ctx, "summary.worker.run")
	defer span.End()
	span.SetAttributes(
		attribute.String("summary.group", w.group),
		attribute.String("summary.consumer", w.consumerName),
		attribute.Int64("summary.batch_size", w.batchSize),
	)

	if err := w.consumer.EnsureSummaryGroup(ctx, w.group); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
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
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			return fmt.Errorf("consume summary jobs failed: %w", err)
		}
		if err := w.processMessages(ctx, messages); err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			return err
		}
	}
}

func (w *Worker) processMessages(ctx context.Context, messages []repository.SummaryJobMessage) error {
	ctx, span := otel.Tracer("context-refiner/infra/summary").Start(ctx, "summary.worker.process_messages")
	defer span.End()
	span.SetAttributes(attribute.Int("summary.message_count", len(messages)))

	for _, message := range messages {
		if err := w.handleJob(ctx, message); err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			w.metrics.ObserveSummaryJob("failed")
			return err
		}
		if err := w.consumer.AckSummaryJob(ctx, w.group, message.ID); err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			w.metrics.ObserveSummaryJob("failed")
			return fmt.Errorf("ack summary job failed: %w", err)
		}
	}
	return nil
}

func (w *Worker) handleJob(ctx context.Context, message repository.SummaryJobMessage) error {
	ctx, span := otel.Tracer("context-refiner/infra/summary").Start(ctx, "summary.worker.handle_job")
	defer span.End()
	span.SetAttributes(
		attribute.String("summary.job_id", message.Job.JobID),
		attribute.String("summary.chunk_id", message.Job.ChunkID),
		attribute.Int("summary.page_ref_count", len(message.Job.PageRefs)),
		attribute.String("summary.provider", w.provider.Name()),
		attribute.String("summary.provider_version", w.provider.Version()),
	)

	artifact, err := w.provider.Summarize(ctx, message.Job)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return fmt.Errorf("summarize job failed: %w", err)
	}
	for _, pageRef := range message.Job.PageRefs {
		if err := w.pageStore.SaveSummary(ctx, pageRef, artifact); err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
			return fmt.Errorf("save summary result failed: %w", err)
		}
	}
	w.metrics.ObserveSummaryJob("processed")
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

func defaultMetrics(recorder observability.Recorder) observability.Recorder {
	if recorder == nil {
		return observability.NewNopRecorder()
	}
	return recorder
}
