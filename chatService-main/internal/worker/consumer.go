package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	kafka "github.com/segmentio/kafka-go"
)

type MessageConsumer struct {
	chatRepo domain.ChatRepository
	reader   *kafka.Reader
	redis    *redis.Client
}

func NewMessageConsumer(brokers []string, topic string, groupID string, repo domain.ChatRepository, redisClient *redis.Client) *MessageConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &MessageConsumer{
		chatRepo: repo,
		reader:   r,
		redis:    redisClient,
	}
}

func (c *MessageConsumer) Start(ctx context.Context) {
	defer func() {
		if err := c.reader.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close kafka reader")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Message consumer shutting down...")
			return
		default:
		}

		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error().Err(err).Msg("Consumer error")
			time.Sleep(time.Second)
			continue
		}

		c.processMessage(ctx, m)
	}
}

func (c *MessageConsumer) isProcessed(ctx context.Context, eventID uuid.UUID) bool {
	if c.redis == nil {
		return false
	}
	key := "chat:consumer:dedup:" + eventID.String()
	exists, err := c.redis.Exists(ctx, key).Result()
	if err != nil {
		log.Warn().Err(err).Str("event_id", eventID.String()).Msg("Redis dedup check failed")
		return false
	}
	return exists > 0
}

func (c *MessageConsumer) markProcessed(ctx context.Context, eventID uuid.UUID) {
	if c.redis == nil {
		return
	}
	key := "chat:consumer:dedup:" + eventID.String()
	if err := c.redis.Set(ctx, key, "1", 24*time.Hour).Err(); err != nil {
		log.Warn().Err(err).Str("event_id", eventID.String()).Msg("Redis dedup mark failed")
	}
}

func (c *MessageConsumer) processMessage(ctx context.Context, m kafka.Message) {
	var envelope domain.EventEnvelope
	if err := json.Unmarshal(m.Value, &envelope); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal event envelope")
		return
	}

	// Deduplication check
	if c.isProcessed(ctx, envelope.EventID) {
		log.Debug().Str("event_id", envelope.EventID.String()).Msg("Event already processed, skipping")
		return
	}

	if envelope.EventType != "message.created" {
		return
	}

	type MessageCreatedPayload struct {
		ChatID    uuid.UUID `json:"chat_id"`
		CreatedAt time.Time `json:"created_at"`
	}

	var payload MessageCreatedPayload
	payloadBytes, _ := json.Marshal(envelope.Payload)
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Error().Err(err).Msg("Failed to parse message.created payload")
		return
	}

	if err := c.chatRepo.UpdateLastMessageAt(ctx, payload.ChatID, payload.CreatedAt); err != nil {
		log.Error().Err(err).Msg("Failed to update last_message_at")
		return
	}

	c.markProcessed(ctx, envelope.EventID)
	log.Debug().Str("event_id", envelope.EventID.String()).Msg("Event processed successfully")
}
