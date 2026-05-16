package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// IdempotencyMiddleware handles Idempotency-Key header
type IdempotencyMiddleware struct {
	redis *redis.Client
}

// ResponseCache хранит кэшированный ответ
type ResponseCache struct {
	StatusCode int         `json:"status_code"`
	Body       interface{} `json:"body"`
	CachedAt   time.Time   `json:"cached_at"`
}

// NewIdempotencyMiddleware создаёт middleware
func NewIdempotencyMiddleware(client *redis.Client) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{
		redis: client,
	}
}

// Handler оборачивает обработчик с идемпотентностью
func (im *IdempotencyMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Только для POST/PUT/DELETE (создание, обновление, удаление)
		if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
			next.ServeHTTP(w, r)
			return
		}

		idempotencyKey := r.Header.Get("Idempotency-Key")
		if idempotencyKey == "" {
			// Нет ключа - просто выполняем
			next.ServeHTTP(w, r)
			return
		}

		if len(idempotencyKey) > 64 {
			http.Error(w, "Idempotency-Key too long (max 64)", http.StatusBadRequest)
			return
		}

		// Проверяем Redis
		cacheKey := "chat:dedup:" + idempotencyKey
		cached, err := im.redis.Get(r.Context(), cacheKey).Result()
		if err == nil {
			// Нашли в кэше - возвращаем сохранённый ответ
			var cache ResponseCache
			if err := json.Unmarshal([]byte(cached), &cache); err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotency-Replayed", "true")
				w.WriteHeader(cache.StatusCode)
				json.NewEncoder(w).Encode(cache.Body)
				return
			}
		}

		// Не в кэше - оборачиваем response writer для перехвата
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           []byte{},
		}

		next.ServeHTTP(wrapped, r)

		// Сохраняем в Redis (TTL 24 часа)
		cache := ResponseCache{
			StatusCode: wrapped.statusCode,
			Body:       json.RawMessage(wrapped.body),
			CachedAt:   time.Now(),
		}
		cacheBytes, _ := json.Marshal(cache)
		im.redis.Set(r.Context(), cacheKey, string(cacheBytes), 24*time.Hour)

		log.Debug().
			Str("idempotency_key", idempotencyKey).
			Int("status", wrapped.statusCode).
			Msg("Cached idempotent response")
	})
}

// responseWriterWrapper перехватывает ответ
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

// ServiceAuthContext добавляет service name в контекст
func ServiceAuthContext(serviceName string) context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, "service_name", serviceName)
}

