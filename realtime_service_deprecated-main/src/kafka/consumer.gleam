import gleam/erlang/process
import infra/redis
import kafka/client as kafka
import kafka/dispatcher

pub type ConsumerState {
  ConsumerState(
    consumer: kafka.KafkaConsumer,
    redis: redis.RedisClient,
    running: Bool,
  )
}

/// Start Kafka consumer loop
pub fn start(
  consumer: kafka.KafkaConsumer,
  redis_client: redis.RedisClient,
) -> Nil {
  let state = ConsumerState(consumer: consumer, redis: redis_client, running: True)
  consume_loop(state)
}

fn consume_loop(state: ConsumerState) -> Nil {
  case state.running {
    False -> Nil
    True -> {
      let messages = kafka.poll(state.consumer)
      case messages {
        [] -> {
          process.sleep(100)
          consume_loop(state)
        }
        msgs -> {
          handle_messages(state, msgs)
          consume_loop(state)
        }
      }
    }
  }
}

fn handle_messages(state: ConsumerState, messages: List(kafka.KafkaMessage)) -> Nil {
  case messages {
    [] -> Nil
    [msg, ..rest] -> {
      dispatcher.dispatch(state.redis, msg.topic, msg.key, msg.value)
      handle_messages(state, rest)
    }
  }
}

/// Stop consumer gracefully
pub fn stop(state: ConsumerState) -> ConsumerState {
  ConsumerState(..state, running: False)
}
