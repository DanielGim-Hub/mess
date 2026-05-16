import contracts/ws/ws_outbound
import infra/redis
import kafka/dedup
import gleam/dynamic/decode
import gleam/json
import youid/uuid

/// Dispatch Kafka message to appropriate handler
pub fn dispatch(
  redis_client: redis.RedisClient,
  topic: String,
  key: String,
  value: String,
) -> Nil {
  case json.parse(value, event_id_decoder()) {
    Ok(event_id) -> {
      case dedup.is_processed(redis_client, event_id) {
        True -> Nil
        False -> {
          dedup.mark_processed(redis_client, event_id)
          handle_event(redis_client, topic, value)
        }
      }
    }
    _ -> handle_event(redis_client, topic, value)
  }
}

fn handle_event(
  redis_client: redis.RedisClient,
  topic: String,
  value: String,
) -> Nil {
  case topic {
    "message.created" -> handle_message_created(redis_client, value)
    "message.updated" -> handle_message_updated(redis_client, value)
    "message.deleted" -> handle_message_deleted(redis_client, value)
    "chat.events" -> handle_chat_event(redis_client, value)
    "presence.events" -> handle_presence_event(redis_client, value)
    "receipt.events" -> handle_receipt_event(redis_client, value)
    _ -> Nil
  }
}

fn handle_message_created(redis_client, value) {
  Nil
}

fn handle_message_updated(redis_client, value) {
  Nil
}

fn handle_message_deleted(redis_client, value) {
  Nil
}

fn handle_chat_event(redis_client, value) {
  Nil
}

fn handle_presence_event(redis_client, value) {
  Nil
}

fn handle_receipt_event(redis_client, value) {
  Nil
}

fn event_id_decoder() {
  use event_id <- decode.field("event_id", decode.string)
  decode.success(event_id)
}
