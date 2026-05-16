from django.db import transaction
from django.utils import timezone

from .models import (
    ChatSequenceCounter,
    Message,
    MessageContentType,
    MessageStatus,
    MessageVersion,
    OutboxEvent,
    OutboxEventStatus,
    Receipt,
    ReceiptStatus,
)


def get_message_by_id(*, chat_id, message_id):
    return Message.objects.filter(chat_id=chat_id, id=message_id).first()


def list_messages(*, chat_id, limit=20, offset=0):
    """List messages in a chat with pagination."""
    qs = Message.objects.filter(chat_id=chat_id).order_by("-sequence_number")
    total = qs.count()
    messages = qs[offset : offset + limit]
    return messages, total


def get_next_sequence_number(chat_id):
    counter, _ = ChatSequenceCounter.objects.select_for_update().get_or_create(
        chat_id=chat_id,
        defaults={"last_sequence": 0},
    )
    counter.last_sequence += 1
    counter.save(update_fields=["last_sequence"])
    return counter.last_sequence


def create_outbox_event(*, event_type, topic, partition_key, payload):
    outbox_event = OutboxEvent.objects.create(
        event_type=event_type,
        topic=topic,
        partition_key=str(partition_key),
        status=OutboxEventStatus.PENDING,
        payload={
            "event_id": None,
            "event_type": event_type,
            "occurred_at": timezone.now().isoformat(),
            "source_service": "message-service",
            "payload_version": 1,
            "payload": payload,
        },
    )

    outbox_event.payload["event_id"] = str(outbox_event.event_id)
    outbox_event.save(update_fields=["payload"])

    return outbox_event


@transaction.atomic
def create_text_message(
    *,
    chat_id,
    sender_id,
    text,
    idempotency_key=None,
    reply_to_id=None,
):
    if idempotency_key:
        existing_message = (
            Message.objects.select_for_update()
            .filter(idempotency_key=idempotency_key)
            .first()
        )
        if existing_message:
            return existing_message, False

    sequence_number = get_next_sequence_number(chat_id)

    message = Message.objects.create(
        chat_id=chat_id,
        sender_id=sender_id,
        content_type=MessageContentType.TEXT,
        text=text,
        attachment=None,
        reply_to_id=reply_to_id,
        status=MessageStatus.SENT,
        sequence_number=sequence_number,
        idempotency_key=idempotency_key,
    )

    create_outbox_event(
        event_type="message.created",
        topic="message.created",
        partition_key=chat_id,
        payload={
            "message_id": str(message.id),
            "chat_id": str(message.chat_id),
            "sender_id": str(message.sender_id),
            "content_type": message.content_type,
            "text": message.text,
            "attachment": message.attachment,
            "reply_to_id": str(message.reply_to_id) if message.reply_to_id else None,
            "sequence_number": message.sequence_number,
            "idempotency_key": message.idempotency_key,
        },
    )

    return message, True


@transaction.atomic
def edit_text_message(
    *,
    chat_id,
    message_id,
    editor_id,
    new_text,
):
    message = (
        Message.objects.select_for_update()
        .filter(chat_id=chat_id, id=message_id)
        .first()
    )

    if not message:
        return None, "not_found"

    if message.sender_id != editor_id:
        return None, "not_author"

    if message.status == MessageStatus.DELETED:
        return None, "deleted"

    if message.content_type != MessageContentType.TEXT:
        return None, "cannot_edit_attachment"

    current_version = MessageVersion.objects.filter(message_id=message.id).count() + 1

    MessageVersion.objects.create(
        message_id=message.id,
        chat_id=message.chat_id,
        version=current_version,
        text=message.text or "",
    )

    previous_text = message.text
    edited_at = timezone.now()

    message.text = new_text
    message.is_edited = True
    message.edited_at = edited_at
    message.save(update_fields=["text", "is_edited", "edited_at", "updated_at"])

    create_outbox_event(
        event_type="message.updated",
        topic="message.updated",
        partition_key=chat_id,
        payload={
            "message_id": str(message.id),
            "chat_id": str(message.chat_id),
            "sender_id": str(message.sender_id),
            "text": message.text,
            "previous_text": previous_text,
            "sequence_number": message.sequence_number,
            "edited_at": edited_at.isoformat(),
        },
    )

    return message, None


@transaction.atomic
def delete_message(
    *,
    chat_id,
    message_id,
    actor_id,
):
    message = (
        Message.objects.select_for_update()
        .filter(chat_id=chat_id, id=message_id)
        .first()
    )

    if not message:
        return "not_found"

    if message.sender_id != actor_id:
        return "not_author"

    if message.status == MessageStatus.DELETED:
        return None

    deleted_at = timezone.now()

    message.status = MessageStatus.DELETED
    message.text = None
    message.attachment = None
    message.save(update_fields=["status", "text", "attachment", "updated_at"])

    create_outbox_event(
        event_type="message.deleted",
        topic="message.deleted",
        partition_key=chat_id,
        payload={
            "message_id": str(message.id),
            "chat_id": str(message.chat_id),
            "deleted_by": str(actor_id),
            "sequence_number": message.sequence_number,
            "deleted_at": deleted_at.isoformat(),
        },
    )

    return None


@transaction.atomic
def create_or_update_receipt(*, chat_id, message_id, user_id, status):
    """Create or update a receipt for a message."""
    message = Message.objects.filter(chat_id=chat_id, id=message_id).first()
    if not message:
        return None, "message_not_found"

    valid_statuses = {ReceiptStatus.DELIVERED, ReceiptStatus.READ}
    if status not in valid_statuses:
        return None, "invalid_status"

    # Don't allow downgrading status (read -> delivered)
    existing = Receipt.objects.filter(message_id=message_id, user_id=user_id).first()
    if existing:
        if existing.status == ReceiptStatus.READ and status == ReceiptStatus.DELIVERED:
            return existing, None  # Already read, ignore delivered
        if existing.status == status:
            return existing, None  # No change
        existing.status = status
        existing.save(update_fields=["status", "updated_at"])
        return existing, None

    receipt = Receipt.objects.create(
        message_id=message_id,
        user_id=user_id,
        chat_id=chat_id,
        status=status,
    )
    return receipt, None


def get_receipts_for_message(*, chat_id, message_id):
    """Get all receipts for a message."""
    return Receipt.objects.filter(chat_id=chat_id, message_id=message_id)
