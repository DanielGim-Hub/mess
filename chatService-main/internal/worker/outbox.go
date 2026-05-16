package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/messenger/chat-service/internal/domain"
	"github.com/rs/zerolog/log"
	kafka "github.com/segmentio/kafka-go"
)

type TransactionManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type OutboxWorker struct {
	repo          domain.OutboxRepository
	tm            TransactionManager
	kafkaWriter   *kafka.Writer
	checkInterval time.Duration
	batchSize     int
}

func NewOutboxWorker(repo domain.OutboxRepository, brokers []string, topic string) *OutboxWorker {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &OutboxWorker{
		repo:          repo,
		kafkaWriter:   w,
		checkInterval: 2 * time.Second,
		batchSize:     50,
	}
}

func NewOutboxWorkerWithTx(repo domain.OutboxRepository, tm TransactionManager, brokers []string, topic string) *OutboxWorker {
	w := NewOutboxWorker(repo, brokers, topic)
	w.tm = tm
	return w
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Outbox worker shutting down...")
			if err := w.kafkaWriter.Close(); err != nil {
				log.Error().Err(err).Msg("Failed to close kafka writer")
			}
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	events, err := w.repo.GetUnpublished(ctx, w.batchSize)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch unpublished events")
		return
	}

	if len(events) == 0 {
		return
	}

	for _, event := range events {
		if err := w.processEvent(ctx, event); err != nil {
			log.Error().Err(err).Str("event_id", event.EventID.String()).Msg("Failed to process outbox event")
			w.handleRetry(ctx, event)
		}
	}
}

func (w *OutboxWorker) processEvent(ctx context.Context, event *domain.OutboxEvent) error {
	envelope := domain.EventEnvelope{
		EventID:        event.EventID,
		EventType:      event.EventType,
		OccurredAt:     event.CreatedAt,
		SourceService:  "chat-service",
		PayloadVersion: 1,
		Payload:        json.RawMessage(event.Payload),
	}

	value, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Key:   []byte(event.PartitionKey),
		Value: value,
		Topic: event.Topic,
	}

	// If transaction manager available, wrap in TX
	if w.tm != nil {
		return w.tm.RunInTx(ctx, func(txCtx context.Context) error {
			if err := w.kafkaWriter.WriteMessages(txCtx, msg); err != nil {
				return err
			}
			return w.repo.MarkPublished(txCtx, event.ID)
		})
	}

	// Non-transactional fallback
	if err := w.kafkaWriter.WriteMessages(ctx, msg); err != nil {
		return err
	}
	return w.repo.MarkPublished(ctx, event.ID)
}

func (w *OutboxWorker) handleRetry(ctx context.Context, event *domain.OutboxEvent) {
	event.RetryCount++
	var failedAt *time.Time
	if event.RetryCount > 5 {
		now := time.Now().UTC()
		failedAt = &now
		log.Error().
			Str("event_id", event.EventID.String()).
			Int("retry_count", event.RetryCount).
			Msg("Event publishing failed after max retries")
	}

	if err := w.repo.UpdateRetryCount(ctx, event.ID, event.RetryCount, failedAt); err != nil {
		log.Error().Err(err).Msg("Failed to update retry count")
	}
}
