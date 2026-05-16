package http

import (
	"context"
	"time"
)

// Key represents an idempotent request key
type Key struct {
	IdempotencyKey string
	Method         string
	Path           string
}

// Response represents a cached response
type Response struct {
	StatusCode int
	Body       string
	ETag       string
	Timestamp  time.Time
}

// Cache interface for idempotency storage
type Cache interface {
	// Get retrieves a cached response by key
	Get(ctx context.Context, key string) (*Response, error)

	// Set stores a response with TTL (24 hours default)
	Set(ctx context.Context, key string, resp *Response, ttl time.Duration) error

	// Delete removes a cached response
	Delete(ctx context.Context, key string) error
}
