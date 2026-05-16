package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
	"github.com/messenger/chat-service/internal/service"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Request/Response DTOs

type CreateChatRequest struct {
	Type      domain.ChatType `json:"type"`
	Title     *string         `json:"title,omitempty"`
	AvatarURL *string         `json:"avatar_url,omitempty"`
	UserIDs   []uuid.UUID     `json:"user_ids"`
}

type AddMemberRequest struct {
	UserID uuid.UUID `json:"user_id"`
}

type UpdateRoleRequest struct {
	Role domain.MemberRole `json:"role"`
}

type UpdateChatRequest struct {
	Title     *string `json:"title"`
	AvatarURL *string `json:"avatar_url"`
}

// ============================================================
// HEALTH ENDPOINTS
// ============================================================

func (h *Handler) HealthLive(w http.ResponseWriter, _ *http.Request) {
	RespondSuccess(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *Handler) HealthReady(w http.ResponseWriter, _ *http.Request) {
	RespondSuccess(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ============================================================
// CHAT ENDPOINTS
// ============================================================

// CreateChat - POST /api/v1/chats
func (h *Handler) CreateChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized, "Request authentication failed", nil)
		return
	}

	var req CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError, "Invalid request body", nil)
		return
	}

	var chat *domain.Chat
	var err error

	if req.Type == domain.ChatTypeDirect {
		// Validate
		if len(req.UserIDs) != 1 {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeValidationError,
				"Direct chat requires exactly 1 target user",
				domain.ValidationErrorDetails{Fields: map[string]string{"user_ids": "exactly 1 required"}})
			return
		}
		if req.UserIDs[0] == userID {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeValidationError,
				"Cannot create direct chat with yourself", nil)
			return
		}
		chat, err = h.svc.CreateDirectChat(ctx, userID, req.UserIDs[0])
	} else if req.Type == domain.ChatTypeGroup {
		// Validate
		if req.Title == nil || *req.Title == "" {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeValidationError,
				"Group chat requires title",
				domain.ValidationErrorDetails{Fields: map[string]string{"title": "required for group chats"}})
			return
		}
		if len(*req.Title) > 128 {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeValidationError,
				"Title too long",
				domain.ValidationErrorDetails{Fields: map[string]string{"title": "max 128 characters"}})
			return
		}
		// 1000 is checked in service too, but good to check early
		if len(req.UserIDs) > 1000 {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeMembersLimitExceeded,
				"Members limit exceeded", nil)
			return
		}
		title := *req.Title
		chat, err = h.svc.CreateGroupChat(ctx, userID, title, req.AvatarURL, req.UserIDs)
	} else {
		RespondError(w, http.StatusUnprocessableEntity, domain.CodeValidationError,
			"Invalid chat type",
			domain.ValidationErrorDetails{Fields: map[string]string{"type": "direct or group"}})
		return
	}

	if err != nil {
		if errors.Is(err, domain.ErrDirectChatExists) {
			RespondError(w, http.StatusConflict, domain.CodeDirectChatAlreadyExists,
				"Direct chat between these users already exists", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to create chat")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to create chat", nil)
		return
	}

	GetMetricsCollector().RecordChatCreated()
	RespondSuccess(w, http.StatusCreated, chat)
}

// GetChat - GET /api/v1/chats/{chat_id}
func (h *Handler) GetChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	// Check membership
	if err := h.svc.IsChatMember(ctx, chatID, userID); err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden,
				"Access denied: not a member of this chat", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to check membership")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to get chat", nil)
		return
	}

	chat, err := h.svc.GetChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to get chat")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to get chat", nil)
		return
	}

	RespondSuccess(w, http.StatusOK, chat)
}

// ListChats - GET /api/v1/chats
func (h *Handler) ListChats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	limit := 20

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	// Using offset for now as cursor - will fetch limit+1 to check if more exist
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	chats, err := h.svc.ListUserChats(ctx, userID, limit+1, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list chats")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to list chats", nil)
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

	resp := NewChatListResponse(chats, nextCursor, hasNext)
	RespondSuccess(w, http.StatusOK, resp)
}

// UpdateChat - PATCH /api/v1/chats/{chat_id}
func (h *Handler) UpdateChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	var req UpdateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid request body", nil)
		return
	}

	// Validate
	if req.Title != nil && len(*req.Title) > 128 {
		RespondError(w, http.StatusUnprocessableEntity, domain.CodeValidationError,
			"Title too long",
			domain.ValidationErrorDetails{Fields: map[string]string{"title": "max 128 characters"}})
		return
	}

	chat, err := h.svc.UpdateChat(ctx, chatID, userID, req.Title, req.AvatarURL)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		if errors.Is(err, domain.ErrPermissionDenied) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden,
				"Insufficient permissions", nil)
			return
		}
		if errors.Is(err, domain.ErrCannotUpdateDirectChat) {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeCannotModifyDirectChat,
				"Cannot update direct chat info", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to update chat")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to update chat", nil)
		return
	}

	RespondSuccess(w, http.StatusOK, chat)
}

// DeleteChat - DELETE /api/v1/chats/{chat_id}
func (h *Handler) DeleteChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	err = h.svc.DeleteChat(ctx, chatID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		if errors.Is(err, domain.ErrPermissionDenied) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden,
				"Only owner can delete chat", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to delete chat")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to delete chat", nil)
		return
	}

	GetMetricsCollector().RecordChatDeleted()
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// MEMBERS ENDPOINTS
// ============================================================

// ListMembers - GET /api/v1/chats/{chat_id}/members
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	// Check membership
	if err := h.svc.IsChatMember(ctx, chatID, userID); err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden,
				"Access denied: not a member of this chat", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to check membership")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to list members", nil)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	members, err := h.svc.ListChatMembers(ctx, chatID, limit+1, offset)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to list members")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to list members", nil)
		return
	}

	if members == nil {
		members = []*domain.ChatMember{}
	}

	hasNext := len(members) > limit
	if hasNext {
		members = members[:limit]
	}

	var nextCursor *string
	if hasNext && len(members) > 0 {
		nextOffsetStr := strconv.Itoa(offset + limit)
		nextCursor = &nextOffsetStr
	}

	resp := NewMemberListResponse(members, nextCursor, hasNext)
	RespondSuccess(w, http.StatusOK, resp)
}

// GetMember - GET /api/v1/chats/{chat_id}/members/{user_id}
func (h *Handler) GetMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	// Check membership
	if err := h.svc.IsChatMember(ctx, chatID, userID); err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden,
				"Access denied: not a member of this chat", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to check membership")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to get member", nil)
		return
	}

	memberID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid user ID format", nil)
		return
	}

	member, err := h.svc.IsMember(ctx, chatID, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeMemberNotFound,
				"Member not found in this chat", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to get member")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to get member", nil)
		return
	}

	RespondSuccess(w, http.StatusOK, member)
}

// AddMember - POST /api/v1/chats/{chat_id}/members
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	inviterID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid request body", nil)
		return
	}

	err = h.svc.AddMember(ctx, chatID, req.UserID, inviterID)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionDenied) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden,
				"Permission denied", nil)
			return
		}
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		if errors.Is(err, domain.ErrAlreadyMember) {
			RespondError(w, http.StatusConflict, domain.CodeConflict,
				"User is already a member of this chat", nil)
			return
		}
		if errors.Is(err, domain.ErrCannotUpdateDirectChat) {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeCannotModifyDirectChat,
				"Cannot modify direct chat", nil)
			return
		}
		if errors.Is(err, domain.ErrMaxMembersReached) {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeMembersLimitExceeded,
				"Members limit exceeded", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to add member")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to add member", nil)
		return
	}

	GetMetricsCollector().RecordMemberAdded()
	w.WriteHeader(http.StatusCreated)
}

// RemoveMember - DELETE /api/v1/chats/{chat_id}/members/{user_id}
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	removerID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid user ID format", nil)
		return
	}

	err = h.svc.RemoveMember(ctx, chatID, targetID, removerID)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionDenied) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden,
				"Permission denied", nil)
			return
		}
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		if errors.Is(err, domain.ErrMemberNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeMemberNotFound,
				"Member not found", nil)
			return
		}
		if errors.Is(err, domain.ErrOwnerMustTransferBeforeLeave) {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeOwnerMustTransferBeforeLeave,
				"Owner must transfer ownership before leaving", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to remove member")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to remove member", nil)
		return
	}

	GetMetricsCollector().RecordMemberRemoved()
	w.WriteHeader(http.StatusNoContent)
}

// UpdateMemberRole - PATCH /api/v1/chats/{chat_id}/members/{user_id}
func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	updaterID, ok := GetUserID(ctx)
	if !ok {
		RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			"Request authentication failed", nil)
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid chat ID format", nil)
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid user ID format", nil)
		return
	}

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError,
			"Invalid request body", nil)
		return
	}

	// Validate Role
	if req.Role != domain.RoleOwner && req.Role != domain.RoleAdmin && req.Role != domain.RoleMember {
		RespondError(w, http.StatusUnprocessableEntity, domain.CodeValidationError,
			"Invalid role",
			domain.ValidationErrorDetails{Fields: map[string]string{"role": "must be owner, admin or member"}})
		return
	}

	err = h.svc.ChangeRole(ctx, chatID, targetID, req.Role, updaterID)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionDenied) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden,
				"Permission denied", nil)
			return
		}
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound,
				"Chat not found", nil)
			return
		}
		if errors.Is(err, domain.ErrMemberNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeMemberNotFound,
				"Member not found", nil)
			return
		}
		if errors.Is(err, domain.ErrCannotTransferOwnerToSelf) {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeCannotTransferOwnerToSelf,
				"Cannot transfer ownership to yourself", nil)
			return
		}
		if errors.Is(err, domain.ErrOwnerTransferTargetInvalid) {
			RespondError(w, http.StatusUnprocessableEntity, domain.CodeOwnerTransferTargetInvalid,
				"Invalid target for ownership transfer", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to update member role")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
			"Failed to update member role", nil)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	// Apply middleware
	r.Use(ErrorResponseMiddleware)
	r.Use(MetricsMiddleware)

	// Health
	r.Get("/health/live", h.HealthLive)
	r.Get("/health/ready", h.HealthReady)
	r.Get("/health/detailed", HealthMetricsHandler)

	// Metrics
	r.Get("/metrics", MetricsHandler)

	// Public API
	r.Route("/api/v1/chats", func(r chi.Router) {
		r.Use(AuthMiddleware)
		r.Get("/", h.ListChats)
		r.Post("/", h.CreateChat)
		r.Route("/{chat_id}", func(r chi.Router) {
			r.Get("/", h.GetChat)
			r.Patch("/", h.UpdateChat)
			r.Delete("/", h.DeleteChat)

			r.Route("/members", func(r chi.Router) {
				r.Get("/", h.ListMembers)
				r.Post("/", h.AddMember)
				r.Route("/{user_id}", func(r chi.Router) {
					r.Get("/", h.GetMember)
					r.Delete("/", h.RemoveMember)
					r.Patch("/", h.UpdateMemberRole)
				})
			})

			r.Route("/metadata", func(r chi.Router) {
				r.Get("/{key}", h.GetMetadata)
				r.Put("/{key}", h.UpsertMetadata)
				r.Delete("/{key}", h.DeleteMetadata)
			})
		})
	})

	// Internal API (Service-to-Service)
	r.Route("/api/v1/internal", func(r chi.Router) {
		r.Get("/chats/{chat_id}/members/{user_id}", h.InternalGetMember)
		r.Get("/chats/{chat_id}/snapshot", h.InternalGetChatSnapshot)
		r.Get("/users/{user_id}/chats", h.InternalListUserChats)
	})
}
