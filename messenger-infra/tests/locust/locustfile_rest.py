"""
Locust REST Load Test for Messenger API Gateway.
Симулирует поток запросов через API Gateway в микросервисы Chat и Message.
"""
import uuid
import random
from locust import HttpUser, task, between


class MessengerRestUser(HttpUser):
    wait_time = between(0.5, 2)

    def on_start(self):
        self.user_id = str(uuid.uuid4())
        self.headers = {
            "X-User-Id": self.user_id,
            "Content-Type": "application/json",
        }
        self.chat_ids = []
        self.message_ids = {}  # chat_id -> list of message_ids

    @task(3)
    def create_chat(self):
        payload = {
            "type": "group",
            "title": f"LoadTest Chat {uuid.uuid4().hex[:8]}",
            "user_ids": [str(uuid.uuid4()), str(uuid.uuid4())],
        }
        with self.client.post(
            "/api/v1/chats",
            json=payload,
            headers=self.headers,
            catch_response=True,
        ) as resp:
            if resp.status_code == 201:
                chat_id = resp.json().get("id")
                if chat_id:
                    self.chat_ids.append(chat_id)
                    self.message_ids[chat_id] = []
                resp.success()
            elif resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(5)
    def send_message(self):
        if not self.chat_ids:
            return
        chat_id = random.choice(self.chat_ids)
        payload = {
            "content_type": "text",
            "text": f"Load test message {uuid.uuid4().hex[:16]}",
        }
        with self.client.post(
            f"/api/v1/chats/{chat_id}/messages",
            json=payload,
            headers=self.headers,
            catch_response=True,
        ) as resp:
            if resp.status_code in (200, 201):
                msg_id = resp.json().get("id")
                if msg_id:
                    self.message_ids.setdefault(chat_id, []).append(msg_id)
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(4)
    def list_messages(self):
        if not self.chat_ids:
            return
        chat_id = random.choice(self.chat_ids)
        params = {"limit": random.randint(10, 50), "offset": 0}
        with self.client.get(
            f"/api/v1/chats/{chat_id}/messages",
            params=params,
            headers=self.headers,
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(2)
    def edit_message(self):
        if not self.chat_ids:
            return
        chat_id = random.choice(self.chat_ids)
        messages = self.message_ids.get(chat_id, [])
        if not messages:
            return
        message_id = random.choice(messages)
        payload = {"text": f"Edited message {uuid.uuid4().hex[:16]}"}
        with self.client.patch(
            f"/api/v1/chats/{chat_id}/messages/{message_id}",
            json=payload,
            headers=self.headers,
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(1)
    def delete_message(self):
        if not self.chat_ids:
            return
        chat_id = random.choice(self.chat_ids)
        messages = self.message_ids.get(chat_id, [])
        if not messages:
            return
        message_id = messages.pop(0)
        with self.client.delete(
            f"/api/v1/chats/{chat_id}/messages/{message_id}",
            headers=self.headers,
            catch_response=True,
        ) as resp:
            if resp.status_code == 204:
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(2)
    def create_receipt(self):
        if not self.chat_ids:
            return
        chat_id = random.choice(self.chat_ids)
        messages = self.message_ids.get(chat_id, [])
        if not messages:
            return
        message_id = random.choice(messages)
        payload = {"status": random.choice(["delivered", "read"])}
        with self.client.post(
            f"/api/v1/chats/{chat_id}/messages/{message_id}/receipts",
            json=payload,
            headers=self.headers,
            catch_response=True,
        ) as resp:
            if resp.status_code in (200, 201):
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(2)
    def list_chats(self):
        self.client.get("/api/v1/chats", headers=self.headers)

    @task(1)
    def get_chat(self):
        if not self.chat_ids:
            return
        chat_id = random.choice(self.chat_ids)
        self.client.get(f"/api/v1/chats/{chat_id}", headers=self.headers)

    @task(1)
    def list_members(self):
        if not self.chat_ids:
            return
        chat_id = random.choice(self.chat_ids)
        self.client.get(f"/api/v1/chats/{chat_id}/members", headers=self.headers)
