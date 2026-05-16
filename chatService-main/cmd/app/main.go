package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/messenger/chat-service/internal/config"
	"github.com/messenger/chat-service/internal/repository/postgres"
	"github.com/messenger/chat-service/internal/service"
	transport "github.com/messenger/chat-service/internal/transport/http"
	"github.com/messenger/chat-service/internal/worker"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// 2. Logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.Environment == "production" {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// 3. Database
	ctx := context.Background()
	dbPool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to connect to database")
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("Database ping failed")
	}

	// 4. Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Addr,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Warn().Err(err).Msg("Redis connection failed, continuing without caching")
		redisClient = nil
	} else {
		log.Info().Str("addr", cfg.Redis.Addr).Msg("Redis connected")
		defer redisClient.Close()
	}

	// 5. Repositories
	repo := postgres.NewRepository(dbPool)

	// 6. Service
	svc := service.NewService(repo, repo, repo, repo, repo)

	// 7. Workers
	outboxWorker := worker.NewOutboxWorkerWithTx(
		repo,
		repo,
		cfg.Kafka.Brokers,
		cfg.Kafka.TopicChatEvents,
	)

	msgConsumer := worker.NewMessageConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.TopicMessageCreated,
		cfg.Kafka.ConsumerGroup,
		repo,
		redisClient,
	)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		outboxWorker.Start(workerCtx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		msgConsumer.Start(workerCtx)
	}()

	// 8. HTTP Server
	r := chi.NewRouter()
	handler := transport.NewHandler(svc)
	handler.RegisterRoutes(r)

	// Apply idempotency middleware if Redis is available
	if redisClient != nil {
		idempotencyMW := transport.NewIdempotencyMiddleware(redisClient)
		r.Use(idempotencyMW.Handler)
		log.Info().Msg("Idempotency middleware enabled")
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// 9. Run
	go func() {
		log.Info().Msgf("Starting server on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info().Msg("Shutting down...")

	// Shutdown Server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	// Stop Workers
	workerCancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Workers stopped gracefully")
	case <-time.After(10 * time.Second):
		log.Warn().Msg("Workers shutdown timed out")
	}

	log.Info().Msg("Exited properly")
}
