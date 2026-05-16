"""
Locust WebSocket Load Test for Realtime Gateway.
Симулирует persistent WS-соединения с корректным envelope протоколом.
"""
import uuid
import json
import random
import time
from locust import User, task, between, events
import websocket


class WebSocketUser(User):
    abstract = True

    def __init__(self, environment):
        super().__init__(environment)
        self.ws = None
        self.user_id = str(uuid.uuid4())
        self.chat_ids = []

    def on_start(self):
        ws_url = f"ws://ws.messenger.local/ws?token=dummy-{self.user_id}"
        try:
            self.ws = websocket.create_connection(ws_url, timeout=5)
            # Wait for connected event
            msg = self.ws.recv()
            data = json.loads(msg)
            if data.get("type") == "connected":
                self.environment.events.request.fire(
                    request_type="WS",
                    name="connect",
                    response_time=0,
                    response_length=len(msg),
                    response=200,
                )
        except Exception as e:
            self.environment.events.request.fire(
                request_type="WS",
                name="connect",
                response_time=0,
                response_length=0,
                exception=e,
            )

    def on_stop(self):
        if self.ws:
            self.ws.close()

    def send_ws(self, event_type, payload):
        if not self.ws:
            return
        start = time.time()
        try:
            msg = json.dumps({"type": event_type, "id": str(uuid.uuid4()), "payload": payload})
            self.ws.send(msg)
            elapsed = (time.time() - start) * 1000
            self.environment.events.request.fire(
                request_type="WS",
                name=event_type,
                response_time=elapsed,
                response_length=len(msg),
                response=200,
            )
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            self.environment.events.request.fire(
                request_type="WS",
                name=event_type,
                response_time=elapsed,
                response_length=0,
                exception=e,
            )


class MessengerWsUser(WebSocketUser):
    wait_time = between(1, 3)

    @task(3)
    def typing_start(self):
        chat_id = str(uuid.uuid4())
        self.chat_ids.append(chat_id)
        self.send_ws("typing.start", {"chat_id": chat_id})

    @task(2)
    def typing_stop(self):
        if self.chat_ids:
            chat_id = random.choice(self.chat_ids)
            self.send_ws("typing.stop", {"chat_id": chat_id})

    @task(2)
    def ack(self):
        self.send_ws("ack", {"event_id": str(uuid.uuid4())})

    @task(1)
    def ping(self):
        self.send_ws("ping", {"client_time": time.strftime("%Y-%m-%dT%H:%M:%SZ")})
