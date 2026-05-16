import uuid

from django.core.exceptions import ValidationError
from django.db import models


class MessageStatus(models.TextChoices):
    SENT = "sent", "sent"
    DELIVERED = "delivered", "delivered"
    READ = "read", "read"
    DELETED = "deleted", "deleted"


class MessageContentType(models.TextChoices):
    TEXT = "text", "text"
    ATTACHMENT = "attachment", "attachment"
    SYSTEM = "system", "system"


class ReceiptStatus(models.TextChoices):
    DELIVERED = "delivered", "delivered"
    READ = "read", "read"


class OutboxEventStatus(models.TextChoices):
    PENDING = "pending", "pending"
    PUBLISHED = "published", "published"
    FAILED = "failed", "failed"


class ChatSequenceCounter(models.Model):
    chat_id = models.UUIDField(primary_key=True)
    last_sequence = models.BigIntegerField(default=0)

    class Meta:
        db_table = "chat_sequence_counters"
        verbose_name = "Chat sequence counter"
        verbose_name_plural = "Chat sequence counters"

    def __str__(self) -> str:
        return f"{self.chat_id} -> {self.last_sequence}"


class Message(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    chat_id = models.UUIDField(db_index=True)
    sender_id = models.UUIDField(db_index=True)

    content_type = models.CharField(
        max_length=20,
        choices=MessageContentType.choices,
    )
    text = models.TextField(null=True, blank=True)
    attachment = models.JSONField(null=True, blank=True)
    reply_to_id = models.UUIDField(null=True, blank=True)

    status = models.CharField(
        max_length=20,
        choices=MessageStatus.choices,
        default=MessageStatus.SENT,
    )
    sequence_number = models.BigIntegerField()
    idempotency_key = models.CharField(
        max_length=64,
        null=True,
        blank=True,
        unique=True,
    )

    is_edited = models.BooleanField(default=False)
    edited_at = models.DateTimeField(null=True, blank=True)

    created_at = models.DateTimeField(auto_now_add=True, db_index=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = "messages"
        verbose_name = "Message"
        verbose_name_plural = "Messages"
        indexes = [
            models.Index(fields=["chat_id", "sequence_number"], name="msg_chat_seq_idx"),
            models.Index(fields=["chat_id", "sender_id", "created_at"], name="msg_chat_sender_created_idx"),
            models.Index(fields=["chat_id", "created_at"], name="msg_chat_created_idx"),
        ]
        constraints = [
            models.UniqueConstraint(
                fields=["chat_id", "sequence_number"],
                name="messages_chat_sequence_unique",
            ),
        ]
        ordering = ["sequence_number"]

    def __str__(self) -> str:
        return f"{self.id} [{self.chat_id}:{self.sequence_number}]"

    def clean(self) -> None:
        super().clean()

        if self.sequence_number is not None and self.sequence_number <= 0:
            raise ValidationError({"sequence_number": "sequence_number must be positive."})

        if self.content_type == MessageContentType.TEXT:
            if self.status != MessageStatus.DELETED and not self.text:
                raise ValidationError({"text": "text is required for text messages."})
            if self.attachment is not None:
                raise ValidationError({"attachment": "attachment must be null for text messages."})

        elif self.content_type == MessageContentType.ATTACHMENT:
            if self.status != MessageStatus.DELETED and not self.attachment:
                raise ValidationError({"attachment": "attachment is required for attachment messages."})

        elif self.content_type == MessageContentType.SYSTEM:
            if self.attachment is not None:
                raise ValidationError({"attachment": "attachment must be null for system messages."})

        if self.status == MessageStatus.DELETED:
            # Удалённые сообщения остаются в истории, но без контента.
            if self.text not in (None, ""):
                raise ValidationError({"text": "deleted messages must not contain text."})
            if self.attachment is not None:
                raise ValidationError({"attachment": "deleted messages must not contain attachment."})

    def save(self, *args, **kwargs):
        self.full_clean()
        return super().save(*args, **kwargs)


class MessageVersion(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    message_id = models.UUIDField(db_index=True)
    chat_id = models.UUIDField(db_index=True)
    version = models.PositiveIntegerField()
    text = models.TextField()
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = "message_versions"
        verbose_name = "Message version"
        verbose_name_plural = "Message versions"
        indexes = [
            models.Index(fields=["message_id", "created_at"], name="msg_ver_message_created_idx"),
        ]
        constraints = [
            models.UniqueConstraint(
                fields=["message_id", "version"],
                name="message_versions_unique_version",
            ),
        ]
        ordering = ["created_at"]

    def __str__(self) -> str:
        return f"{self.message_id} v{self.version}"


class Receipt(models.Model):
    message_id = models.UUIDField()
    user_id = models.UUIDField()
    chat_id = models.UUIDField(db_index=True)
    status = models.CharField(
        max_length=20,
        choices=ReceiptStatus.choices,
    )
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        db_table = "receipts"
        verbose_name = "Receipt"
        verbose_name_plural = "Receipts"
        constraints = [
            models.UniqueConstraint(
                fields=["message_id", "user_id"],
                name="receipts_message_user_unique",
            ),
        ]
        indexes = [
            models.Index(fields=["chat_id", "message_id"], name="receipt_chat_message_idx"),
            models.Index(fields=["user_id", "chat_id", "updated_at"], name="receipt_user_chat_updated_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.message_id}:{self.user_id} -> {self.status}"


class OutboxEvent(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    event_id = models.UUIDField(default=uuid.uuid4, unique=True, db_index=True)

    event_type = models.CharField(max_length=100)
    topic = models.CharField(max_length=100)
    partition_key = models.CharField(max_length=100)
    payload = models.JSONField()

    status = models.CharField(
        max_length=20,
        choices=OutboxEventStatus.choices,
        default=OutboxEventStatus.PENDING,
    )
    created_at = models.DateTimeField(auto_now_add=True, db_index=True)
    published_at = models.DateTimeField(null=True, blank=True)
    failed_at = models.DateTimeField(null=True, blank=True)
    retry_count = models.IntegerField(default=0)

    class Meta:
        db_table = "outbox_events"
        verbose_name = "Outbox event"
        verbose_name_plural = "Outbox events"
        indexes = [
            models.Index(fields=["status", "created_at"], name="outbox_status_created_idx"),
            models.Index(fields=["published_at"], name="outbox_published_idx"),
        ]

    def __str__(self) -> str:
        return f"{self.event_type} [{self.status}]"

    def clean(self) -> None:
        super().clean()

        if self.retry_count < 0:
            raise ValidationError({"retry_count": "retry_count cannot be negative."})

    def save(self, *args, **kwargs):
        self.full_clean()
        return super().save(*args, **kwargs)