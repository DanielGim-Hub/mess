# Messenger Infrastructure

Полная инфраструктурная платформа для распределённой системы обмена мгновенными сообщениями (Мессенджер).

## Архитектура

Система включает три основных микросервиса:
- **Chat Service** (Go + PostgreSQL) — управление чатами и участниками
- **Message Service** (Python/Django + PostgreSQL) — хранение и обработка сообщений
- **Realtime Gateway** (Gleam/BEAM + Redis + Kafka) — WebSocket-доставка, presence и realtime-события

## Структура репозитория

```text
messenger-infra/
├── docker/                    # Dockerfile для каждого микросервиса
│   ├── chat-service/
│   ├── message-service/
│   └── realtime-gateway/
├── k8s/
│   ├── helm-charts/           # Helm чарты для 3+ микросервисов и зависимостей
│   │   ├── chat-service/
│   │   ├── message-service/
│   │   ├── realtime-gateway/
│   │   ├── kafka/
│   │   └── redis/
│   ├── namespaces/            # Базовые namespaces, SA, secrets
│   ├── argo-apps/             # ArgoCD App of Apps
│   ├── istio/                 # Service Mesh (mTLS, Circuit Breaker, retries)
│   ├── ingress/               # HAProxy + Keepalived
│   ├── valkey/                # Valkey (Redis-форк) для rate limiting
│   └── karpenter/             # Автомасштабирование нод
├── terraform/                 # IaC (namespaces, SA, RBAC)
├── ansible/
│   └── roles/strimzi-kafka/   # Ansible Role для Kafka (Strimzi)
├── observability/             # VictoriaMetrics, VictoriaLogs, Grafana, Jaeger, OTel
├── cicd/                      # GitLab Runner + GitLab CI пайплайны
├── tests/locust/              # Нагрузочное тестирование
├── docker-compose.yml         # Локальный запуск всех сервисов
└── deploy.sh                  # Единый скрипт деплоя
```

## Быстрый старт

### Локальная разработка (Docker Compose)

```bash
cd messenger-infra
docker compose up --build
```

Сервисы будут доступны:
- Chat Service: http://localhost:8080
- Message Service: http://localhost:8000
- Realtime Gateway: ws://localhost:8082
- Kafka: localhost:9092
- Valkey: localhost:6379
- MinIO: http://localhost:9000

### Kubernetes Deployment

```bash
# 1. Убедитесь, что кластер доступен
kubectl cluster-info

# 2. Запустите полный деплой
./deploy.sh all

# Или поэтапно:
./deploy.sh infra
./deploy.sh terraform
./deploy.sh kafka
./deploy.sh redis
./deploy.sh mesh
./deploy.sh rate-limit
./deploy.sh services
./deploy.sh observability
./deploy.sh argocd
```

## Блоки заданий

### Блок 1: Подготовка локальной инфраструктуры и Kubernetes

- **Talos Linux + Cilium (eBPF)**: Кластер готов к работе с Cilium CNI. NetworkPolicies обеспечивают L3/L4 изоляцию между неймспейсами.
- **Karpenter**: Манифест `k8s/karpenter/nodepool.yaml` настраивает автомасштабирование воркер-нод по CPU/memory триггерам.

### Блок 2: Управление конфигурацией (IaC + GitOps)

- **Terraform**: Создание namespaces, Service Accounts, RBAC (least privilege). Файлы в `terraform/`.
- **ArgoCD**: Паттерн App of Apps. Root Application `messenger-root` отслеживает директорию `k8s/argo-apps/apps/` и автоматически синхронизирует дочерние приложения.
- **Ansible Role для Kafka**: `ansible/roles/strimzi-kafka/` разворачивает Strimzi Operator, Kafka кластер (KRaft, 3 брокера), топики (`message.created`, `chat.events`, `receipt.events`, `presence.events`) и SASL/SCRAM пользователя.

### Блок 3: Ядро системы и трафик

- **Istio Service Mesh**:
  - `PeerAuthentication` с STRICT mTLS для сервисов `chat-service`, `message-service`, `realtime-gateway`.
  - `DestinationRule` с Circuit Breaker (5 consecutive 5xx, 30s interval, max 50% ejection) для `realtime-gateway -> message-service`.
  - `VirtualService` с retry-политикой (3 попытки, exponential backoff) для `chat-service -> message-service`.
- **Ingress + HAProxy + Keepalived**:
  - HAProxy балансирует внешний TCP (443/8443) на ноды кластера.
  - Keepalived (VRRP) обеспечивает VIP с миграцией < 2 сек при отказе.
- **Rate Limiting**:
  - Envoy Rate Limit Service с backend на Valkey (Redis).
  - Лимиты: 100 req/min per-user, 50 msg/min per-chat, 20 WS handshake/min per-IP.
  - `EnvoyFilter` для локального rate limiting на Istio IngressGateway.

### Блок 4: Observability

Выбран стек: **VictoriaMetrics + VictoriaLogs + Grafana + Jaeger + OpenTelemetry Collector**.

**Обоснование:**
- Единообразие архитектуры VM/VL
- Лучшая компрессия и производительность при высокой cardinality (user_id, chat_id)
- OpenTelemetry как единый стандарт для traces

**Компоненты:**
- **VictoriaMetrics Cluster**: vminsert, vmselect, vmstorage (2 реплики). VMAgent (DaemonSet) собирает метрики.
- **VictoriaLogs**: single-node с PVC для долгосрочного хранения.
- **Jaeger**: all-in-one для локальной среды.
- **OpenTelemetry Collector**: gateway-режим, принимает OTLP, маршрутизирует traces -> Jaeger, metrics -> VM, logs -> VL.
- **Grafana**: единая точка визуализации с дашбордами:
  - Messenger Golden Signals (Latency p50/p95/p99, RPS, Errors 5xx)
  - Kafka Overview (Consumer Lag)
  - Realtime Gateway (WS connections, presence events)
  - Rate Limiter (429 responses)

### Блок 5: CI/CD и окружение разработки

- **GitLab Runner**: 3 реплики в неймспейсе `gitlab-runners` с Kubernetes executor.
- **GitLab CI Pipeline** (`.gitlab-ci.yml`):
  - **Build**: Kaniko сборка образов для каждого микросервиса.
  - **Push**: Публикация в локальный registry.
  - **Update Helm Chart**: Автоматическое обновление image tag в values.yaml.
  - **ArgoCD Sync**: Триггер синхронизации root-приложения.

### Блок 6: Тестирование и валидация платформы

- **Locust REST**: 500 VU отправляют сообщения, создают чаты, скроллят историю.
- **Locust WebSocket**: 1000 VU держат persistent WS, отправляют `typing.start/stop`.
- **Locust Kafka**: Симуляция 2000 msg/s в топик `message.created`, проверка consumer lag.

Запуск тестов:
```bash
./deploy.sh test
```

## Переменные окружения

| Переменная | Описание | Значение по умолчанию |
|---|---|---|
| `DATABASE_URL` | PostgreSQL DSN | см. values.yaml |
| `KAFKA_BROKERS` | Брокеры Kafka | `kafka:9092` |
| `REDIS_ADDR` / `REDIS_URL` | Valkey/Redis | `valkey:6379` |
| `S3_ENDPOINT` | MinIO endpoint | `http://minio:9000` |
| `PORT` | HTTP/WS порт | `8080` / `8000` / `8082` |

## Зоны ответственности команды

- **Гимаев Даниэль Рустемович** — Chat Service, Terraform, ArgoCD, Helm чарты, финализация инфраструктурной документации
- **Набиуллин Камиль Маратович** — Message Service, Kubernetes/Talos/Cilium/Karpenter, Istio/HAProxy/Keepalived, Locust нагрузочное тестирование
- **Трофимов Александр Анатольевич** — Realtime Gateway, Ansible Role Strimzi, Observability (VM/VL/Grafana/Jaeger/OTel), Rate Limiting
