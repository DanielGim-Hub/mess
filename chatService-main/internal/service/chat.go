package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
)

func (s *Service) CreateDirectChat(ctx context.Context, creatorID, targetID uuid.UUID) (*domain.Chat, error) {
	// Optimization: check existing first
	existing, err := s.chatRepo.GetDirectChat(ctx, creatorID, targetID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrChatNotFound) {
		return nil, err
	}

	chat := &domain.Chat{
		ID:        uuid.New(),
		Type:      domain.ChatTypeDirect,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		CreatedBy: creatorID,
	}

	err = s.tm.RunInTx(ctx, func(ctx context.Context) error {
		// Double check inside TX? Optional. DB constraint protects.
		if err := s.chatRepo.Create(ctx, chat); err != nil {
			return err
		}

		// Add members
		members := []*domain.ChatMember{
			{
				ID:       uuid.New(),
				ChatID:   chat.ID,
				UserID:   creatorID,
				Role:     domain.RoleMember,
				JoinedAt: chat.CreatedAt,
			},
			{
				ID:       uuid.New(),
				ChatID:   chat.ID,
				UserID:   targetID,
				Role:     domain.RoleMember,
				JoinedAt: chat.CreatedAt,
			},
		}

		for _, m := range members {
			if err := s.memberRepo.Add(ctx, m); err != nil {
				return err
			}
		}

		// Index
		if err := s.chatRepo.SaveDirectChatIndex(ctx, creatorID, targetID, chat.ID); err != nil {
			return err
		}

		// Event
		payload := domain.ChatCreatedPayload{
			ChatID:    chat.ID,
			Type:      chat.Type,
			CreatedBy: creatorID,
			MemberIDs: []uuid.UUID{creatorID, targetID},
		}
		if err := s.publishEvent(ctx, chat.ID, domain.EventTypeChatCreated, payload); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return chat, nil
}

func (s *Service) CreateGroupChat(ctx context.Context, creatorID uuid.UUID, title string, avatarURL *string, initialMembers []uuid.UUID) (*domain.Chat, error) {
	// Validate title
	if title == "" || len(title) > 128 {
		return nil, domain.ErrValidationError
	}
	
	// Validate initial members count (2-1000)
	if len(initialMembers) < 1 || len(initialMembers) > 999 {
		return nil, domain.ErrMaxMembersReached
	}
	
	chat := &domain.Chat{
		ID:        uuid.New(),
		Type:      domain.ChatTypeGroup,
		Title:     &title,
		AvatarURL: avatarURL,
		CreatedBy: creatorID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := s.tm.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.chatRepo.Create(ctx, chat); err != nil {
			return err
		}

		// Creator is owner
		owner := &domain.ChatMember{
			ID:       uuid.New(),
			ChatID:   chat.ID,
			UserID:   creatorID,
			Role:     domain.RoleOwner,
			JoinedAt: chat.CreatedAt,
		}
		if err := s.memberRepo.Add(ctx, owner); err != nil {
			return err
		}

		// Add other initial members
		allMemberIDs := []uuid.UUID{creatorID}
		for _, uid := range initialMembers {
			if uid == creatorID {
				continue
			}
			m := &domain.ChatMember{
				ID:        uuid.New(),
				ChatID:    chat.ID,
				UserID:    uid,
				Role:      domain.RoleMember,
				InvitedBy: &creatorID,
				JoinedAt:  chat.CreatedAt,
			}
			if err := s.memberRepo.Add(ctx, m); err != nil {
				return err
			}
			allMemberIDs = append(allMemberIDs, uid)
		}

		// Event
		payload := domain.ChatCreatedPayload{
			ChatID:    chat.ID,
			Type:      chat.Type,
			Title:     chat.Title,
			CreatedBy: creatorID,
			MemberIDs: allMemberIDs,
		}
		if err := s.publishEvent(ctx, chat.ID, domain.EventTypeChatCreated, payload); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return chat, nil
}

func (s *Service) GetChat(ctx context.Context, chatID uuid.UUID) (*domain.Chat, error) {
	return s.chatRepo.GetByID(ctx, chatID)
}

func (s *Service) ListUserChats(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.Chat, error) {
	return s.chatRepo.ListByUserID(ctx, userID, limit, offset)
}

func (s *Service) UpdateChat(ctx context.Context, chatID, userID uuid.UUID, title *string, avatarURL *string) (*domain.Chat, error) {
	// Check permissions: only owner or admin can update group info?
	// Spec says: "owner can transfer ownership". It implies role management.
	// Assume admins/owners can update group info.

	chat, err := s.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	if chat.Type == domain.ChatTypeDirect {
		return nil, domain.ErrCannotUpdateDirectChat
	}

	// Check member role
	member, err := s.memberRepo.Get(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if member.Role != domain.RoleOwner && member.Role != domain.RoleAdmin {
		return nil, domain.ErrPermissionDenied
	}

	// Prepare updates
	changes := []string{}
	if title != nil && (chat.Title == nil || *title != *chat.Title) {
		chat.Title = title
		changes = append(changes, "title")
	}
	if avatarURL != nil && (chat.AvatarURL == nil || *avatarURL != *chat.AvatarURL) {
		chat.AvatarURL = avatarURL
		changes = append(changes, "avatar_url")
	}

	if len(changes) == 0 {
		return chat, nil
	}
    
    chat.UpdatedAt = time.Now().UTC()

	err = s.tm.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.chatRepo.Update(ctx, chat); err != nil {
			return err
		}

		payload := domain.ChatUpdatedPayload{
			ChatID:    chat.ID,
			UpdatedBy: userID,
			Changes:   changes,
		}
		return s.publishEvent(ctx, chat.ID, domain.EventTypeChatUpdated, payload)
	})

	return chat, err
}

func (s *Service) DeleteChat(ctx context.Context, chatID, userID uuid.UUID) error {
	// Check permissions: only owner?
	member, err := s.memberRepo.Get(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if member.Role != domain.RoleOwner {
		return domain.ErrPermissionDenied
	}

	// Get members for event payload before deletion (to notify them)
	members, err := s.memberRepo.ListByChatID(ctx, chatID)
	if err != nil {
		return err
	}
	memberIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		memberIDs[i] = m.UserID
	}

	return s.tm.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.chatRepo.Delete(ctx, chatID); err != nil {
			return err
		}

		payload := domain.ChatDeletedPayload{
			ChatID:    chatID,
			DeletedBy: userID,
			MemberIDs: memberIDs,
		}
		return s.publishEvent(ctx, chatID, domain.EventTypeChatDeleted, payload)
	})
}

// ListChatMembers returns active members of a chat with pagination
func (s *Service) ListChatMembers(ctx context.Context, chatID uuid.UUID, limit int, offset int) ([]*domain.ChatMember, error) {
	return s.memberRepo.ListByChatID(ctx, chatID) // Pagination applied at handler level for now
}
