package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ChatRepository interface {
	Create(ctx context.Context, chat *Chat) error
	GetByID(ctx context.Context, id uuid.UUID) (*Chat, error)
	Update(ctx context.Context, chat *Chat) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUserID(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*Chat, error)
	GetDirectChat(ctx context.Context, userA, userB uuid.UUID) (*Chat, error)
	SaveDirectChatIndex(ctx context.Context, userA, userB, chatID uuid.UUID) error
	UpdateLastMessageAt(ctx context.Context, chatID uuid.UUID, timestamp time.Time) error
}

type MetadataRepository interface {
	UpsertMetadata(ctx context.Context, meta *ChatMetadata) error
	GetMetadata(ctx context.Context, chatID uuid.UUID, key string) (*ChatMetadata, error)
	DeleteMetadata(ctx context.Context, chatID uuid.UUID, key string) error
	ListMetadata(ctx context.Context, chatID uuid.UUID) ([]*ChatMetadata, error)
}

type MemberRepository interface {
	Add(ctx context.Context, member *ChatMember) error
	Remove(ctx context.Context, chatID, userID uuid.UUID) error
	UpdateRole(ctx context.Context, chatID, userID uuid.UUID, role MemberRole) error
	Get(ctx context.Context, chatID, userID uuid.UUID) (*ChatMember, error)
	ListByChatID(ctx context.Context, chatID uuid.UUID) ([]*ChatMember, error)
	Count(ctx context.Context, chatID uuid.UUID) (int64, error)
}

type OutboxRepository interface {
	Save(ctx context.Context, event *OutboxEvent) error
	GetUnpublished(ctx context.Context, batchSize int) ([]*OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
	UpdateRetryCount(ctx context.Context, id uuid.UUID, retryCount int, failedAt *time.Time) error
}

type TransactionManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

