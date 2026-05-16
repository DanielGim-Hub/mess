# Распределённая система обмена мгновенными сообщениями (Мессенджер)

> **Версия:** 2.0  
> **Дата:** 2026-05-16  
> **Статус:** Реализовано (Production Ready)

---

## Содержание

1. [Общее описание](#1-общее-описание)
2. [Высокоуровневая архитектура](#2-высокоуровневая-архитектура)
3. [Микросервисы](#3-микросервисы)
   - 3.1 [Chat Service (Go)](#31-chat-service-go)
   - 3.2 [Message Service (Python/Django)](#32-message-service-pythondjango)
   - 3.3 [Realtime Gateway (Gleam/BEAM)](#33-realtime-gateway-gleambeam)
4. [API Reference](#4-api-reference)
5. [Базы данных](#5-базы-данных)
6. [Событийная модель (Kafka)](#6-событийная-модель-kafka)
7. [Инфраструктура](#7-инфраструктура)
8. [Observability](#8-observability)
9. [CI/CD](#9-cicd)
10. [Тестирование](#10-тестирование)
11. [Безопасность](#11-безопасность)
12. [Развёртывание](#12-развёртывание)

---

## 1. Общее описание

Система представляет собой **распределённый мессенджер**, построенный на микросервисной архитектуре. Основная цель — обеспечить масштабируемый обмен сообщениями в реальном времени с гарантией доставки, историей сообщений и управлением чатами.




### Ключевые характеристики

| Характеристика | Значение |
|---|---|
| **Архитектура** | Микросервисная (3 сервиса + инфраструктура) |
| **Протоколы** | HTTP REST, WebSocket, Kafka |
| **Базы данных** | PostgreSQL (2 инстанса), Redis/Valkey |
| **Очереди событий** | Apache Kafka (KRaft) |
| **Оркестрация** | Kubernetes (Istio Service Mesh) |
| **GitOps** | ArgoCD |
| **Observability** | VictoriaMetrics, VictoriaLogs, Jaeger, Grafana |
| **CI/CD** | GitLab CI + GitLab Runner |

---

## 2. Высокоуровневая архитектура

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Клиенты (Web/Mobile)                            │
│                         HTTP REST  │  WebSocket (WSS)                        │
└──────────────────┬─────────────────┼──────────────────┬──────────────────────┘
                   │                 │                  │
         ┌─────────▼─────────┐       │        ┌─────────▼──────────┐
         │  HAProxy +        │◄──────┘        │  Istio Ingress     │
         │  Keepalived (VRRP)│                │  Gateway           │
         └─────────┬─────────┘                └─────────┬──────────┘
                   │                                    │
         ┌─────────▼─────────┐              ┌─────────▼──────────┐
         │  Istio Ingress    │              │  Realtime Gateway  │
         │  Gateway (443)    │              │  (WebSocket, 8082) │
         └─────────┬─────────┘              └────────────────────┘
                   │                                    ▲
     ┌─────────────┼─────────────┐                      │ Kafka
     │             │             │           ┌──────────┴──────────┐
     ▼             ▼             ▼           │  Kafka (KRaft)      │
┌──────────┐ ┌──────────┐ ┌────────────┐    │  message.created    │
│  Chat    │ │ Message  │ │  Rate      │    │  chat.events        │
│ Service  │ │ Service  │ │  Limit     │    │  receipt.events     │
│ (8080)   │ │ (8000)   │ │  Service   │    │  presence.events    │
└────┬─────┘ └────┬─────┘ └────────────┘    └─────────────────────┘
     │            │
     ▼            ▼
┌──────────┐ ┌──────────┐
│PostgreSQL│ │PostgreSQL│
│(chats)   │ │(messages)│
└──────────┘ └──────────┘
```

### Взаимодействие сервисов

| От | К | Протокол | Назначение |
|---|---|---|---|
| Клиент | Chat Service | HTTP REST | CRUD чатов, участников, метаданных |
| Клиент | Message Service | HTTP REST | Отправка, редактирование, удаление сообщений |
| Клиент | Realtime Gateway | WebSocket | Получение событий в реальном времени |
| Chat Service | Kafka | Producer | `chat.events` (created/updated/deleted) |
| Message Service | Kafka | Producer | `message.created`, `message.updated`, `message.deleted` |
| Realtime Gateway | Kafka | Consumer | `message.created`, `chat.events`, `receipt.events` |
| Realtime Gateway | Kafka | Producer | `presence.events`, `receipt.events` |
| Chat Service | Message Service | HTTP Internal | Проверка членства (`/internal/chats/{id}/members/{uid}`) |
| Message Service | Chat Service | HTTP Internal | Снапшот чата, список чатов пользователя |

---

## 3. Микросервисы

### 3.1 Chat Service (Go)

**Стек:** Go 1.24.0, Chi Router, pgx/v5, segmentio/kafka-go, go-redis/v9, zerolog  
**Порт:** 8080  
**База данных:** PostgreSQL (схема `chats`)

#### Архитектура (Layered / Clean Architecture)

```
cmd/app/main.go          → DI, graceful shutdown, Redis, IdempotencyMiddleware
internal/config/         → Env-конфигурация
internal/domain/         → Сущности, интерфейсы, ошибки, события
internal/service/        → Бизнес-логика (chat, member, metadata)
internal/repository/     → PostgreSQL репозиторий + транзакции
internal/transport/http/ → Handlers, middleware, DTO
internal/worker/         → Outbox Worker (транзакционный), Message Consumer
```

#### Реализованные функции

- **CRUD чатов:** direct/group, soft-delete, title/avatar
- **Управление участниками:** добавление, удаление, смена ролей (owner/admin/member)
- **Права доступа:** owner может всё, admin — участников (кроме owner), member — только чтение
- **Direct-чаты:** ровно 2 участника, уникальность через `direct_chat_index`
- **Group-чаты:** до 1000 участников
- **Метаданные чатов:** JSONB key-value через `chat_metadata`
- **Idempotency:** Redis-backed middleware для POST/PUT/DELETE (TTL 24ч)
- **Outbox Pattern:** атомарная запись в БД + Kafka через транзакцию
- **Kafka Consumer:** обновление `last_message_at` по событию `message.created` с Redis dedup
- **Graceful Shutdown:** `sync.WaitGroup` для workers, таймаут 10 сек

#### Доменные сущности

| Сущность | Описание |
|---|---|
| `Chat` | Чат (direct/group), UUID PK, soft-delete (`deleted_at`) |
| `ChatMember` | Участник чата с ролью (owner/admin/member), soft-remove (`left_at`) |
| `ChatMetadata` | Произвольные JSONB-данные по ключу `(chat_id, key)` |
| `OutboxEvent` | Событие для Kafka с retry-логикой (до 5 попыток) |

#### Безопасность

- Все публичные endpoints проверяют членство запрашивающего в чате (`IsChatMember`)
- Internal API защищён `X-Service-Name` + `Authorization: Bearer <token>`
- `InternalGetMember` возвращает `InternalChatMembershipDTO` с `IsMember`, `Role`, `ChatType`

---

### 3.2 Message Service (Python/Django)

**Стек:** Django 5.2.12, Django REST Framework 3.17.0, drf-spectacular, kafka-python-ng, psycopg  
**Порт:** 8000  
**База данных:** PostgreSQL (схема `message_service`)

#### Архитектура

```
config/              → Django project settings (env-based SECRET_KEY, DEBUG)
message_api/         → Основной app
├── models.py        → Message, MessageVersion, Receipt, OutboxEvent, ChatSequenceCounter
├── serializers.py   → DRF сериализаторы (Send, Edit, List, Receipt)
├── services.py      → Service Layer (бизнес-логика)
├── views.py         → DRF views (REST API + Receipts)
├── urls.py          → URL routing
└── management/      → Outbox worker, Archive command
```

#### Реализованные функции

- **Сообщения:** отправка, редактирование (с версионированием), soft-delete
- **Идемпотентность:** `Idempotency-Key` заголовок, unique constraint в БД
- **Sequence numbers:** атомарный счётчик `ChatSequenceCounter` (`select_for_update`)
- **Receipts API:** `POST /receipts` (delivered/read) с защитой от downgrade
- **List Messages:** `GET /chats/{id}/messages` с limit/offset пагинацией
- **Outbox Worker:**
  - `FOR UPDATE SKIP LOCKED` для конкурентных воркеров
  - Graceful shutdown по SIGTERM/SIGINT
  - Retry logic (max 5), failed статус
  - Подтверждение записи в Kafka (`future.get(timeout=10)`)
- **Архивация:** `COPY TO` → gzip → S3/MinIO → `DETACH PARTITION` → `DROP TABLE`
- **Security:** `SECRET_KEY`, `DEBUG`, `ALLOWED_HOSTS` из переменных окружения

#### Модели данных

| Модель | Описание |
|---|---|
| `ChatSequenceCounter` | Атомарный счётчик `sequence_number` в чате |
| `Message` | UUID, chat_id, sender_id, content_type, text/attachment, sequence_number, status |
| `MessageVersion` | История редактирований (version, text) |
| `Receipt` | Статус доставки/прочтения `(message_id, user_id)` |
| `OutboxEvent` | Outbox-запись для Kafka с retry_count |

---

### 3.3 Realtime Gateway (Gleam/BEAM)

**Стек:** Gleam (компилируется в Erlang/OTP), Mist (HTTP/WebSocket), ywt (JWT)  
**Порт:** 8082  
**Статус:** Реализовано (полный цикл WS + Redis + Kafka)

#### Архитектура

```
src/
├── realtime_service.gleam     → Точка входа, инициализация Redis/Kafka
├── infra/
│   ├── config.gleam           → Env-конфигурация
│   ├── auth.gleam             → JWT RS256 валидация через JWKS
│   ├── redis.gleam            → Redis client (sessions, presence, typing, dedup)
│   └── ratelimit.gleam        → Token bucket rate limiting
├── kafka/
│   ├── client.gleam           → Kafka producer/consumer FFI
│   ├── consumer.gleam         → Consumer loop с обработкой
│   ├── dispatcher.gleam       → Маршрутизация событий по типу
│   └── dedup.gleam            → Дедупликация через Redis (24h TTL)
├── domain/
│   ├── session.gleam          → Типы сессий
│   └── presence.gleam         → Типы presence
├── presence/
│   └── service.gleam          → Online/offline + Redis
├── typing/
│   └── service.gleam          → Typing indicators + Redis TTL
├── receipts/
│   └── service.gleam          → Ack → receipt.delivered в Kafka
├── routing/
│   └── service.gleam          → Маршрутизация событий до пользователей
└── ws/
    ├── connection.gleam       → WS lifecycle (init, message, close)
    ├── controller.gleam       → Handshake + DI
    ├── router.gleam           → HTTP routing
    └── token.gleam            → Извлечение токена
```

#### Реализованные функции

- **WebSocket handshake:** JWT RS256 валидация, автоотключение по expiry
- **Session registry:** `gw:session:{user_id}` → `{node_id}:{connection_id}` (TTL 5 мин)
- **Presence:** `gw:presence:{user_id}` → "online" (TTL 65с), события в Kafka
- **Typing indicators:** `gw:typing:{chat_id}:{user_id}` (TTL 5с), broadcast `typing.started/stopped`
- **Ack → Receipts:** `ack` от клиента → публикация `receipt.delivered` в Kafka
- **Rate limiting:** 60 сообщений/мин, 1 typing/3сек, 20 handshake/мин
- **Duplicate connections:** при новом подключении старая сессия перезаписывается
- **Kafka Consumer:** фоновый процесс для `message.created`, `chat.events`, `receipt.events`, `presence.events`
- **Kafka Producer:** публикация `presence.online/offline`, `receipt.delivered`
- **Redis Pub/Sub:** `gw:node:{node_id}` для межузловой маршрутизации

#### WebSocket-протокол

Все сообщения имеют единый envelope:
```json
{ "type": "...", "id": "uuid", "payload": { } }
```

**Входящие:** `ack`, `typing.start`, `typing.stop`, `ping`  
**Исходящие:** `connected`, `message.new`, `message.updated`, `message.deleted`, `receipt.updated`, `presence.updated`, `chat.updated`, `typing.started`, `typing.stopped`, `error`, `pong`

---

## 4. API Reference

### 4.1 Chat Service

#### Public API (`X-User-Id: <uuid>`)

| Метод | Путь | Описание | Коды |
|---|---|---|---|
| `POST` | `/api/v1/chats` | Создать чат (direct/group) | 201, 200 (duplicate direct), 409, 422 |
| `GET` | `/api/v1/chats` | Список чатов (limit/offset) | 200 |
| `GET` | `/api/v1/chats/{id}` | Получить чат | 200, 404 |
| `PATCH` | `/api/v1/chats/{id}` | Обновить чат (title, avatar_url) | 200, 403, 404 |
| `DELETE` | `/api/v1/chats/{id}` | Soft-delete чата | 204, 403, 404 |
| `GET` | `/api/v1/chats/{id}/members` | Список участников (limit/offset) | 200, 404 |
| `POST` | `/api/v1/chats/{id}/members` | Добавить участника | 201, 403, 422 |
| `GET` | `/api/v1/chats/{id}/members/{uid}` | Получить участника | 200, 404 |
| `DELETE` | `/api/v1/chats/{id}/members/{uid}` | Удалить участника | 204, 403, 422 |
| `PATCH` | `/api/v1/chats/{id}/members/{uid}` | Изменить роль | 200, 403, 422 |
| `GET` | `/api/v1/chats/{id}/metadata/{key}` | Получить метаданные | 200, 404 |
| `PUT` | `/api/v1/chats/{id}/metadata/{key}` | Установить метаданные | 200, 403 |
| `DELETE` | `/api/v1/chats/{id}/metadata/{key}` | Удалить метаданные | 204, 403, 404 |

#### Internal API (`X-Service-Name` + `Authorization: Bearer <token>`)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/api/v1/internal/chats/{id}/members/{uid}` | Проверка членства |
| `GET` | `/api/v1/internal/chats/{id}/snapshot` | Снапшот чата + участники |
| `GET` | `/api/v1/internal/users/{uid}/chats` | Чаты пользователя |

#### Health / Metrics

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/health/live` | Liveness |
| `GET` | `/health/ready` | Readiness |
| `GET` | `/metrics` | Prometheus text |

### 4.2 Message Service

**Базовый путь:** `/api/v1/`

| Метод | Путь | Описание | Коды |
|---|---|---|---|
| `POST` | `/chats/{chat_id}/messages` | Отправить сообщение | 201, 409 (idempotency) |
| `GET` | `/chats/{chat_id}/messages` | Список сообщений | 200 |
| `GET` | `/chats/{chat_id}/messages/{id}` | Получить сообщение | 200, 404 |
| `PATCH` | `/chats/{chat_id}/messages/{id}` | Редактировать текст | 200, 403, 422 |
| `DELETE` | `/chats/{chat_id}/messages/{id}` | Soft-delete | 204, 403 |
| `POST` | `/chats/{chat_id}/messages/{id}/receipts` | Создать/обновить receipt | 201, 404 |
| `GET` | `/chats/{chat_id}/messages/{id}/receipts` | Получить receipts | 200 |
| `GET` | `/health/live` | Health check | 200 |

**Документация OpenAPI:** `/api/schema/`, Swagger UI: `/api/docs/`

### 4.3 Realtime Gateway

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/ws?token=<jwt>` | WebSocket upgrade |

---

## 5. Базы данных

### 5.1 Chat Service (PostgreSQL)

```sql
-- Таблицы
chats              -- UUID PK, type, title, avatar_url, created_by, last_message_at, deleted_at
chat_members       -- id, chat_id FK, user_id, role, invited_by, joined_at, left_at
chat_metadata      -- chat_id PK, key PK, value JSONB, updated_at
direct_chat_index  -- user_id_a, user_id_b, chat_id
outbox_events      -- id, event_id, event_type, topic, partition_key, payload JSONB, status, retry_count

-- Constraints
chats_title_required_for_group / chats_title_null_for_direct
chat_members_unique_active
idx_chat_members_one_owner  -- один owner на чат
outbox_max_retries          -- retry_count <= 10

-- Views
active_chats        -- deleted_at IS NULL
active_chat_members -- left_at IS NULL
```

### 5.2 Message Service (PostgreSQL)

```sql
-- Таблицы
chat_sequence_counters  -- chat_id PK, last_sequence
messages                -- UUID PK, chat_id, sender_id, content_type, text, attachment JSONB, 
                        -- reply_to_id, status, sequence_number, idempotency_key, is_edited, edited_at
message_versions        -- UUID PK, message_id, chat_id, version, text
receipts                -- BigAuto PK, message_id, user_id, chat_id, status
outbox_events           -- UUID PK, event_id, event_type, topic, partition_key, payload, status, retry_count

-- Indexes
msg_chat_seq_idx            -- (chat_id, sequence_number)
msg_chat_sender_created_idx -- (chat_id, sender_id, created_at)
msg_chat_created_idx        -- (chat_id, created_at)
outbox_status_created_idx   -- (status, created_at)

-- Constraints
UNIQUE (chat_id, sequence_number)
UNIQUE (message_id, version)
UNIQUE (message_id, user_id)  -- receipts
```

### 5.3 Redis (Valkey)

| Ключ | Тип | TTL | Назначение |
|---|---|---|---|
| `gw:session:{user_id}` | Hash | 300с | Маршрутизация: node_id + connection_id |
| `gw:conn:{connection_id}` | Hash | 300с | Обратный lookup |
| `gw:presence:{user_id}` | String | 65с | Статус online/offline |
| `gw:typing:{chat_id}:{user_id}` | String | 5с | Дедупликация typing |
| `gw:dedup:{event_id}` | String | 86400с | Дедупликация Kafka (24ч) |
| `gw:node:{node_id}` | Pub/Sub | — | Межузловая маршрутизация |
| `chat:dedup:{key}` | String | 86400с | Idempotency REST API (24ч) |
| `chat:consumer:dedup:{event_id}` | String | 86400с | Дедупликация consumer (24ч) |
| `gw:ratelimit:msg:{user_id}` | String | 60с | Rate limit: 60 msg/min |
| `gw:ratelimit:typing:{chat_id}:{user_id}` | String | 3с | Rate limit: 1 typing/3сек |
| `gw:ratelimit:handshake:{ip}` | String | 60с | Rate limit: 20 handshake/min |

---

## 6. Событийная модель (Kafka)

### Топики

| Топик | Partitions | Replicas | Producer | Consumer | Назначение |
|---|---|---|---|---|---|
| `message.created` | 6 | 3 | Message Service | Chat Service, Realtime Gateway | Новое сообщение |
| `message.updated` | 6 | 3 | Message Service | Realtime Gateway | Редактирование |
| `message.deleted` | 6 | 3 | Message Service | Realtime Gateway | Удаление |
| `chat.events` | 6 | 3 | Chat Service | Message Service, Realtime Gateway | CRUD чатов |
| `receipt.events` | 6 | 3 | Realtime Gateway | — | Статусы доставки |
| `presence.events` | 6 | 3 | Realtime Gateway | — | Online/offline |

### Схема событий (Envelope)

```json
{
  "event_id": "uuid",
  "event_type": "message.created | message.updated | message.deleted | chat.created | chat.updated | presence.updated | receipt.delivered",
  "topic": "message.created",
  "partition_key": "<chat_id | user_id>",
  "payload": { ... },
  "metadata": {
    "source_service": "chat-service | message-service | realtime-gateway",
    "payload_version": "1.0",
    "occurred_at": "2026-01-01T00:00:00Z"
  }
}
```

---

## 7. Инфраструктура

### 7.1 Kubernetes

**Namespaces:**
- `chat-service`, `message-service`, `realtime-gateway` (istio-injection: enabled)
- `kafka`, `redis`, `observability`, `argocd`, `istio-system`, `gitlab-runners`

**Helm Charts:**
- `chat-service` — 2 реплики, HPA 2–10, PDB, NetworkPolicy (ingress + egress)
- `message-service` — 3 реплики, HPA 3–15, Outbox Worker (StatefulSet), Archive CronJob
- `realtime-gateway` — 2 реплики, HPA 2–20, preStop hook (sleep 10)
- `kafka` — Bitnami Kafka 31.x, KRaft, 3 реплики, SASL/SCRAM
- `redis` — Bitnami Valkey 2.x, standalone

**Istio Service Mesh:**
- STRICT mTLS для сервисных namespace
- Circuit Breaker (`realtime-gateway` → `message-service`)
- Retry: 3 попытки с exponential backoff
- Rate Limiting: Envoy Rate Limit Service + Valkey backend
- AuthorizationPolicy: default-deny + explicit allow для каждого сервиса

**Ingress:**
- HAProxy (2 реплики, StatefulSet с anti-affinity) + Keepalived (VRRP VIP)
- Istio IngressGateway: 443 (HTTPS), 8443 (WSS), TLS SIMPLE

### 7.2 Terraform

- Создание 9 namespace
- ServiceAccount + RBAC (least privilege)
- External Secrets Operator (v0.14.0)

### 7.3 Ansible (Strimzi Kafka)

- Strimzi Operator 0.45.0
- Kafka 3.8.0 **KRaft mode** (без ZooKeeper)
- 4 топика с 6 партициями и 3 репликами
- SASL/SCRAM пользователь `messenger-app` с ACL
- Kafka Metrics ConfigMap для JMX Prometheus Exporter

### 7.4 Cilium (eBPF)

- CiliumNetworkPolicy с L7 фильтрацией
- HTTP методы и paths ограничены для каждого сервиса
- Hubble для наблюдаемости L3/L4

---

## 8. Observability

### Метрики — VictoriaMetrics (кластер)

| Компонент | Тип | Реплики | Назначение |
|---|---|---|---|
| VMAgent | DaemonSet | — | Сбор метрик с подов |
| vminsert | Deployment | 2 | Приём данных |
| vmselect | Deployment | 2 | Query API (порт 8481) |
| vmstorage | StatefulSet | 2 | Хранилище 50Gi |

### Логи — VictoriaLogs

- StatefulSet, 1 реплика, 50Gi PVC, порт 9428
- Fluent Bit → OTLP HTTP 4318 → OpenTelemetry Collector → VictoriaLogs

### Трейсы — Jaeger

- Deployment all-in-one 1.60.0
- OTLP gRPC 4317, HTTP 4318, UI 16686

### OpenTelemetry Collector

- 2 реплики, Contrib 0.111.0
- Pipelines: traces → otlp/jaeger, metrics → vminsert, logs → VictoriaLogs

### Grafana

- Deployment 11.3.0, admin/admin
- Data sources: VictoriaMetrics, VictoriaLogs, Jaeger
- Provisioned dashboards (schemaVersion 36):
  1. **Messenger Golden Signals** — latency p95, RPS, errors 5xx, WS connections, saturation
  2. **Kafka Overview** — consumer lag, partition lag, throughput
  3. **Rate Limiter** — 429 responses, bucket fill

---

## 9. CI/CD

### GitLab CI Pipeline (`.gitlab-ci.yml`)

| Stage | Описание |
|---|---|
| **build** | Kaniko-сборка каждого сервиса с кэшем |
| **push** | Встроено в Kaniko (registry `localhost:5000`) |
| **update-helm** | Автозамена тегов в `values.yaml` |
| **sync-argocd** | `kubectl patch app messenger-root` |

### GitLab Runner

- Deployment: 3 реплики, Kubernetes executor
- Namespace: `gitlab-runners`

---

## 10. Тестирование

### Unit-тесты

- **Chat Service:** компилируется, проходит `go vet` (тесты в процессе добавления)
- **Message Service:** тесты маршрутизации URL
- **Realtime Gateway:** 11 юнит-тестов на JWT и WS-декодирование

### Нагрузочное тестирование (Locust)

```bash
cd messenger-infra/tests/locust
locust -f locustfile_rest.py       # 500 VU, REST API
locust -f locustfile_websocket.py  # 1000 VU, WebSocket
locust -f locustfile_kafka.py      # 2000 msg/s, Kafka producer
```

| Сценарий | Нагрузка | Описание |
|---|---|---|
| REST | 500 VU | create_chat (w:3), send_message (w:5), list_messages (w:4), edit_message (w:2), delete_message (w:1), create_receipt (w:2), list_chats (w:2), get_chat (w:1) |
| WebSocket | 1000 VU | typing_start (w:3), typing_stop (w:2), ack (w:2), ping (w:1) |
| Kafka | 2000 msg/s | produce_message_created (w:10), produce_chat_event (w:3), produce_presence_event (w:2), produce_receipt_event (w:2) |

---

## 11. Безопасность

### Аутентификация

- **Публичные API:** `X-User-Id` заголовок (доверие к upstream gateway)
- **Internal API:** `X-Service-Name` + `Authorization: Bearer <token>`
- **Realtime Gateway:** JWT RS256 через JWKS endpoint

### Авторизация

- RBAC в Chat Service (owner/admin/member)
- Istio AuthorizationPolicy + STRICT mTLS
- NetworkPolicies: egress только на необходимые сервисы, ingress от разрешённых источников
- CiliumNetworkPolicy: L7 фильтрация HTTP методов и paths

### Rate Limiting

- Envoy Rate Limit Service (Valkey backend):
  - Per-user: 100 запросов/мин на REST API
  - Per-chat: 50 сообщений/мин
  - Per-IP: 20 WebSocket handshake/мин
- WS rate limiting (Realtime Gateway):
  - 60 сообщений/мин per user
  - 1 typing.start/3сек per chat

### TLS

- cert-manager: selfsigned-issuer для локальной среды
- `messenger-tls-secret` для Istio IngressGateway
- STRICT mTLS между сервисами

---

## 12. Развёртывание

### 12.1 Локальная разработка (Docker Compose)

```bash
cd messenger-infra
make build-services
make up
```

Проверка:
```bash
curl http://localhost:8080/health/live
curl http://localhost:8000/api/v1/health/live
```

### 12.2 Kubernetes (полный деплой)

```bash
cd messenger-infra
make deploy-k8s   # или ./deploy.sh all
```

Поэтапно:
```bash
./deploy.sh infra          # Terraform: namespace, SA, RBAC, External Secrets
./deploy.sh kafka          # Ansible: Strimzi + Kafka + topics (KRaft)
./deploy.sh redis          # Helm: Valkey
./deploy.sh mesh           # Istio: mTLS, Circuit Breaker, IngressGateway, AuthorizationPolicy
./deploy.sh rate-limit     # Envoy Rate Limit
./deploy.sh ingress        # HAProxy + Keepalived (StatefulSet)
./deploy.sh services       # Helm: chat-service, message-service, realtime-gateway
./deploy.sh observability  # VictoriaMetrics, VictoriaLogs, Jaeger, Grafana
./deploy.sh argocd         # ArgoCD App of Apps
```

### 12.3 Проверка деплоя

```bash
kubectl get pods -n chat-service
kubectl get pods -n message-service
kubectl get pods -n realtime-gateway
kubectl get pods -n observability

# Проброс портов Grafana
kubectl port-forward svc/grafana 3000:3000 -n observability
# Открыть http://localhost:3000 (admin/admin)
```

---

## Команда

| Участник | Зона ответственности |
|---|---|
| **Гимаев Даниэль Рустемович** | Chat Service (Go), Terraform, ArgoCD, Helm чарты, финальная документация |
| **Набиуллин Камиль Маратович** | Message Service (Python/Django), Kubernetes/Talos/Cilium/Karpenter, Istio/HAProxy/Keepalived, нагрузочное тестирование |
| **Трофимов Александр Анатольевич** | Realtime Gateway (Gleam), Ansible Role Strimzi, Observability (VM/VL/Grafana/Jaeger), Rate Limiting |
