import envie

pub type Config {
  Config(
    port: Int,
    redis_url: String,
    kafka_brokers: String,
    auth_jwks_url: String,
    node_id: String,
  )
}

pub fn load() -> Config {
  Config(
    port: envie.get_int("PORT") |> envie.unwrap(8082),
    redis_url: envie.get_string("REDIS_URL") |> envie.unwrap("redis://localhost:6379"),
    kafka_brokers: envie.get_string("KAFKA_BROKERS") |> envie.unwrap("localhost:9092"),
    auth_jwks_url: envie.get_string("AUTH_JWKS_URL") |> envie.unwrap(""),
    node_id: envie.get_string("NODE_ID") |> envie.unwrap("node-1"),
  )
}
