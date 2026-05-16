// Redis infrastructure module
// Uses Erlang FFI for redis operations via eredis or similar

import gleam/dynamic

pub type RedisError {
  ConnectionError(String)
  CommandError(String)
}

pub type RedisClient

/// Initialize Redis connection from URL
@external(erlang, "redis_ffi", "connect")
pub fn connect(url: String) -> Result(RedisClient, RedisError)

/// Set key with TTL (seconds)
@external(erlang, "redis_ffi", "setex")
pub fn setex(
  client: RedisClient,
  key: String,
  value: String,
  ttl_seconds: Int,
) -> Result(Nil, RedisError)

/// Get key value
@external(erlang, "redis_ffi", "get")
pub fn get(client: RedisClient, key: String) -> Result(String, RedisError)

/// Delete key
@external(erlang, "redis_ffi", "del")
pub fn del(client: RedisClient, key: String) -> Result(Nil, RedisError)

/// Publish to channel
@external(erlang, "redis_ffi", "publish")
pub fn publish(
  client: RedisClient,
  channel: String,
  message: String,
) -> Result(Nil, RedisError)

// Session keys
pub fn session_key(user_id: String) -> String {
  "gw:session:" <> user_id
}

pub fn presence_key(user_id: String) -> String {
  "gw:presence:" <> user_id
}

pub fn typing_key(chat_id: String, user_id: String) -> String {
  "gw:typing:" <> chat_id <> ":" <> user_id
}

pub fn dedup_key(event_id: String) -> String {
  "gw:dedup:" <> event_id
}

pub fn node_key(node_id: String) -> String {
  "gw:node:" <> node_id
}
