import infra/redis
import kafka/client as kafka
import youid/uuid

/// Process ack from client and publish receipt.delivered to Kafka
pub fn process_ack(
  redis_client: redis.RedisClient,
  kafka_producer: kafka.KafkaProducer,
  user_id: String,
  event_id: String,
) -> Nil {
  // Dedup check
  let dedup_key = "gw:dedup:" <> event_id
  case redis.get(redis_client, dedup_key) {
    Ok(_) -> Nil  // Already processed
    _ -> {
      let _ = redis.setex(redis_client, dedup_key, "1", 86_400)
      let payload =
        "{\"event_id\":\"" <> event_id <> "\",\"user_id\":\"" <> user_id <> "\",\"status\":\"delivered\"}"
      let _ = kafka.produce(kafka_producer, "receipt.events", user_id, payload)
      Nil
    }
  }
}
