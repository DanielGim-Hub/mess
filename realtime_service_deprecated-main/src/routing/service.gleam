import contracts/ws/ws_outbound.{type OutboundMessage}
import infra/redis
import gleam/string

/// Broadcast message to a user via their node
pub fn route_to_user(
  redis_client: redis.RedisClient,
  user_id: String,
  message: OutboundMessage,
) -> Nil {
  case redis.get(redis_client, "gw:session:" <> user_id) {
    Ok(node_and_conn) -> {
      let parts = string.split(node_and_conn, ":")
      case parts {
        [node_id, ..] -> {
          let channel = "gw:node:" <> node_id
          let _ = redis.publish(redis_client, channel, encode_message(message))
          Nil
        }
        _ -> Nil
      }
    }
    _ -> Nil
  }
}

/// Broadcast message to all users in a chat
pub fn broadcast_to_chat(
  redis_client: redis.RedisClient,
  chat_id: String,
  sender_id: String,
  message: OutboundMessage,
) -> Nil {
  let channel = "gw:chat:" <> chat_id
  let _ = redis.publish(redis_client, channel, encode_message(message))
  Nil
}

fn encode_message(message: OutboundMessage) -> String {
  contracts/ws/ws_outbound.encode(message)
}
