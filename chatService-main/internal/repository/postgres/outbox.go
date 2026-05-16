package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
)

func (r *Repository) Save(ctx context.Context, event *domain.OutboxEvent) error {
	q := `
		INSERT INTO outbox_events (id, event_id, event_type, topic, partition_key, payload, created_at, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	db := r.getQueryExec(ctx)
	// If ID is not set, set it
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	_, err := db.Exec(ctx, q,
		event.ID, event.EventID, event.EventType, event.Topic,
		event.PartitionKey, event.Payload, event.CreatedAt, event.RetryCount,
	)
	return err
}

func (r *Repository) GetUnpublished(ctx context.Context, batchSize int) ([]*domain.OutboxEvent, error) {
	q := `
		SELECT id, event_id, event_type, topic, partition_key, payload, created_at, published_at, failed_at, retry_count
		FROM outbox_events
		WHERE published_at IS NULL AND retry_count < 5
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.getQueryExec(ctx).Query(ctx, q, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		err := rows.Scan(
			&e.ID, &e.EventID, &e.EventType, &e.Topic, &e.PartitionKey,
			&e.Payload, &e.CreatedAt, &e.PublishedAt, &e.FailedAt, &e.RetryCount,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, nil
}

func (r *Repository) MarkPublished(ctx context.Context, id uuid.UUID) error {
	q := `
		UPDATE outbox_events
		SET published_at = NOW()
		WHERE id = $1
	`
	_, err := r.getQueryExec(ctx).Exec(ctx, q, id)
	return err
}

func (r *Repository) UpdateRetryCount(ctx context.Context, id uuid.UUID, retryCount int, failedAt *time.Time) error {
	q := `
		UPDATE outbox_events
		SET retry_count = $1, failed_at = $2
		WHERE id = $3
	`
	_, err := r.getQueryExec(ctx).Exec(ctx, q, retryCount, failedAt, id)
	return err
}

