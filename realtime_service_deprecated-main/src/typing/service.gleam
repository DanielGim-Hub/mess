import infra/redis

pub fn start_typing(
  redis_client: redis.RedisClient,
  chat_id: String,
  user_id: String,
) -> Nil {
  let key = "gw:typing:" <> chat_id <> ":" <> user_id
  let _ = redis.setex(redis_client, key, "1", 5)
  Nil
}

pub fn stop_typing(
  redis_client: redis.RedisClient,
  chat_id: String,
  user_id: String,
) -> Nil {
  let key = "gw:typing:" <> chat_id <> ":" <> user_id
  let _ = redis.del(redis_client, key)
  Nil
}

pub fn is_typing(
  redis_client: redis.RedisClient,
  chat_id: String,
  user_id: String,
) -> Bool {
  let key = "gw:typing:" <> chat_id <> ":" <> user_id
  case redis.get(redis_client, key) {
    Ok(_) -> True
    _ -> False
  }
}
