# ─── Chat Service (Go) ───────────────────────────────────────────────────────
# Multi-stage build для минимального образа

FROM golang:1.23-alpine AS builder

WORKDIR /build

# Установка зависимостей
RUN apk add --no-cache git ca-certificates tzdata

# Копируем модули
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Собираем статический бинарник
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /bin/chat-service \
    ./cmd/app/main.go

# ─── Final stage ─────────────────────────────────────────────────────────────
FROM scratch

# Копируем CA certificates и timezone data из builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Копируем бинарник
COPY --from=builder /bin/chat-service /bin/chat-service

# Миграции (для init-containers или запуска)
COPY --from=builder /build/migrations /migrations

EXPOSE 8080

# Некорневой пользователь (scratch не поддерживает useradd, используем numeric)
USER 65534:65534

ENTRYPOINT ["/bin/chat-service"]
