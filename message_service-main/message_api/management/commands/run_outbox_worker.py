import json
import logging
import signal
import sys
import time
from datetime import datetime, timedelta

from django.core.management.base import BaseCommand
from django.db import connection, transaction
from django.utils import timezone

logger = logging.getLogger(__name__)


class Command(BaseCommand):
    help = "Run Outbox Worker: reads outbox_events and publishes to Kafka"

    def add_arguments(self, parser):
        parser.add_argument(
            "--interval",
            type=int,
            default=1,
            help="Polling interval in seconds (default: 1)",
        )
        parser.add_argument(
            "--batch-size",
            type=int,
            default=100,
            help="Batch size per poll (default: 100)",
        )
        parser.add_argument(
            "--max-retries",
            type=int,
            default=5,
            help="Max retries before marking as failed (default: 5)",
        )

    def handle(self, *args, **options):
        self.interval = options["interval"]
        self.batch_size = options["batch_size"]
        self.max_retries = options["max_retries"]
        self.running = True

        # Graceful shutdown
        signal.signal(signal.SIGTERM, self._signal_handler)
        signal.signal(signal.SIGINT, self._signal_handler)

        brokers = self._get_kafka_brokers()
        self.stdout.write(self.style.SUCCESS(f"Starting Outbox Worker... Kafka: {brokers}"))

        try:
            from kafka import KafkaProducer

            self.producer = KafkaProducer(
                bootstrap_servers=brokers,
                value_serializer=lambda v: json.dumps(v).encode("utf-8"),
            )
        except Exception as e:
            self.stdout.write(self.style.ERROR(f"Kafka connect failed: {e}"))
            self.producer = None

        while self.running:
            try:
                self._process_batch()
            except Exception as e:
                logger.exception("Outbox worker error")
            time.sleep(self.interval)

        if self.producer:
            self.producer.close()
        self.stdout.write(self.style.SUCCESS("Outbox Worker stopped."))

    def _signal_handler(self, signum, frame):
        self.stdout.write(self.style.WARNING(f"Received signal {signum}, shutting down..."))
        self.running = False

    def _get_kafka_brokers(self):
        import os

        env_brokers = os.getenv("KAFKA_BROKERS", "kafka:9092")
        return [b.strip() for b in env_brokers.split(",")]

    def _process_batch(self):
        with connection.cursor() as cursor:
            # Select unpublished events with SKIP LOCKED for concurrent workers
            cursor.execute(
                """
                SELECT id, event_id, event_type, topic, partition_key, payload, retry_count
                FROM outbox_events
                WHERE published_at IS NULL AND (failed_at IS NULL OR retry_count < %s)
                ORDER BY created_at
                LIMIT %s
                FOR UPDATE SKIP LOCKED
                """,
                [self.max_retries, self.batch_size],
            )
            rows = cursor.fetchall()

            if not rows:
                return

            for row in rows:
                event_id, event_uuid, event_type, topic, partition_key, payload, retry_count = row
                success = self._publish_event(topic, partition_key, payload)

                if success:
                    cursor.execute(
                        """
                        UPDATE outbox_events
                        SET published_at = %s, status = 'published'
                        WHERE id = %s
                        """,
                        [timezone.now(), event_id],
                    )
                    logger.info(f"Published event {event_uuid} to {topic}")
                else:
                    new_retry = retry_count + 1
                    failed_at = timezone.now() if new_retry >= self.max_retries else None
                    status_val = "failed" if new_retry >= self.max_retries else "pending"
                    cursor.execute(
                        """
                        UPDATE outbox_events
                        SET retry_count = %s, failed_at = %s, status = %s
                        WHERE id = %s
                        """,
                        [new_retry, failed_at, status_val, event_id],
                    )
                    logger.warning(
                        f"Failed to publish event {event_uuid}, retry {new_retry}/{self.max_retries}"
                    )

    def _publish_event(self, topic, partition_key, payload):
        if not self.producer:
            return False
        try:
            future = self.producer.send(
                topic,
                key=partition_key.encode() if partition_key else None,
                value=payload,
            )
            future.get(timeout=10)  # Wait for confirmation
            return True
        except Exception as e:
            logger.error(f"Kafka publish error: {e}")
            return False
