package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
)

type Service struct {
	chatRepo   domain.ChatRepository
	memberRepo domain.MemberRepository
	metaRepo   domain.MetadataRepository
	outboxRepo domain.OutboxRepository
	tm         domain.TransactionManager
}

func NewService(
	chatRepo domain.ChatRepository,
	memberRepo domain.MemberRepository,
	metaRepo   domain.MetadataRepository,
	outboxRepo domain.OutboxRepository,
	tm domain.TransactionManager,
) *Service {
	return &Service{
		chatRepo:   chatRepo,
		memberRepo: memberRepo,
		metaRepo:   metaRepo,
		outboxRepo: outboxRepo,
		tm:         tm,
	}
}

// Internal Service Methods
func (s *Service) IsMember(ctx context.Context, chatID, userID uuid.UUID) (*domain.ChatMember, error) {
return s.memberRepo.Get(ctx, chatID, userID)
}

// IsChatMember checks if user is a member of the chat (for authorization)
func (s *Service) IsChatMember(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.memberRepo.Get(ctx, chatID, userID)
	return err
}
func (s *Service) GetChatSnapshot(ctx context.Context, chatID uuid.UUID) (*domain.Chat, []*domain.ChatMember, error) {
chat, err := s.chatRepo.GetByID(ctx, chatID)
if err != nil {
return nil, nil, err
}
members, err := s.memberRepo.ListByChatID(ctx, chatID)
if err != nil {
return nil, nil, err
}
return chat, members, nil
}
// Metadata Service Methods
func (s *Service) UpsertMetadata(ctx context.Context, chatID, userID uuid.UUID, key string, value json.RawMessage) error {
// Check permissions? Usually owner/admin.
member, err := s.memberRepo.Get(ctx, chatID, userID)
if err != nil {
return err
}
if member.Role != domain.RoleOwner && member.Role != domain.RoleAdmin {
return domain.ErrPermissionDenied
}
meta := &domain.ChatMetadata{
ChatID: chatID,
Key:    key,
Value:  value,
}
return s.tm.RunInTx(ctx, func(ctx context.Context) error {
if err := s.metaRepo.UpsertMetadata(ctx, meta); err != nil {
return err
}
changes := []string{"metadata"}
payload := domain.ChatUpdatedPayload{
ChatID:    chatID,
UpdatedBy: userID,
Changes:   changes,
}
return s.publishEvent(ctx, chatID, domain.EventTypeChatUpdated, payload)
})
}
func (s *Service) GetMetadata(ctx context.Context, chatID uuid.UUID, key string) (*domain.ChatMetadata, error) {
return s.metaRepo.GetMetadata(ctx, chatID, key)
}
func (s *Service) DeleteMetadata(ctx context.Context, chatID, userID uuid.UUID, key string) error {
member, err := s.memberRepo.Get(ctx, chatID, userID)
if err != nil {
return err
}
if member.Role != domain.RoleOwner && member.Role != domain.RoleAdmin {
return domain.ErrPermissionDenied
}
return s.tm.RunInTx(ctx, func(ctx context.Context) error {
if err := s.metaRepo.DeleteMetadata(ctx, chatID, key); err != nil {
return err
}
payload := domain.ChatUpdatedPayload{
ChatID:    chatID,
UpdatedBy: userID,
Changes:   []string{"metadata"},
}
return s.publishEvent(ctx, chatID, domain.EventTypeChatUpdated, payload)
})
}
func (s *Service) ListMetadata(ctx context.Context, chatID uuid.UUID) ([]*domain.ChatMetadata, error) {
return s.metaRepo.ListMetadata(ctx, chatID)
}

