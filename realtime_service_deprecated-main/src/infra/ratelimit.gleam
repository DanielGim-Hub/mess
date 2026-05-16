import infra/redis

/// Check if user exceeded rate limit (token bucket in Redis)
/// Returns True if request should be allowed
pub fn check_rate_limit(
  redis_client: redis.RedisClient,
  key: String,
  max_requests: Int,
  window_seconds: Int,
) -> Bool {
  let current = case redis.get(redis_client, key) {
    Ok(val) -> parse_int(val)
    _ -> 0
  }

  case current >= max_requests {
    True -> False
    False -> {
      let new_val = current + 1
      let _ = redis.setex(redis_client, key, int_to_string(new_val), window_seconds)
      True
    }
  }
}

/// Check message rate limit per user (60/min)
pub fn check_message_rate(
  redis_client: redis.RedisClient,
  user_id: String,
) -> Bool {
  let key = "gw:ratelimit:msg:" <> user_id
  check_rate_limit(redis_client, key, 60, 60)
}

/// Check typing rate limit per chat (1 per 3 sec)
pub fn check_typing_rate(
  redis_client: redis.RedisClient,
  chat_id: String,
  user_id: String,
) -> Bool {
  let key = "gw:ratelimit:typing:" <> chat_id <> ":" <> user_id
  check_rate_limit(redis_client, key, 1, 3)
}

/// Check WS handshake rate limit per IP (20/min)
pub fn check_handshake_rate(
  redis_client: redis.RedisClient,
  ip: String,
) -> Bool {
  let key = "gw:ratelimit:handshake:" <> ip
  check_rate_limit(redis_client, key, 20, 60)
}

@external(erlang, "erlang", "binary_to_integer")
fn parse_int(value: String) -> Int

@external(erlang, "erlang", "integer_to_binary")
fn int_to_string(value: Int) -> String
