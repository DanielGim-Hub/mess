import uuid

from rest_framework import serializers

from .models import Message, Receipt


class SendMessageRequestSerializer(serializers.Serializer):
    content_type = serializers.ChoiceField(choices=["text", "attachment", "system"])
    text = serializers.CharField(required=False, allow_blank=False, max_length=4096)
    attachment_id = serializers.UUIDField(required=False)
    reply_to_id = serializers.UUIDField(required=False, allow_null=True)

    def validate(self, attrs):
        content_type = attrs.get("content_type")
        text = attrs.get("text")
        attachment_id = attrs.get("attachment_id")

        if content_type == "text":
            if not text:
                raise serializers.ValidationError({"text": "This field is required for text messages."})
            if attachment_id:
                raise serializers.ValidationError({"attachment_id": "Must not be provided for text messages."})

        elif content_type == "attachment":
            if not attachment_id:
                raise serializers.ValidationError({"attachment_id": "This field is required for attachment messages."})
            if text:
                raise serializers.ValidationError({"text": "Must not be provided for attachment messages in current MVP."})

        elif content_type == "system":
            raise serializers.ValidationError({"content_type": "Client cannot send system messages."})

        return attrs


class MessageSerializer(serializers.ModelSerializer):
    class Meta:
        model = Message
        fields = [
            "id",
            "chat_id",
            "sender_id",
            "content_type",
            "text",
            "attachment",
            "reply_to_id",
            "status",
            "sequence_number",
            "is_edited",
            "edited_at",
            "created_at",
            "updated_at",
        ]


class EditMessageRequestSerializer(serializers.Serializer):
    text = serializers.CharField(required=True, allow_blank=False, max_length=4096)


class ReceiptSerializer(serializers.ModelSerializer):
    class Meta:
        model = Receipt
        fields = ["message_id", "user_id", "chat_id", "status", "created_at", "updated_at"]


class CreateReceiptRequestSerializer(serializers.Serializer):
    status = serializers.ChoiceField(choices=["delivered", "read"])


class MessageListQuerySerializer(serializers.Serializer):
    limit = serializers.IntegerField(required=False, min_value=1, max_value=100, default=20)
    offset = serializers.IntegerField(required=False, min_value=0, default=0)
