package http

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
)

// ============================================================
// Response DTOs - соответствуют api/chat-service.yaml
// ============================================================

// Pagination
type CursorPaginationDTO struct {
	NextCursor *string `json:"next_cursor"`
	HasNext    bool    `json:"has_next"`
}

// Chat Related
type ChatDTO struct {
	ID            uuid.UUID  `json:"id"`
	Type          string     `json:"type"`
	Title         *string    `json:"title,omitempty"`
	AvatarURL     *string    `json:"avatar_url,omitempty"`
	CreatedBy     uuid.UUID  `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	MembersCount  int64      `json:"members_count"`
}

type ChatListResponse struct {
	Items      []*ChatDTO          `json:"items"`
	Pagination CursorPaginationDTO `json:"pagination"`
}

// Member Related
type ChatMemberDTO struct {
	UserID    uuid.UUID  `json:"user_id"`
	Role      string     `json:"role"`
	JoinedAt  time.Time  `json:"joined_at"`
	InvitedBy *uuid.UUID `json:"invited_by,omitempty"`
}

type MemberListResponse struct {
	Items      []*ChatMemberDTO    `json:"items"`
	Pagination CursorPaginationDTO `json:"pagination"`
}

// Internal Endpoints
type InternalChatMembershipDTO struct {
	ChatID    uuid.UUID `json:"chat_id"`
	UserID    uuid.UUID `json:"user_id"`
	IsMember  bool      `json:"is_member"`
	Role      *string   `json:"role,omitempty"`
	JoinedAt  *time.Time `json:"joined_at,omitempty"`
	ChatType  string     `json:"chat_type"`
	CheckedAt time.Time  `json:"checked_at"`
}

type InternalChatSnapshotDTO struct {
	Chat       *ChatDTO           `json:"chat"`
	Members    []*ChatMemberDTO   `json:"members"`
	SnapshotAt time.Time           `json:"snapshot_at"`
}

type InternalUserChatRefDTO struct {
	ChatID        uuid.UUID  `json:"chat_id"`
	Type          string     `json:"type"`
	Role          string     `json:"role"`
	JoinedAt      time.Time  `json:"joined_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	MembersCount  int64      `json:"members_count"`
}

type InternalUserChatRefListResponse struct {
	Items      []*InternalUserChatRefDTO `json:"items"`
	Pagination CursorPaginationDTO        `json:"pagination"`
}

// ============================================================
// Converters - Domain to DTO
// ============================================================

func DomainChatToDTO(c *domain.Chat) *ChatDTO {
	if c == nil {
		return nil
	}
	return &ChatDTO{
		ID:            c.ID,
		Type:          string(c.Type),
		Title:         c.Title,
		AvatarURL:     c.AvatarURL,
		CreatedBy:     c.CreatedBy,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
		LastMessageAt: c.LastMessageAt,
		MembersCount:  c.MembersCount,
	}
}

func DomainChatsToDTO(chats []*domain.Chat) []*ChatDTO {
	if chats == nil {
		return []*ChatDTO{}
	}
	dtos := make([]*ChatDTO, len(chats))
	for i, c := range chats {
		dtos[i] = DomainChatToDTO(c)
	}
	return dtos
}

func DomainMemberToDTO(m *domain.ChatMember) *ChatMemberDTO {
	if m == nil {
		return nil
	}
	return &ChatMemberDTO{
		UserID:    m.UserID,
		Role:      string(m.Role),
		JoinedAt:  m.JoinedAt,
		InvitedBy: m.InvitedBy,
	}
}

func DomainMembersToDTO(members []*domain.ChatMember) []*ChatMemberDTO {
	if members == nil {
		return []*ChatMemberDTO{}
	}
	dtos := make([]*ChatMemberDTO, len(members))
	for i, m := range members {
		dtos[i] = DomainMemberToDTO(m)
	}
	return dtos
}

// ============================================================
// Pagination Helpers
// ============================================================

// CalculatePaginationCursor - создаёт opaque cursor для next_cursor
func CalculatePaginationCursor(items interface{}, hasMore bool) *string {
	if !hasMore {
		return nil
	}
	
	// Encoding для next cursor - можно использовать base64 or UUID
	// Упрощённый вариант - используем time-based cursor
	cursorStr := strconv.FormatInt(time.Now().Unix(), 10)
	return &cursorStr
}

// NewPaginationDTO создаёт DTO пагинации
func NewPaginationDTO(nextCursor *string, hasNext bool) CursorPaginationDTO {
	return CursorPaginationDTO{
		NextCursor: nextCursor,
		HasNext:    hasNext,
	}
}

// ============================================================
// Response Builder Helpers
// ============================================================

func NewChatListResponse(chats []*domain.Chat, nextCursor *string, hasNext bool) *ChatListResponse {
	return &ChatListResponse{
		Items:      DomainChatsToDTO(chats),
		Pagination: NewPaginationDTO(nextCursor, hasNext),
	}
}

func NewMemberListResponse(members []*domain.ChatMember, nextCursor *string, hasNext bool) *MemberListResponse {
	return &MemberListResponse{
		Items:      DomainMembersToDTO(members),
		Pagination: NewPaginationDTO(nextCursor, hasNext),
	}
}

func NewInternalChatSnapshotDTO(chat *domain.Chat, members []*domain.ChatMember) *InternalChatSnapshotDTO {
	return &InternalChatSnapshotDTO{
		Chat:       DomainChatToDTO(chat),
		Members:    DomainMembersToDTO(members),
		SnapshotAt: time.Now().UTC(),
	}
}

func DomainUserChatRefToDTO(c *domain.Chat, role string, joinedAt time.Time) *InternalUserChatRefDTO {
	return &InternalUserChatRefDTO{
		ChatID:        c.ID,
		Type:          string(c.Type),
		Role:          role,
		JoinedAt:      joinedAt,
		UpdatedAt:     c.UpdatedAt,
		LastMessageAt: c.LastMessageAt,
		MembersCount:  c.MembersCount,
	}
}


