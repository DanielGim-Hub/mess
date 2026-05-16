# ─── Message Service (Python/Django) ─────────────────────────────────────────

FROM python:3.12-slim AS builder

WORKDIR /app

# Системные зависимости для psycopg
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libpq-dev && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# ─── Final stage ─────────────────────────────────────────────────────────────
FROM python:3.12-slim

WORKDIR /app

# Установка runtime-зависимостей
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq5 curl && rm -rf /var/lib/apt/lists/*

# Копируем установленные пакеты
COPY --from=builder /root/.local /root/.local
ENV PATH=/root/.local/bin:$PATH

# Копируем код
COPY . .

# Статика
RUN mkdir -p /app/static

EXPOSE 8000

# Healthcheck
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8000/health/live || exit 1

# Запуск через Gunicorn (WSGI)
CMD ["gunicorn", "--bind", "0.0.0.0:8000", "--workers", "4", "--threads", "2", "--access-logfile", "-", "--error-logfile", "-", "config.wsgi:application"]
