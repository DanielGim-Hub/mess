package http

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MetricsCollector собирает метрики приложения
type MetricsCollector struct {
	mu sync.RWMutex

	// Counters
	HTTPRequestsTotal    int64
	HTTPErrorsTotal      int64
	ChatCreatedTotal     int64
	ChatDeletedTotal     int64
	MemberAddedTotal     int64
	MemberRemovedTotal   int64
	EventsPublishedTotal int64

	// Histograms (simplified)
	RequestLatencies []int64 // в миллисекундах
	DBLatencies      []int64

	// Gauges
	ActiveConnections int64
	PendingEvents     int64
}

var metricsCollector = &MetricsCollector{
	RequestLatencies: []int64{},
	DBLatencies:      []int64{},
}

// RecordHTTPRequest записывает метрику HTTP запроса
func (mc *MetricsCollector) RecordHTTPRequest(statusCode int, latencyMs int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.HTTPRequestsTotal++
	if statusCode >= 400 {
		mc.HTTPErrorsTotal++
	}
	mc.RequestLatencies = append(mc.RequestLatencies, latencyMs)
}

// RecordChatCreated записывает создание чата
func (mc *MetricsCollector) RecordChatCreated() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ChatCreatedTotal++
}

// RecordChatDeleted записывает удаление чата
func (mc *MetricsCollector) RecordChatDeleted() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ChatDeletedTotal++
}

// RecordMemberAdded записывает добавление участника
func (mc *MetricsCollector) RecordMemberAdded() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.MemberAddedTotal++
}

// RecordMemberRemoved записывает удаление участника
func (mc *MetricsCollector) RecordMemberRemoved() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.MemberRemovedTotal++
}

// RecordEventPublished записывает публикацию события
func (mc *MetricsCollector) RecordEventPublished() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.EventsPublishedTotal++
}

// GetMetrics возвращает копию метрик
func (mc *MetricsCollector) GetMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Calculate averages
	avgReqLatency := int64(0)
	if len(mc.RequestLatencies) > 0 {
		total := int64(0)
		for _, lat := range mc.RequestLatencies {
			total += lat
		}
		avgReqLatency = total / int64(len(mc.RequestLatencies))
	}

	avgDBLatency := int64(0)
	if len(mc.DBLatencies) > 0 {
		total := int64(0)
		for _, lat := range mc.DBLatencies {
			total += lat
		}
		avgDBLatency = total / int64(len(mc.DBLatencies))
	}

	// Calculate percentiles (simplified p95)
	p95ReqLatency := int64(0)
	if len(mc.RequestLatencies) > 0 {
		idx := len(mc.RequestLatencies) * 95 / 100
		if idx < len(mc.RequestLatencies) {
			p95ReqLatency = mc.RequestLatencies[idx]
		}
	}

	errorRate := float64(0)
	if mc.HTTPRequestsTotal > 0 {
		errorRate = float64(mc.HTTPErrorsTotal) / float64(mc.HTTPRequestsTotal) * 100
	}

	return map[string]interface{}{
		"http_requests_total":         mc.HTTPRequestsTotal,
		"http_errors_total":           mc.HTTPErrorsTotal,
		"http_error_rate_percent":     errorRate,
		"http_request_latency_avg_ms": avgReqLatency,
		"http_request_latency_p95_ms": p95ReqLatency,
		"db_latency_avg_ms":           avgDBLatency,
		"chats_created_total":         mc.ChatCreatedTotal,
		"chats_deleted_total":         mc.ChatDeletedTotal,
		"members_added_total":         mc.MemberAddedTotal,
		"members_removed_total":       mc.MemberRemovedTotal,
		"events_published_total":      mc.EventsPublishedTotal,
		"active_connections":          mc.ActiveConnections,
		"pending_events":              mc.PendingEvents,
	}
}

// MetricsHandler обслуживает /metrics endpoint
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := metricsCollector.GetMetrics()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	// Prometheus format
	fmt.Fprintf(w, "# HELP chat_service_http_requests_total Total HTTP requests\n")
	fmt.Fprintf(w, "# TYPE chat_service_http_requests_total counter\n")
	fmt.Fprintf(w, "chat_service_http_requests_total %d\n", metrics["http_requests_total"])

	fmt.Fprintf(w, "# HELP chat_service_http_errors_total Total HTTP errors\n")
	fmt.Fprintf(w, "# TYPE chat_service_http_errors_total counter\n")
	fmt.Fprintf(w, "chat_service_http_errors_total %d\n", metrics["http_errors_total"])

	fmt.Fprintf(w, "# HELP chat_service_http_error_rate_percent HTTP error rate percentage\n")
	fmt.Fprintf(w, "# TYPE chat_service_http_error_rate_percent gauge\n")
	fmt.Fprintf(w, "chat_service_http_error_rate_percent %.2f\n", metrics["http_error_rate_percent"])

	fmt.Fprintf(w, "# HELP chat_service_http_request_latency_avg_ms Average HTTP request latency\n")
	fmt.Fprintf(w, "# TYPE chat_service_http_request_latency_avg_ms gauge\n")
	fmt.Fprintf(w, "chat_service_http_request_latency_avg_ms %d\n", metrics["http_request_latency_avg_ms"])

	fmt.Fprintf(w, "# HELP chat_service_http_request_latency_p95_ms 95th percentile HTTP request latency\n")
	fmt.Fprintf(w, "# TYPE chat_service_http_request_latency_p95_ms gauge\n")
	fmt.Fprintf(w, "chat_service_http_request_latency_p95_ms %d\n", metrics["http_request_latency_p95_ms"])

	fmt.Fprintf(w, "# HELP chat_service_db_latency_avg_ms Average database latency\n")
	fmt.Fprintf(w, "# TYPE chat_service_db_latency_avg_ms gauge\n")
	fmt.Fprintf(w, "chat_service_db_latency_avg_ms %d\n", metrics["db_latency_avg_ms"])

	fmt.Fprintf(w, "# HELP chat_service_chats_created_total Total chats created\n")
	fmt.Fprintf(w, "# TYPE chat_service_chats_created_total counter\n")
	fmt.Fprintf(w, "chat_service_chats_created_total %d\n", metrics["chats_created_total"])

	fmt.Fprintf(w, "# HELP chat_service_chats_deleted_total Total chats deleted\n")
	fmt.Fprintf(w, "# TYPE chat_service_chats_deleted_total counter\n")
	fmt.Fprintf(w, "chat_service_chats_deleted_total %d\n", metrics["chats_deleted_total"])

	fmt.Fprintf(w, "# HELP chat_service_members_added_total Total members added\n")
	fmt.Fprintf(w, "# TYPE chat_service_members_added_total counter\n")
	fmt.Fprintf(w, "chat_service_members_added_total %d\n", metrics["members_added_total"])

	fmt.Fprintf(w, "# HELP chat_service_members_removed_total Total members removed\n")
	fmt.Fprintf(w, "# TYPE chat_service_members_removed_total counter\n")
	fmt.Fprintf(w, "chat_service_members_removed_total %d\n", metrics["members_removed_total"])

	fmt.Fprintf(w, "# HELP chat_service_events_published_total Total events published\n")
	fmt.Fprintf(w, "# TYPE chat_service_events_published_total counter\n")
	fmt.Fprintf(w, "chat_service_events_published_total %d\n", metrics["events_published_total"])

	fmt.Fprintf(w, "# HELP chat_service_active_connections Active connections\n")
	fmt.Fprintf(w, "# TYPE chat_service_active_connections gauge\n")
	fmt.Fprintf(w, "chat_service_active_connections %d\n", metrics["active_connections"])

	fmt.Fprintf(w, "# HELP chat_service_pending_events Pending events\n")
	fmt.Fprintf(w, "# TYPE chat_service_pending_events gauge\n")
	fmt.Fprintf(w, "chat_service_pending_events %d\n", metrics["pending_events"])

	log.Debug().Msg("Metrics endpoint called")
}

// MetricsMiddleware перехватывает запросы и записывает метрики
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseMetricsWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		latency := time.Since(start).Milliseconds()
		metricsCollector.RecordHTTPRequest(wrapped.statusCode, latency)

		log.Debug().
			Str("method", r.Method).
			Str("path", r.RequestURI).
			Int("status", wrapped.statusCode).
			Int64("latency_ms", latency).
			Msg("HTTP request")
	})
}

// responseMetricsWriter перехватывает status code
type responseMetricsWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseMetricsWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseMetricsWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

// HealthMetricsHandler возвращает подробное состояние здоровья
func HealthMetricsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := metricsCollector.GetMetrics()

	health := map[string]interface{}{
		"status":  "healthy",
		"uptime":  time.Now().Unix(),
		"metrics": metrics,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Simple JSON output
	fmt.Fprintf(w, `{
  "status": "healthy",
  "uptime": %d,
  "http_requests_total": %d,
  "http_errors_total": %d,
  "error_rate_percent": %.2f,
  "chats_created": %d,
  "chats_deleted": %d,
  "members_added": %d,
  "members_removed": %d,
  "events_published": %d,
  "latency_avg_ms": %d,
  "latency_p95_ms": %d
}`,
		metrics["uptime"],
		metrics["http_requests_total"],
		metrics["http_errors_total"],
		metrics["http_error_rate_percent"],
		metrics["chats_created_total"],
		metrics["chats_deleted_total"],
		metrics["members_added_total"],
		metrics["members_removed_total"],
		metrics["events_published_total"],
		metrics["http_request_latency_avg_ms"],
		metrics["http_request_latency_p95_ms"],
	)

	_ = health
}

// GetMetricsCollector возвращает глобальный коллектор
func GetMetricsCollector() *MetricsCollector {
	return metricsCollector
}
