// Kafka client wrapper for Realtime Gateway
// Uses franz library (already in dependencies)

import gleam/dynamic

pub type KafkaError {
  ConnectionError(String)
  PublishError(String)
}

pub type KafkaProducer
pub type KafkaConsumer

/// Initialize Kafka producer
@external(erlang, "kafka_ffi", "new_producer")
pub fn new_producer(brokers: String) -> Result(KafkaProducer, KafkaError)

/// Publish event to topic
@external(erlang, "kafka_ffi", "produce")
pub fn produce(
  producer: KafkaProducer,
  topic: String,
  key: String,
  value: String,
) -> Result(Nil, KafkaError)

/// Initialize Kafka consumer
@external(erlang, "kafka_ffi", "new_consumer")
pub fn new_consumer(
  brokers: String,
  topics: List(String),
  group_id: String,
) -> Result(KafkaConsumer, KafkaError)

/// Poll messages (returns empty list if none)
@external(erlang, "kafka_ffi", "poll")
pub fn poll(consumer: KafkaConsumer) -> List(KafkaMessage)

pub type KafkaMessage {
  KafkaMessage(topic: String, key: String, value: String, offset: Int)
}
