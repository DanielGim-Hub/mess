import uuid

from rest_framework import status
from rest_framework.response import Response
from rest_framework.views import APIView

from .serializers import (
    CreateReceiptRequestSerializer,
    EditMessageRequestSerializer,
    MessageListQuerySerializer,
    MessageSerializer,
    ReceiptSerializer,
    SendMessageRequestSerializer,
)
from .services import (
    create_or_update_receipt,
    create_text_message,
    delete_message,
    edit_text_message,
    get_message_by_id,
    get_receipts_for_message,
    list_messages,
)


class BaseMessageAPIView(APIView):
    authentication_classes = []
    permission_classes = []

    @staticmethod
    def error_response(code, message, http_status, details=None):
        payload = {
            "error": {
                "code": code,
                "message": message,
            }
        }
        if details is not None:
            payload["error"]["details"] = details
        return Response(payload, status=http_status)

    def get_user_uuid(self, request):
        user_id = request.headers.get("X-User-Id")
        if not user_id:
            return None, self.error_response(
                "unauthorized",
                "Missing X-User-Id header",
                status.HTTP_401_UNAUTHORIZED,
            )

        try:
            return uuid.UUID(str(user_id)), None
        except ValueError:
            return None, self.error_response(
                "validation_error",
                "Invalid X-User-Id UUID",
                status.HTTP_422_UNPROCESSABLE_ENTITY,
            )

    def parse_chat_uuid(self, chat_id):
        try:
            return uuid.UUID(str(chat_id)), None
        except ValueError:
            return None, self.error_response(
                "validation_error",
                "Invalid chat_id UUID",
                status.HTTP_422_UNPROCESSABLE_ENTITY,
            )

    def parse_message_uuid(self, message_id):
        try:
            return uuid.UUID(str(message_id)), None
        except ValueError:
            return None, self.error_response(
                "validation_error",
                "Invalid message_id UUID",
                status.HTTP_422_UNPROCESSABLE_ENTITY,
            )


class SendMessageView(BaseMessageAPIView):
    def post(self, request, chat_id):
        serializer = SendMessageRequestSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)

        user_uuid, error = self.get_user_uuid(request)
        if error:
            return error

        chat_uuid, error = self.parse_chat_uuid(chat_id)
        if error:
            return error

        payload = serializer.validated_data
        idempotency_key = request.headers.get("Idempotency-Key")

        if payload["content_type"] == "text":
            message, created = create_text_message(
                chat_id=chat_uuid,
                sender_id=user_uuid,
                text=payload["text"],
                idempotency_key=idempotency_key,
                reply_to_id=payload.get("reply_to_id"),
            )
        else:
            return self.error_response(
                "not_implemented",
                "Attachment messages are not implemented yet",
                status.HTTP_501_NOT_IMPLEMENTED,
            )

        return Response(
            MessageSerializer(message).data,
            status=status.HTTP_201_CREATED if created else status.HTTP_200_OK,
        )


class MessageListView(BaseMessageAPIView):
    def get(self, request, chat_id):
        user_uuid, error = self.get_user_uuid(request)
        if error:
            return error

        chat_uuid, error = self.parse_chat_uuid(chat_id)
        if error:
            return error

        query_serializer = MessageListQuerySerializer(data=request.query_params)
        query_serializer.is_valid(raise_exception=True)
        limit = query_serializer.validated_data.get("limit", 20)
        offset = query_serializer.validated_data.get("offset", 0)

        messages, total = list_messages(chat_id=chat_uuid, limit=limit, offset=offset)

        return Response({
            "items": MessageSerializer(messages, many=True).data,
            "total": total,
            "limit": limit,
            "offset": offset,
        }, status=status.HTTP_200_OK)


class MessageDetailView(BaseMessageAPIView):
    def get(self, request, chat_id, message_id):
        _, error = self.get_user_uuid(request)
        if error:
            return error

        chat_uuid, error = self.parse_chat_uuid(chat_id)
        if error:
            return error

        message_uuid, error = self.parse_message_uuid(message_id)
        if error:
            return error

        message = get_message_by_id(chat_id=chat_uuid, message_id=message_uuid)
        if not message:
            return self.error_response(
                "message_not_found",
                "Message with specified id does not exist",
                status.HTTP_404_NOT_FOUND,
            )

        return Response(MessageSerializer(message).data, status=status.HTTP_200_OK)

    def patch(self, request, chat_id, message_id):
        user_uuid, error = self.get_user_uuid(request)
        if error:
            return error

        serializer = EditMessageRequestSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)

        chat_uuid, error = self.parse_chat_uuid(chat_id)
        if error:
            return error

        message_uuid, error = self.parse_message_uuid(message_id)
        if error:
            return error

        message, error_code = edit_text_message(
            chat_id=chat_uuid,
            message_id=message_uuid,
            editor_id=user_uuid,
            new_text=serializer.validated_data["text"],
        )

        if error_code == "not_found":
            return self.error_response(
                "message_not_found",
                "Message with specified id does not exist",
                status.HTTP_404_NOT_FOUND,
            )

        if error_code == "not_author":
            return self.error_response(
                "not_message_author",
                "Only author can edit this message",
                status.HTTP_403_FORBIDDEN,
            )

        if error_code == "deleted":
            return self.error_response(
                "message_deleted",
                "Cannot edit deleted message",
                status.HTTP_422_UNPROCESSABLE_ENTITY,
            )

        if error_code == "cannot_edit_attachment":
            return self.error_response(
                "cannot_edit_attachment",
                "Cannot edit attachment message",
                status.HTTP_422_UNPROCESSABLE_ENTITY,
            )

        return Response(MessageSerializer(message).data, status=status.HTTP_200_OK)

    def delete(self, request, chat_id, message_id):
        user_uuid, error = self.get_user_uuid(request)
        if error:
            return error

        chat_uuid, error = self.parse_chat_uuid(chat_id)
        if error:
            return error

        message_uuid, error = self.parse_message_uuid(message_id)
        if error:
            return error

        error_code = delete_message(
            chat_id=chat_uuid,
            message_id=message_uuid,
            actor_id=user_uuid,
        )

        if error_code == "not_found":
            return self.error_response(
                "message_not_found",
                "Message with specified id does not exist",
                status.HTTP_404_NOT_FOUND,
            )

        if error_code == "not_author":
            return self.error_response(
                "not_message_author",
                "Only author can delete this message",
                status.HTTP_403_FORBIDDEN,
            )

        return Response(status=status.HTTP_204_NO_CONTENT)


class ReceiptView(BaseMessageAPIView):
    def post(self, request, chat_id, message_id):
        user_uuid, error = self.get_user_uuid(request)
        if error:
            return error

        chat_uuid, error = self.parse_chat_uuid(chat_id)
        if error:
            return error

        message_uuid, error = self.parse_message_uuid(message_id)
        if error:
            return error

        serializer = CreateReceiptRequestSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)

        receipt, error_code = create_or_update_receipt(
            chat_id=chat_uuid,
            message_id=message_uuid,
            user_id=user_uuid,
            status=serializer.validated_data["status"],
        )

        if error_code == "message_not_found":
            return self.error_response(
                "message_not_found",
                "Message not found",
                status.HTTP_404_NOT_FOUND,
            )

        if error_code == "invalid_status":
            return self.error_response(
                "validation_error",
                "Invalid receipt status",
                status.HTTP_422_UNPROCESSABLE_ENTITY,
            )

        return Response(ReceiptSerializer(receipt).data, status=status.HTTP_201_CREATED)

    def get(self, request, chat_id, message_id):
        _, error = self.get_user_uuid(request)
        if error:
            return error

        chat_uuid, error = self.parse_chat_uuid(chat_id)
        if error:
            return error

        message_uuid, error = self.parse_message_uuid(message_id)
        if error:
            return error

        receipts = get_receipts_for_message(chat_id=chat_uuid, message_id=message_uuid)
        return Response(ReceiptSerializer(receipts, many=True).data, status=status.HTTP_200_OK)


class HealthLiveView(APIView):
    authentication_classes = []
    permission_classes = []

    def get(self, request):
        return Response({"status": "ok"}, status=status.HTTP_200_OK)
