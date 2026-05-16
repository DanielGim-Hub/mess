# Project: Realtime Gateway (Gleam)

## Goal
Implement a WebSocket realtime gateway according to specs in /docs.

## Architecture
- Event-driven service (Kafka)
- WebSocket server for clients
- Redis for ephemeral state (sessions, routing, presence)
- JWT authentication via JWKS

## Important constraints
- 1 active connection per user
- All Kafka events must be deduplicated by event_id
- WebSocket protocol must follow docs/websocket.md
- Redis keys must follow docs/redis.md
- Kafka payloads must follow docs/kafka-schema.md

## Modules structure
- domain/ → core types and logic
- contracts/ → JSON and Kafka payloads
- ws/ → websocket logic
- kafka/ → consumers and producers
- presence/ → online/offline logic
- receipts/ → delivery/read receipts
- routing/ → message delivery
- infra/ → redis, kafka, http, auth

## Suggested folder tree

```text
src/
  realtime_service.gleam        # entrypoint / app startup

  domain/
    auth.gleam                  # authenticated user, auth errors
    session.gleam               # connection/session domain types
    presence.gleam              # online/offline domain logic
    receipt.gleam               # delivered/read domain types
    routing.gleam               # routing decisions and delivery targets

  contracts/
    ws/
      ws_inbound.gleam          # client -> server websocket messages
      ws_outbound.gleam         # server -> client websocket messages
      ws_error.gleam            # websocket decode errors
    kafka/
      message_events.gleam      # message.created / updated / deleted payloads
      receipt_events.gleam      # receipt.delivered / receipt.read payloads
      presence_events.gleam     # presence online/offline payloads

  ws/
    handshake.gleam             # token extraction + auth check before upgrade
    connection.gleam            # one websocket connection process/state
    dispatcher.gleam            # routes inbound ws messages to features
    heartbeat.gleam             # ping/pong + idle timeout

  kafka/
    consumer.gleam              # kafka consumer startup
    dispatcher.gleam            # route consumed events by topic/type
    dedup.gleam                 # event_id deduplication
    producer.gleam              # publish receipt/presence events

  presence/
    service.gleam               # connect/disconnect presence workflow

  receipts/
    service.gleam               # ack -> receipt.delivered workflow

  routing/
    service.gleam               # deliver event to local or remote node

  infra/
    auth.gleam                  # jwt validation + jwks integration
    redis.gleam                 # redis access
    kafka.gleam                 # kafka client setup
    http.gleam                  # outbound http helpers
    config.gleam                # env/config loading
```

Notes:
- contracts/ describes message formats only
- domain/ contains pure types and rules, without Redis/Kafka/WebSocket details
- infra/ talks to external systems
- feature folders like ws/, presence/, receipts/, routing/ implement use cases
- start simple: if a feature is small, one file is enough; split only when logic grows

## Coding rules
- Do NOT mix domain logic with infrastructure
- Keep functions small and pure when possible
- Do NOT put everything in one file
- Prefer explicit types over raw strings
- Follow feature-based modular structure

## When implementing features
Always:
1. Read corresponding spec from /docs
2. Define types in contracts/ first
3. Then implement logic
4. Then connect infra

## Priority features (implementation order)
1. WebSocket handshake + JWT validation
2. Redis session registry
3. connected event + ping/pong
4. Kafka consumer skeleton + dedup
5. message.created → message.new
6. ack → receipt.delivered
7. presence online/offline
8. typing.start / typing.stop

## Language
- Always respond in Russian
