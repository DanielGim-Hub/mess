# ─── Realtime Gateway (Gleam/BEAM) ───────────────────────────────────────────

FROM ghcr.io/gleam-lang/gleam:v1.6.1-erlang-alpine AS builder

WORKDIR /build

# Установка rebar3 для сборки Erlang-зависимостей
RUN apk add --no-cache git curl build-base

# Копируем gleam.toml и manifest.toml
COPY gleam.toml manifest.toml ./
RUN gleam deps download

# Копируем исходники
COPY src ./src
COPY test ./test
COPY entrypoint.sh ./entrypoint.sh

# Сборка
RUN gleam build --target erlang

# ─── Final stage ─────────────────────────────────────────────────────────────
FROM erlang:27-alpine

WORKDIR /app

# Копируем собранный проект
COPY --from=builder /build/build/erlang-shipment /app
COPY --from=builder /build/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8082

# BEAM нода для распределённого Erlang (опционально)
ENV ERL_AFLAGS="-kernel shell_history enabled"

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD nc -z localhost 8082 || exit 1

ENTRYPOINT ["./entrypoint.sh"]
CMD ["start"]
