package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
	"github.com/rs/zerolog/log"
)

// Allowed service names for internal endpoints
var AllowedServiceNames = map[string]bool{
	"message-service":  true,
	"realtime-gateway": true,
}

// validateServiceAuth проверяет X-Service-Name header и Authorization token
func validateServiceAuth(w http.ResponseWriter, r *http.Request) bool {
	serviceName := r.Header.Get("X-Service-Name")
	if serviceName == "" {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Missing X-Service-Name header", nil)
		return false
	}

	if !AllowedServiceNames[serviceName] {
		RespondError(w, http.StatusForbidden, domain.CodeForbidden,
			"Service not allowed", nil)
		return false
	}

	// Validate JWT token
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Extract token from "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			token := parts[1]

			// Validate token
			claims, err := ValidateServiceToken(token, serviceName)
			if err != nil {
				RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
					"Invalid token: "+err.Error(), nil)
				return false
			}

			// Validate claims
			if err := ValidateClaimsForService(claims, serviceName); err != nil {
				RespondError(w, http.StatusForbidden, domain.CodeForbidden,
					"Invalid claims: "+err.Error(), nil)
				return false
			}
		} else {
			RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
				"Invalid Authorization header format", nil)
			return false
		}
	}

	return true
}

// Internal Endpoints

// InternalGetMember - GET /api/v1/internal/chats/{chat_id}/members/{user_id}
func (h *Handler) InternalGetMember(w http.ResponseWriter, r *http.Request) {
	if !validateServiceAuth(w, r) {
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid user ID format", nil)
		return
	}

	chat, members, err := h.svc.GetChatSnapshot(r.Context(), chatID)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to get chat snapshot")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to get member", nil)
		return
	}

	var member *domain.ChatMember
	for _, m := range members {
		if m.UserID == userID {
			member = m
			break
		}
	}

	isMember := member != nil && member.LeftAt == nil
	role := ""
	var joinedAt *time.Time
	if isMember {
		role = string(member.Role)
		joinedAt = &member.JoinedAt
	}

	dto := InternalChatMembershipDTO{
		ChatID:    chatID,
		UserID:    userID,
		IsMember:  isMember,
		Role:      &role,
		JoinedAt:  joinedAt,
		ChatType:  string(chat.Type),
		CheckedAt: time.Now().UTC(),
	}

	RespondSuccess(w, http.StatusOK, dto)
}

// InternalGetChatSnapshot - GET /api/v1/internal/chats/{chat_id}/snapshot
func (h *Handler) InternalGetChatSnapshot(w http.ResponseWriter, r *http.Request) {
	if !validateServiceAuth(w, r) {
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	chat, members, err := h.svc.GetChatSnapshot(r.Context(), chatID)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to get chat snapshot")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to get chat snapshot", nil)
		return
	}

	resp := NewInternalChatSnapshotDTO(chat, members)
	RespondSuccess(w, http.StatusOK, resp)
}

// InternalListUserChats - GET /api/v1/internal/users/{user_id}/chats
func (h *Handler) InternalListUserChats(w http.ResponseWriter, r *http.Request) {
	if !validateServiceAuth(w, r) {
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid user ID format", nil)
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	chats, err := h.svc.ListUserChats(r.Context(), userID, limit+1, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list user chats")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to list user chats", nil)
		return
	}

	if chats == nil {
		chats = []*domain.Chat{}
	}

	hasNext := len(chats) > limit
	if hasNext {
		chats = chats[:limit]
	}

	var nextCursor *string
	if hasNext && len(chats) > 0 {
		nextOffsetStr := strconv.Itoa(offset + limit)
		nextCursor = &nextOffsetStr
	}

	refs := make([]*InternalUserChatRefDTO, len(chats))
	for i, c := range chats {
		// Get membership to determine role and joined_at
		member, _ := h.svc.IsMember(r.Context(), c.ID, userID)
		role := "member"
		joinedAt := c.CreatedAt
		if member != nil {
			role = string(member.Role)
			joinedAt = member.JoinedAt
		}
		refs[i] = DomainUserChatRefToDTO(c, role, joinedAt)
	}

	resp := &InternalUserChatRefListResponse{
		Items:      refs,
		Pagination: NewPaginationDTO(nextCursor, hasNext),
	}
	RespondSuccess(w, http.StatusOK, resp)
}
