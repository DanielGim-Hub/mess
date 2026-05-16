"""
Locust Kafka Load Test.
Симуляция пиковой нагрузки в топики Kafka, проверка consumer lag.
"""
import uuid
import json
import time
from locust import User, task, between
from kafka import KafkaProducer


class KafkaUser(User):
    abstract = True

    def __init__(self, environment):
        super().__init__(environment)
        self.producer = None

    def on_start(self):
        try:
            self.producer = KafkaProducer(
                bootstrap_servers=["kafka:9092"],
                value_serializer=lambda v: json.dumps(v).encode("utf-8"),
            )
        except Exception as e:
            self.environment.events.request.fire(
                request_type="KAFKA",
                name="producer_connect",
                response_time=0,
                response_length=0,
                exception=e,
            )

    def on_stop(self):
        if self.producer:
            self.producer.close()

    def send_event(self, topic, payload, key=None):
        if not self.producer:
            return
        start = time.time()
        try:
            partition_key = key or str(uuid.uuid4())
            future = self.producer.send(topic, key=partition_key.encode(), value=payload)
            future.get(timeout=5)
            elapsed = (time.time() - start) * 1000
            self.environment.events.request.fire(
                request_type="KAFKA",
                name=f"produce_{topic}",
                response_time=elapsed,
                response_length=len(json.dumps(payload)),
                response=200,
            )
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            self.environment.events.request.fire(
                request_type="KAFKA",
                name=f"produce_{topic}",
                response_time=elapsed,
                response_length=0,
                exception=e,
            )


class MessengerKafkaUser(KafkaUser):
    wait_time = between(0.01, 0.05)

    @task(10)
    def produce_message_created(self):
        chat_id = str(uuid.uuid4())
        payload = {
            "event_id": str(uuid.uuid4()),
            "event_type": "message.created",
            "topic": "message.created",
            "partition_key": chat_id,
            "payload": {
                "message_id": str(uuid.uuid4()),
                "chat_id": chat_id,
                "sender_id": str(uuid.uuid4()),
                "content_type": "text",
                "text": f"Kafka load test {uuid.uuid4().hex[:16]}",
                "sequence_number": random.randint(1, 100000),
                "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ"),
            },
            "metadata": {
                "source_service": "message-service",
                "payload_version": "1.0",
                "occurred_at": time.strftime("%Y-%m-%dT%H:%M:%SZ"),
            },
        }
        self.send_event("message.created", payload, key=chat_id)

    @task(3)
    def produce_chat_event(self):
        chat_id = str(uuid.uuid4())
        payload = {
            "event_id": str(uuid.uuid4()),
            "event_type": "chat.updated",
            "topic": "chat.events",
            "partition_key": chat_id,
            "payload": {
                "chat_id": chat_id,
                "type": "group",
                "title": f"LoadTest {uuid.uuid4().hex[:8]}",
                "updated_by": str(uuid.uuid4()),
                "changes": ["title"],
            },
        }
        self.send_event("chat.events", payload, key=chat_id)

    @task(2)
    def produce_presence_event(self):
        user_id = str(uuid.uuid4())
        payload = {
            "event_id": str(uuid.uuid4()),
            "event_type": "presence.updated",
            "topic": "presence.events",
            "partition_key": user_id,
            "payload": {
                "user_id": user_id,
                "status": random.choice(["online", "offline"]),
                "node_id": f"node-{random.randint(1, 5)}",
            },
        }
        self.send_event("presence.events", payload, key=user_id)

    @task(2)
    def produce_receipt_event(self):
        chat_id = str(uuid.uuid4())
        payload = {
            "event_id": str(uuid.uuid4()),
            "event_type": "receipt.delivered",
            "topic": "receipt.events",
            "partition_key": chat_id,
            "payload": {
                "message_id": str(uuid.uuid4()),
                "chat_id": chat_id,
                "user_id": str(uuid.uuid4()),
                "status": "delivered",
            },
        }
        self.send_event("receipt.events", payload, key=chat_id)
