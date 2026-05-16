package domain

import (
	"encoding/json"
)

// Error Response структура согласно контракту
type ErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

type ErrorDetails struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Error codes (snake_case) - Chat Service specific
const (
	CodeUnauthorized                 = "unauthorized"
	CodeTokenExpired                 = "token_expired"
	CodeForbidden                    = "forbidden"
	CodeNotFound                     = "not_found"
	CodeValidationError              = "validation_error"
	CodeConflict                     = "conflict"
	CodeRateLimitExceeded            = "rate_limit_exceeded"
	CodeInternalError                = "internal_error"
	CodeChatNotFound                 = "chat_not_found"
	CodeMemberNotFound               = "member_not_found"
	CodeDirectChatAlreadyExists      = "direct_chat_already_exists"
	CodeCannotModifyDirectChat       = "cannot_modify_direct_chat"
	CodeCannotRemoveOwner            = "cannot_remove_owner"
	CodeOwnerMustTransferBeforeLeave = "owner_must_transfer_before_leave"
	CodeCannotTransferOwnerToSelf    = "cannot_transfer_owner_to_self"
	CodeOwnerTransferTargetInvalid   = "owner_transfer_target_invalid"
	CodeMembersLimitExceeded         = "members_limit_exceeded"
	CodePermissionDenied             = "permission_denied"
	CodeMetadataNotFound             = "metadata_not_found"
)

// Domain errors mapped to codes
var ErrorCodeMap = map[string]string{
	"chat not found":             CodeChatNotFound,
	"member not found":           CodeMemberNotFound,
	"direct chat already exists": CodeDirectChatAlreadyExists,
	"invalid chat type":          CodeValidationError,
	"max members reached":        CodeMembersLimitExceeded,
	"permission denied":          CodePermissionDenied,
	"metadata not found":         CodeMetadataNotFound,
	"owner cannot leave":         CodeOwnerMustTransferBeforeLeave,
	"cannot modify direct chat":  CodeCannotModifyDirectChat,
}

// Validation error details
type ValidationErrorDetails struct {
	Fields map[string]string `json:"fields"`
}

// NewErrorResponse creates a standard error response
func NewErrorResponse(code, message string, details interface{}) *ErrorResponse {
	return &ErrorResponse{
		Error: ErrorDetails{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// MarshalJSON marshals the error response
func (er *ErrorResponse) MarshalJSON() ([]byte, error) {
	type Alias ErrorResponse
	return json.Marshal((*Alias)(er))
}

// GetErrorCode maps error messages to error codes
func GetErrorCode(err string) string {
	if code, ok := ErrorCodeMap[err]; ok {
		return code
	}
	return CodeInternalError
}

// Common error constructors
func NewUnauthorizedError() *ErrorResponse {
	return NewErrorResponse(
		CodeUnauthorized,
		"Request authentication failed",
		nil,
	)
}

func NewForbiddenError() *ErrorResponse {
	return NewErrorResponse(
		CodeForbidden,
		"Insufficient permissions",
		nil,
	)
}

func NewNotFoundError(resource string) *ErrorResponse {
	return NewErrorResponse(
		CodeNotFound,
		"Resource not found: "+resource,
		nil,
	)
}

func NewValidationError(fields map[string]string) *ErrorResponse {
	return NewErrorResponse(
		CodeValidationError,
		"Request validation failed",
		ValidationErrorDetails{Fields: fields},
	)
}

func NewConflictError(message string) *ErrorResponse {
	return NewErrorResponse(
		CodeConflict,
		message,
		nil,
	)
}

func NewInternalError(message string) *ErrorResponse {
	return NewErrorResponse(
		CodeInternalError,
		message,
		nil,
	)
}
