package config

import (
	"os"
)

type Config struct {
	Environment string
	Port        string
	Database    DatabaseConfig
	Kafka       KafkaConfig
	Redis       RedisConfig
	LogLevel    string
}

type DatabaseConfig struct {
	DSN string
}

type KafkaConfig struct {
	Brokers             []string
	TopicChatEvents     string
	TopicMessageCreated string
	ConsumerGroup       string
}

type RedisConfig struct {
	Addr string
}

func Load() (*Config, error) {
	return &Config{
		Environment: getEnv("ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		LogLevel:    getEnv("LOG_LEVEL", "debug"),
		Database: DatabaseConfig{
			DSN: getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/chats?sslmode=disable"),
		},
		Kafka: KafkaConfig{
			Brokers:             []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			TopicChatEvents:     getEnv("KAFKA_TOPIC_CHAT_EVENTS", "chat.events"),
			TopicMessageCreated: getEnv("KAFKA_TOPIC_MESSAGE_CREATED", "message.created"),
			ConsumerGroup:       getEnv("KAFKA_CONSUMER_GROUP", "chat-service.message.created"),
		},
		Redis: RedisConfig{
			Addr: getEnv("REDIS_ADDR", "localhost:6379"),
		},
	}, nil
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
