import infra/redis

/// Check if event was already processed (24h TTL)
pub fn is_processed(
  redis_client: redis.RedisClient,
  event_id: String,
) -> Bool {
  let key = "gw:dedup:" <> event_id
  case redis.get(redis_client, key) {
    Ok(_) -> True
    _ -> False
  }
}

/// Mark event as processed
pub fn mark_processed(
  redis_client: redis.RedisClient,
  event_id: String,
) -> Nil {
  let key = "gw:dedup:" <> event_id
  let _ = redis.setex(redis_client, key, "1", 86_400)
  Nil
}
