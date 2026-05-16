package http

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// LogOperation логирует операцию с full контекстом
func LogOperation(ctx context.Context, operation string, details map[string]interface{}) {
	requestID := GetRequestID(ctx)
	userID, _ := GetUserID(ctx)

	logger := log.Info().
		Str("operation", operation).
		Str("request_id", requestID)

	if userID != uuid.Nil {
		logger = logger.Str("user_id", userID.String())
	}

	for key, value := range details {
		logger = logger.Interface(key, value)
	}

	logger.Msg("Operation executed")
}

// LogError логирует ошибку с контекстом
func LogError(ctx context.Context, operation string, err error, details map[string]interface{}) {
	requestID := GetRequestID(ctx)
	userID, _ := GetUserID(ctx)

	logger := log.Error().
		Err(err).
		Str("operation", operation).
		Str("request_id", requestID)

	if userID != uuid.Nil {
		logger = logger.Str("user_id", userID.String())
	}

	for key, value := range details {
		logger = logger.Interface(key, value)
	}

	logger.Msg("Operation failed")
}

// LogWarning логирует warning с контекстом
func LogWarning(ctx context.Context, operation string, reason string, details map[string]interface{}) {
	requestID := GetRequestID(ctx)
	userID, _ := GetUserID(ctx)

	logger := log.Warn().
		Str("operation", operation).
		Str("reason", reason).
		Str("request_id", requestID)

	if userID != uuid.Nil {
		logger = logger.Str("user_id", userID.String())
	}

	for key, value := range details {
		logger = logger.Interface(key, value)
	}

	logger.Msg("Operation warning")
}

