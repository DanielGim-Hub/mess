package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Roles
type MemberRole string

const (
	RoleOwner  MemberRole = "owner"
	RoleAdmin  MemberRole = "admin"
	RoleMember MemberRole = "member"
)

// Chat Types
type ChatType string

const (
	ChatTypeDirect ChatType = "direct"
	ChatTypeGroup  ChatType = "group"
)

// Chat represents a chat room/conversation
type Chat struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	Type          ChatType   `json:"type" db:"type"`
	Title         *string    `json:"title,omitempty" db:"title"`
	AvatarURL     *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	CreatedBy     uuid.UUID  `json:"created_by" db:"created_by"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty" db:"last_message_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	MembersCount  int64      `json:"members_count" db:"members_count"`
}

// ChatMember represents a user in a chat
type ChatMember struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	ChatID    uuid.UUID  `json:"chat_id" db:"chat_id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Role      MemberRole `json:"role" db:"role"`
	InvitedBy *uuid.UUID `json:"invited_by,omitempty" db:"invited_by"`
	JoinedAt  time.Time  `json:"joined_at" db:"joined_at"`
	LeftAt    *time.Time `json:"left_at,omitempty" db:"left_at"`
}

// OutboxEvent represents an event to be published to Kafka
type OutboxEvent struct {
	ID           uuid.UUID  `db:"id"`
	EventID      uuid.UUID  `db:"event_id"`
	EventType    string     `db:"event_type"`
	Topic        string     `db:"topic"`
	PartitionKey string     `db:"partition_key"`
	Payload      []byte     `db:"payload"` // JSONB
	CreatedAt    time.Time  `db:"created_at"`
	PublishedAt  *time.Time `db:"published_at"`
	FailedAt     *time.Time `db:"failed_at"`
	RetryCount   int        `db:"retry_count"`
}

// ChatMetadata represents strict key-value metadata for a chat
type ChatMetadata struct {
	ChatID    uuid.UUID       `json:"chat_id" db:"chat_id"`
	Key       string          `json:"key" db:"key"`
	Value     json.RawMessage `json:"value" db:"value"` // JSONB
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

var (
	ErrChatNotFound              = errors.New("chat not found")
	ErrMemberNotFound            = errors.New("member not found")
	ErrInvalidChatType           = errors.New("invalid chat type")
	ErrDirectChatExists          = errors.New("direct chat already exists")
	ErrMaxMembersReached         = errors.New("max members reached")
	ErrPermissionDenied          = errors.New("permission denied")
	ErrMetadataNotFound          = errors.New("metadata not found")
	ErrAlreadyMember             = errors.New("already a member")
	ErrCannotUpdateDirectChat    = errors.New("cannot modify direct chat")
	ErrCannotTransferOwnerToSelf = errors.New("cannot transfer owner to self")
	ErrOwnerTransferTargetInvalid = errors.New("owner transfer target invalid")
	ErrCannotRemoveOwner         = errors.New("cannot remove owner")
	ErrOwnerMustTransferBeforeLeave = errors.New("owner cannot leave")
	ErrValidationError           = errors.New("validation error")
)
