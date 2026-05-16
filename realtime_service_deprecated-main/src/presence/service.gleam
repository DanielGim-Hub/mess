import domain/presence.{type PresenceEvent, Online, Offline, presence_key}
import infra/redis
import youid/uuid

/// Publish presence.online and store in Redis
pub fn set_online(
  redis_client: redis.RedisClient,
  user_id: String,
  node_id: String,
) -> Nil {
  let _ = redis.setex(redis_client, presence_key(user_id), "online", 65)
  let _ = redis.setex(redis_client, "gw:session:" <> user_id, node_id, 300)
  Nil
}

/// Publish presence.offline and remove from Redis
pub fn set_offline(
  redis_client: redis.RedisClient,
  user_id: String,
) -> Nil {
  let _ = redis.del(redis_client, presence_key(user_id))
  let _ = redis.del(redis_client, "gw:session:" <> user_id)
  Nil
}

/// Check if user is online
pub fn is_online(
  redis_client: redis.RedisClient,
  user_id: String,
) -> Bool {
  case redis.get(redis_client, presence_key(user_id)) {
    Ok("online") -> True
    _ -> False
  }
}

/// Get node_id where user is connected
pub fn get_user_node(
  redis_client: redis.RedisClient,
  user_id: String,
) -> Result(String, redis.RedisError) {
  redis.get(redis_client, "gw:session:" <> user_id)
}
