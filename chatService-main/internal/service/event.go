package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
)

func (s *Service) publishEvent(ctx context.Context, chatID uuid.UUID, eventType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	event := &domain.OutboxEvent{
		EventID:      uuid.New(),
		EventType:    eventType,
		Topic:        domain.TopicChatEvents,
		PartitionKey: chatID.String(),
		Payload:      payloadBytes,
		CreatedAt:    time.Now().UTC(),
		RetryCount:   0,
	}

	return s.outboxRepo.Save(ctx, event)
}

