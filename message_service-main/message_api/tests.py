import uuid
from unittest.mock import patch

from django.test import SimpleTestCase
from rest_framework import status
from rest_framework.test import APIClient


class MessageApiRoutingTests(SimpleTestCase):
    def setUp(self):
        self.client = APIClient()
        self.chat_id = uuid.UUID("11111111-1111-1111-1111-111111111111")
        self.message_id = uuid.UUID("9c1a300d-0096-46be-9801-cd2be97127d1")
        self.user_id = uuid.UUID("22222222-2222-2222-2222-222222222222")
        self.detail_url = f"/api/v1/chats/{self.chat_id}/messages/{self.message_id}"
        self.collection_url = f"/api/v1/chats/{self.chat_id}/messages"

    def auth_headers(self):
        return {"HTTP_X_USER_ID": str(self.user_id)}

    @patch("message_api.views.get_message_by_id")
    def test_get_message_detail_uses_rest_contract_url(self, get_message_by_id_mock):
        get_message_by_id_mock.return_value = {
            "id": str(self.message_id),
            "chat_id": str(self.chat_id),
            "sender_id": str(self.user_id),
            "content_type": "text",
            "text": "hello",
            "attachment": None,
            "reply_to_id": None,
            "status": "sent",
            "sequence_number": 1,
            "is_edited": False,
            "edited_at": None,
            "created_at": "2026-03-22T12:00:00Z",
            "updated_at": "2026-03-22T12:00:00Z",
        }

        response = self.client.get(self.detail_url, **self.auth_headers())

        self.assertEqual(response.status_code, status.HTTP_200_OK)
        get_message_by_id_mock.assert_called_once_with(
            chat_id=self.chat_id,
            message_id=self.message_id,
        )

    @patch("message_api.views.edit_text_message")
    def test_patch_message_detail_uses_same_url(self, edit_text_message_mock):
        edit_text_message_mock.return_value = (
            {
                "id": str(self.message_id),
                "chat_id": str(self.chat_id),
                "sender_id": str(self.user_id),
                "content_type": "text",
                "text": "updated text",
                "attachment": None,
                "reply_to_id": None,
                "status": "sent",
                "sequence_number": 1,
                "is_edited": True,
                "edited_at": "2026-03-22T12:30:00Z",
                "created_at": "2026-03-22T12:00:00Z",
                "updated_at": "2026-03-22T12:30:00Z",
            },
            None,
        )

        response = self.client.patch(
            self.detail_url,
            data={"text": "updated text"},
            format="json",
            **self.auth_headers(),
        )

        self.assertEqual(response.status_code, status.HTTP_200_OK)
        edit_text_message_mock.assert_called_once_with(
            chat_id=self.chat_id,
            message_id=self.message_id,
            editor_id=self.user_id,
            new_text="updated text",
        )

    @patch("message_api.views.delete_message")
    def test_delete_message_detail_uses_same_url(self, delete_message_mock):
        delete_message_mock.return_value = None

        response = self.client.delete(self.detail_url, **self.auth_headers())

        self.assertEqual(response.status_code, status.HTTP_204_NO_CONTENT)
        delete_message_mock.assert_called_once_with(
            chat_id=self.chat_id,
            message_id=self.message_id,
            actor_id=self.user_id,
        )

    @patch("message_api.views.create_text_message")
    def test_post_message_collection_still_works(self, create_text_message_mock):
        create_text_message_mock.return_value = (
            {
                "id": str(self.message_id),
                "chat_id": str(self.chat_id),
                "sender_id": str(self.user_id),
                "content_type": "text",
                "text": "hello",
                "attachment": None,
                "reply_to_id": None,
                "status": "sent",
                "sequence_number": 1,
                "is_edited": False,
                "edited_at": None,
                "created_at": "2026-03-22T12:00:00Z",
                "updated_at": "2026-03-22T12:00:00Z",
            },
            True,
        )

        response = self.client.post(
            self.collection_url,
            data={"content_type": "text", "text": "hello"},
            format="json",
            **self.auth_headers(),
        )

        self.assertEqual(response.status_code, status.HTTP_201_CREATED)
        create_text_message_mock.assert_called_once_with(
            chat_id=self.chat_id,
            sender_id=self.user_id,
            text="hello",
            idempotency_key=None,
            reply_to_id=None,
        )

    def test_edit_suffix_route_is_not_supported(self):
        response = self.client.patch(
            f"{self.detail_url}/edit",
            data={"text": "updated text"},
            format="json",
            **self.auth_headers(),
        )

        self.assertEqual(response.status_code, status.HTTP_404_NOT_FOUND)
