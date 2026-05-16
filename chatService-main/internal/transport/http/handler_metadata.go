package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
	"github.com/rs/zerolog/log"
)

// Metadata Endpoints

type UpsertMetadataRequest struct {
	Value json.RawMessage `json:"value"`
}

// UpsertMetadata - PUT /api/v1/chats/{chat_id}/metadata/{key}
func (h *Handler) UpsertMetadata(w http.ResponseWriter, r *http.Request) {
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
			"Failed to upsert metadata", nil)
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError, 
			"Metadata key is required", 
			domain.ValidationErrorDetails{Fields: map[string]string{"key": "required"}})
		return
	}

	var req UpsertMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError, 
			"Invalid request body", nil)
		return
	}

	err = h.svc.UpsertMetadata(ctx, chatID, userID, key, req.Value)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionDenied) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden, 
				"Insufficient permissions", nil)
			return
		}
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound, 
				"Chat not found", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to upsert metadata")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError, 
			"Failed to upsert metadata", nil)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetMetadata - GET /api/v1/chats/{chat_id}/metadata/{key}
func (h *Handler) GetMetadata(w http.ResponseWriter, r *http.Request) {
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
			"Failed to get metadata", nil)
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError, 
			"Metadata key is required", nil)
		return
	}

	meta, err := h.svc.GetMetadata(ctx, chatID, key)
	if err != nil {
		if errors.Is(err, domain.ErrMetadataNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeMetadataNotFound, 
				"Metadata not found", nil)
			return
		}
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound, 
				"Chat not found", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to get metadata")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError, 
			"Failed to get metadata", nil)
		return
	}

	RespondSuccess(w, http.StatusOK, meta)
}

// DeleteMetadata - DELETE /api/v1/chats/{chat_id}/metadata/{key}
func (h *Handler) DeleteMetadata(w http.ResponseWriter, r *http.Request) {
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
			"Failed to delete metadata", nil)
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		RespondError(w, http.StatusBadRequest, domain.CodeValidationError, 
			"Metadata key is required", nil)
		return
	}

	err = h.svc.DeleteMetadata(ctx, chatID, userID, key)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionDenied) {
			RespondError(w, http.StatusForbidden, domain.CodeForbidden, 
				"Insufficient permissions", nil)
			return
		}
		if errors.Is(err, domain.ErrMetadataNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeMetadataNotFound, 
				"Metadata not found", nil)
			return
		}
		if errors.Is(err, domain.ErrChatNotFound) {
			RespondError(w, http.StatusNotFound, domain.CodeChatNotFound, 
				"Chat not found", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to delete metadata")
		RespondError(w, http.StatusInternalServerError, domain.CodeInternalError, 
			"Failed to delete metadata", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}


