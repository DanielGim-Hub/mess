package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/messenger/chat-service/internal/domain"
	"github.com/rs/zerolog/log"
)

type contextKey string

const (
	UserIDKey      contextKey = "user_id"
	RequestIDKey   contextKey = "request_id"
	ServiceNameKey contextKey = "service_name"
)

// AuthMiddleware валидирует X-User-Id header и инжектит в контекст
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.Header.Get("X-User-Id")
		if userIDStr == "" {
			RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized, "Request authentication failed", nil)
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized, "Invalid User ID format", nil)
			return
		}

		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ErrorResponseMiddleware обрабатывает panics
func ErrorResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := r.Header.Get("X-Request-Id")
				log.Error().
					Interface("panic", err).
					Str("request_id", requestID).
					Str("method", r.Method).
					Str("path", r.RequestURI).
					Msg("Panic in handler")
				RespondError(w, http.StatusInternalServerError, domain.CodeInternalError,
					"Internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RespondError отправляет стандартный error response
func RespondError(w http.ResponseWriter, statusCode int, code, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := domain.NewErrorResponse(code, message, details)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("Failed to encode error response")
	}
}

// RespondSuccess отправляет успешный JSON response
func RespondSuccess(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Error().Err(err).Msg("Failed to encode response")
		}
	}
}

// GetUserID извлекает user ID из контекста
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return id, ok
}

// GetRequestID извлекает request ID из контекста
func GetRequestID(ctx context.Context) string {
	id, ok := ctx.Value(RequestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}

// GetServiceName извлекает service name из контекста
func GetServiceName(ctx context.Context) string {
	name, ok := ctx.Value(ServiceNameKey).(string)
	if !ok {
		return ""
	}
	return name
}

// RateLimitMiddleware добавляет rate limit headers
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement rate limiting with Redis counter
		// For now just add headers
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", "99")
		w.Header().Set("X-RateLimit-Reset", "3600")
		
		next.ServeHTTP(w, r)
	})
}

// ServiceAuthMiddleware валидирует service token (для internal endpoints)
// Примечание: используется отдельно, не с AuthMiddleware
func ServiceAuthMiddleware(allowedServices map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serviceName := r.Header.Get("X-Service-Name")
			if serviceName == "" {
				RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
					"Missing X-Service-Name header", nil)
				return
			}

			if !allowedServices[serviceName] {
				RespondError(w, http.StatusForbidden, domain.CodeForbidden,
					"Service not allowed", nil)
				return
			}

			// TODO: Validate JWT token
			// authHeader := r.Header.Get("Authorization")
			// if authHeader == "" {
			//     RespondError(w, http.StatusUnauthorized, domain.CodeUnauthorized,
			//         "Missing Authorization header", nil)
			//     return
			// }
			// Validate JWT claims...

			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			ctx := context.WithValue(r.Context(), ServiceNameKey, serviceName)
			ctx = context.WithValue(ctx, RequestIDKey, requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

