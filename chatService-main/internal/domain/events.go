package domain

import (
	"time"

	"github.com/google/uuid"
)

// Event Envelope
type EventEnvelope struct {
	EventID        uuid.UUID   `json:"event_id"`
	EventType      string      `json:"event_type"` // e.g., "chat.created"
	OccurredAt     time.Time   `json:"occurred_at"`
	SourceService  string      `json:"source_service"` // "chat-service"
	PayloadVersion int         `json:"payload_version"`
	Payload        interface{} `json:"payload"`
}

// Event Types (Topics are usually inferred or mapped)
const (
	EventTypeChatCreated = "chat.created"
	EventTypeChatUpdated = "chat.updated"
	EventTypeChatDeleted = "chat.deleted"
	TopicChatEvents      = "chat.events"
)

// Payloads

type ChatCreatedPayload struct {
	ChatID    uuid.UUID   `json:"chat_id"`
	Type      ChatType    `json:"type"`
	Title     *string     `json:"title"`
	CreatedBy uuid.UUID   `json:"created_by"`
	MemberIDs []uuid.UUID `json:"member_ids"`
}

type ChatUpdatedPayload struct {
	ChatID         uuid.UUID   `json:"chat_id"`
	Changes        []string    `json:"changes"` // "title", "avatar_url", "members", "roles"
	UpdatedBy      uuid.UUID   `json:"updated_by"`
	MembersAdded   []uuid.UUID `json:"members_added"`
	MembersRemoved []uuid.UUID `json:"members_removed"`
}

type ChatDeletedPayload struct {
	ChatID    uuid.UUID   `json:"chat_id"`
	DeletedBy uuid.UUID   `json:"deleted_by"`
	MemberIDs []uuid.UUID `json:"member_ids"`
}

