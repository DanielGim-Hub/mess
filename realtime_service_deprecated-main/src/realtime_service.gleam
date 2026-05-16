import dream/servers/mist/server
import gleam/erlang/process
import infra/config
import infra/redis
import kafka/client as kafka
import kafka/consumer
import ws/router

const default_port = 8082

pub fn main() -> Nil {
  let cfg = config.load()

  // Initialize Redis
  let redis_client = case redis.connect(cfg.redis_url) {
    Ok(client) -> {
      let _ = redis.setex(client, "gw:node:" <> cfg.node_id, "active", 60)
      Some(client)
    }
    Error(_) -> None
  }

  // Initialize Kafka producer
  let kafka_producer = case kafka.new_producer(cfg.kafka_brokers) {
    Ok(producer) -> Some(producer)
    Error(_) -> None
  }

  // Initialize Kafka consumer in background
  case redis_client, kafka.new_consumer(cfg.kafka_brokers, ["message.created", "chat.events", "receipt.events", "presence.events"], "realtime-gateway-consumer") {
    Some(redis), Ok(kafka_consumer) -> {
      process.start(fn() { consumer.start(kafka_consumer, redis) }, True)
      Nil
    }
    _, _ -> Nil
  }

  server.new()
  |> server.router(router.create_router())
  |> server.listen(cfg.port)
}
