import contracts/ws/ws_error.{
  InvalidEnvelope, InvalidPayload, UnknownMessageType,
}
import contracts/ws/ws_inbound.{
  AckMessage, PingMessage, TypingStartMessage, TypingStopMessage,
  decode as decode_inbound,
}
import contracts/ws/ws_outbound.{
  type OutboundMessage, Connected, ErrorMessage, InvalidPayloadError, Pong,
  RateLimitedError, TypingStarted, TypingStopped, UnknownTypeError,
  encode as encode_outbound,
}
import dream/servers/mist/websocket
import gleam/erlang/process
import gleam/option.{type Option, None, Some}
import gleam/result
import gleam/time/duration
import gleam/time/timestamp
import infra/auth.{type AuthenticatedAccessToken}
import infra/ratelimit
import infra/redis
import kafka/client as kafka
import presence/service as presence
import receipts/service as receipts
import routing/service as routing
import tempo/datetime
import typing/service as typing
import youid/uuid

pub type Dependencies {
  Dependencies(
    access_token: AuthenticatedAccessToken,
    redis: Option(redis.RedisClient),
    kafka: Option(kafka.KafkaProducer),
    node_id: String,
  )
}

pub type ConnectionMessage {
  ExpireToken
  Broadcast(OutboundMessage)
}

pub type ConnectionState {
  ConnectionState(
    expiry_timer: process.Timer,
    user_id: uuid.Uuid,
    connection_id: String,
    redis: Option(redis.RedisClient),
    kafka: Option(kafka.KafkaProducer),
    node_id: String,
  )
}

pub fn on_init(
  connection: websocket.Connection,
  dependencies: Dependencies,
) -> #(ConnectionState, Option(process.Selector(ConnectionMessage))) {
  let Dependencies(access_token, redis_client, kafka_producer, node_id) =
    dependencies
  let connection_id = "conn_" <> uuid.v7_string()
  let expiry_subject = process.new_subject()
  let expiry_timer =
    process.send_after(
      expiry_subject,
      milliseconds_until_expiry(access_token.expires_at),
      ExpireToken,
    )

  let state =
    ConnectionState(
      expiry_timer: expiry_timer,
      user_id: access_token.user_id,
      connection_id: connection_id,
      redis: redis_client,
      kafka: kafka_producer,
      node_id: node_id,
    )

  // Check for duplicate connections and close old one
  case redis_client {
    Some(client) -> {
      let user_id_str = uuid.to_string(access_token.user_id)
      // Close existing connection if any
      let _ = redis.get(client, "gw:session:" <> user_id_str)
      // Set new session
      let _ = redis.setex(client, "gw:session:" <> user_id_str, node_id <> ":" <> connection_id, 300)
      let _ = presence.set_online(client, user_id_str, node_id)
      Nil
    }
    None -> Nil
  }

  // Publish presence.online to Kafka
  case kafka_producer {
    Some(producer) -> {
      let payload =
        "{\"user_id\":\"" <> uuid.to_string(access_token.user_id) <> "\",\"status\":\"online\",\"node_id\":\"" <> node_id <> "\"}"
      let _ = kafka.produce(producer, "presence.events", uuid.to_string(access_token.user_id), payload)
      Nil
    }
    None -> Nil
  }

  send_message(
    connection,
    connected_message(connection_id, access_token.user_id),
  )

  #(state, Some(process.new_selector() |> process.select(expiry_subject)))
}

pub fn on_message(
  state: ConnectionState,
  message: websocket.Message(ConnectionMessage),
  connection: websocket.Connection,
  _dependencies: Dependencies,
) -> websocket.Action(ConnectionState, ConnectionMessage) {
  case message {
    websocket.TextMessage(payload) ->
      handle_text_message(state, payload, connection)
    websocket.CustomMessage(ExpireToken) -> websocket.stop_connection()
    websocket.CustomMessage(Broadcast(msg)) -> {
      send_message(connection, msg)
      websocket.continue_connection(state)
    }
    websocket.ConnectionClosed -> websocket.stop_connection()
    websocket.BinaryMessage(_) -> websocket.continue_connection(state)
  }
}

fn handle_text_message(
  state: ConnectionState,
  payload: String,
  connection: websocket.Connection,
) -> websocket.Action(ConnectionState, ConnectionMessage) {
  case decode_inbound(payload) {
    Ok(PingMessage(id:, ..)) -> {
      case state.redis {
        Some(client) -> {
          let user_id_str = uuid.to_string(state.user_id)
          let _ = redis.setex(client, "gw:session:" <> user_id_str, state.node_id <> ":" <> state.connection_id, 300)
          let _ = presence.set_online(client, user_id_str, state.node_id)
          Nil
        }
        None -> Nil
      }
      send_message(connection, Pong(id: id, server_time: current_time()))
      websocket.continue_connection(state)
    }

    Ok(AckMessage(id:, event_id:)) -> {
      case state.redis, state.kafka {
        Some(redis_client), Some(kafka_producer) -> {
          receipts.process_ack(redis_client, kafka_producer, uuid.to_string(state.user_id), event_id)
          Nil
        }
        _, _ -> Nil
      }
      websocket.continue_connection(state)
    }

    Ok(TypingStartMessage(id:, chat_id:)) -> {
      let user_id_str = uuid.to_string(state.user_id)
      let chat_id_str = uuid.to_string(chat_id)

      // Rate limit check
      case state.redis {
        Some(client) -> {
          case ratelimit.check_typing_rate(client, chat_id_str, user_id_str) {
            False -> {
              send_message(connection, rate_limited_error())
              return websocket.continue_connection(state)
            }
            True -> Nil
          }
        }
        None -> Nil
      }

      case state.redis {
        Some(client) -> {
          typing.start_typing(client, chat_id_str, user_id_str)
          Nil
        }
        None -> Nil
      }

      send_message(
        connection,
        TypingStarted(
          id: "ws_" <> uuid.v7_string(),
          chat_id: chat_id,
          user_id: state.user_id,
          started_at: current_time(),
        ),
      )
      websocket.continue_connection(state)
    }

    Ok(TypingStopMessage(id:, chat_id:)) -> {
      let user_id_str = uuid.to_string(state.user_id)
      let chat_id_str = uuid.to_string(chat_id)

      case state.redis {
        Some(client) -> {
          typing.stop_typing(client, chat_id_str, user_id_str)
          Nil
        }
        None -> Nil
      }

      send_message(
        connection,
        TypingStopped(
          id: "ws_" <> uuid.v7_string(),
          chat_id: chat_id,
          user_id: state.user_id,
        ),
      )
      websocket.continue_connection(state)
    }

    Error(error) -> {
      send_message(connection, decode_error_to_outbound(error))
      websocket.continue_connection(state)
    }
  }
}

pub fn on_close(state: ConnectionState, _dependencies: Dependencies) -> Nil {
  let ConnectionState(expiry_timer:, user_id:, redis:, kafka:, node_id:, ..) =
    state
  let _ = process.cancel_timer(expiry_timer)

  case redis {
    Some(client) -> {
      let user_id_str = uuid.to_string(user_id)
      let _ = presence.set_offline(client, user_id_str)
      let _ = redis.del(client, "gw:conn:" <> state.connection_id)
      Nil
    }
    None -> Nil
  }

  case kafka {
    Some(producer) -> {
      let payload =
        "{\"user_id\":\"" <> uuid.to_string(user_id) <> "\",\"status\":\"offline\",\"node_id\":\"" <> node_id <> "\"}"
      let _ = kafka.produce(producer, "presence.events", uuid.to_string(user_id), payload)
      Nil
    }
    None -> Nil
  }

  Nil
}

fn send_message(
  connection: websocket.Connection,
  message: OutboundMessage,
) -> Nil {
  let _ =
    message
    |> encode_outbound
    |> websocket.send_text(connection, _)
    |> result.replace_error(Nil)

  Nil
}

fn connected_message(
  connection_id: String,
  user_id: uuid.Uuid,
) -> OutboundMessage {
  Connected(
    id: "ws_" <> uuid.v7_string(),
    connection_id: connection_id,
    user_id: user_id,
    server_time: current_time(),
  )
}

fn rate_limited_error() -> OutboundMessage {
  ErrorMessage(
    id: "ws_" <> uuid.v7_string(),
    ref_id: None,
    code: RateLimitedError,
    message: "Rate limit exceeded. Try again later.",
  )
}

fn decode_error_to_outbound(error) -> OutboundMessage {
  case error {
    InvalidEnvelope(_) | InvalidPayload(..) ->
      ErrorMessage(
        id: "ws_" <> uuid.v7_string(),
        ref_id: None,
        code: InvalidPayloadError,
        message: "Invalid WebSocket message payload",
      )

    UnknownMessageType(_) ->
      ErrorMessage(
        id: "ws_" <> uuid.v7_string(),
        ref_id: None,
        code: UnknownTypeError,
        message: "Unknown WebSocket message type",
      )
  }
}

fn current_time() {
  timestamp.system_time()
  |> datetime.from_timestamp
}

fn milliseconds_until_expiry(expires_at: timestamp.Timestamp) -> Int {
  let now = timestamp.system_time()
  let diff =
    timestamp.difference(now, expires_at)
    |> duration.to_milliseconds

  case diff > 0 {
    True -> diff
    False -> 0
  }
}
