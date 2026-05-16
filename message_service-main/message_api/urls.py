from django.urls import path

from .views import (
    HealthLiveView,
    MessageDetailView,
    MessageListView,
    ReceiptView,
    SendMessageView,
)

urlpatterns = [
    path("health/live", HealthLiveView.as_view(), name="health-live"),
    path("chats/<uuid:chat_id>/messages", MessageListView.as_view(), name="message-list"),
    path("chats/<uuid:chat_id>/messages/<uuid:message_id>", MessageDetailView.as_view(), name="message-detail"),
    path("chats/<uuid:chat_id>/messages/<uuid:message_id>/receipts", ReceiptView.as_view(), name="message-receipts"),
]
