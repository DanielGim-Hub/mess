package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
)

func (s *Service) AddMember(ctx context.Context, chatID, userID, inviterID uuid.UUID) error {
	chat, err := s.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return err
	}
	if chat.Type == domain.ChatTypeDirect {
		// Cannot modify direct chat to add members
		return domain.ErrCannotUpdateDirectChat
	}

	// Check inviter permissions
	inviter, err := s.memberRepo.Get(ctx, chatID, inviterID)
	if err != nil {
		return err
	}
	if inviter.Role != domain.RoleOwner && inviter.Role != domain.RoleAdmin {
		return domain.ErrPermissionDenied
	}

	// Check if already member
	_, err = s.memberRepo.Get(ctx, chatID, userID)
	if err == nil {
		return domain.ErrAlreadyMember
	}

	// Check members count limit (max 1000)
	count, err := s.memberRepo.Count(ctx, chatID)
	if err != nil {
		return err
	}
	if count >= 1000 {
		return domain.ErrMaxMembersReached
	}

	return s.tm.RunInTx(ctx, func(ctx context.Context) error {
		member := &domain.ChatMember{
			ID:        uuid.New(),
			ChatID:    chatID,
			UserID:    userID,
			Role:      domain.RoleMember,
			InvitedBy: &inviterID,
			JoinedAt:  time.Now().UTC(),
		}
		if err := s.memberRepo.Add(ctx, member); err != nil {
			return err
		}

		payload := domain.ChatUpdatedPayload{
			ChatID:       chatID,
			UpdatedBy:    inviterID,
			Changes:      []string{"members"},
			MembersAdded: []uuid.UUID{userID},
		}
		return s.publishEvent(ctx, chatID, domain.EventTypeChatUpdated, payload)
	})
}

func (s *Service) RemoveMember(ctx context.Context, chatID, userID, removerID uuid.UUID) error {
	member, err := s.memberRepo.Get(ctx, chatID, userID)
	if err != nil {
		return err
	}

	remover, err := s.memberRepo.Get(ctx, chatID, removerID)
	if err != nil {
		return err
	}

	// Permission logic
	isSelf := userID == removerID
	if isSelf {
		// Leaving
		if member.Role == domain.RoleOwner {
			return domain.ErrOwnerMustTransferBeforeLeave
		}
	} else {
		// Kicking
		if remover.Role == domain.RoleMember {
			return domain.ErrPermissionDenied
		}
		if remover.Role == domain.RoleAdmin && member.Role != domain.RoleMember {
			// Admin can only kick members, not other admins (common rule)
			// Spec: "admin can... change their roles (except owner)". Kicking is removing.
			// Let's protect admins from admins for safety, or follow spec loosely "except owner".
			// If spec says "except owner", maybe admin can kick admin.
			// Let's assume Admin cannot kick Admin/Owner.
			if member.Role == domain.RoleAdmin || member.Role == domain.RoleOwner {
				return domain.ErrPermissionDenied
			}
		}
		// Owner can kick anyone.
	}

	return s.tm.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.memberRepo.Remove(ctx, chatID, userID); err != nil {
			return err
		}

		payload := domain.ChatUpdatedPayload{
			ChatID:         chatID,
			UpdatedBy:      removerID,
			Changes:        []string{"members"},
			MembersRemoved: []uuid.UUID{userID},
		}
		return s.publishEvent(ctx, chatID, domain.EventTypeChatUpdated, payload)
	})
}

func (s *Service) ChangeRole(ctx context.Context, chatID, userID uuid.UUID, newRole domain.MemberRole, updaterID uuid.UUID) error {
	updater, err := s.memberRepo.Get(ctx, chatID, updaterID)
	if err != nil {
		return err
	}
	target, err := s.memberRepo.Get(ctx, chatID, userID)
	if err != nil {
		return err
	}

	if newRole == domain.RoleOwner {
		if updater.Role != domain.RoleOwner {
			return domain.ErrPermissionDenied
		}
		
		// Cannot transfer owner to self
		if userID == updaterID {
			return domain.ErrCannotTransferOwnerToSelf
		}
		
		// Target must be an active member (not left)
		if target.LeftAt != nil {
			return domain.ErrOwnerTransferTargetInvalid
		}
		
		// Transfer ownership
		return s.tm.RunInTx(ctx, func(ctx context.Context) error {
			// Demote updater to Admin
			if err := s.memberRepo.UpdateRole(ctx, chatID, updaterID, domain.RoleAdmin); err != nil {
				return err
			}
			// Promote target to Owner
			if err := s.memberRepo.UpdateRole(ctx, chatID, userID, domain.RoleOwner); err != nil {
				return err
			}

			payload := domain.ChatUpdatedPayload{
				ChatID:    chatID,
				UpdatedBy: updaterID,
				Changes:   []string{"roles"},
			}
			return s.publishEvent(ctx, chatID, domain.EventTypeChatUpdated, payload)
		})
	}

	// Non-owner role change
	if updater.Role != domain.RoleOwner && updater.Role != domain.RoleAdmin {
		return domain.ErrPermissionDenied
	}
	// Admin cannot change role of Owner or other Admin?
	if target.Role == domain.RoleOwner {
		return domain.ErrPermissionDenied
	}
	if updater.Role == domain.RoleAdmin {
		// Admin changing someone.
		if target.Role == domain.RoleAdmin {
			return domain.ErrPermissionDenied // Admin cannot demote admin
		}
		if newRole == domain.RoleAdmin {
			// Admin creating another admin? Allowed?
			// Spec: "admin can... change their roles".
			// Let's assume yes.
		}
	}

	return s.tm.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.memberRepo.UpdateRole(ctx, chatID, userID, newRole); err != nil {
			return err
		}
		payload := domain.ChatUpdatedPayload{
			ChatID:    chatID,
			UpdatedBy: updaterID,
			Changes:   []string{"roles"},
		}
		return s.publishEvent(ctx, chatID, domain.EventTypeChatUpdated, payload)
	})
}
