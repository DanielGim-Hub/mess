import gleam/json
import gleam/option.{type Option, None, Some}
import tempo.{type DateTime, ISO8601Seconds}
import tempo/datetime
import youid/uuid.{type Uuid}

pub type OutboundMessage {
  Connected(
    id: String,
    connection_id: String,
    user_id: Uuid,
    server_time: DateTime,
  )
  MessageNew(id: String, message: Message)
  MessageUpdated(id: String, message: Message)
  MessageDeleted(
    id: String,
    chat_id: Uuid,
    message_id: Uuid,
    sequence_number: Int,
    deleted_at: DateTime,
  )
  ReceiptUpdated(id: String, receipt: Receipt)
  PresenceUpdated(
    id: String,
    user_id: Uuid,
    status: PresenceStatus,
    last_seen_at: Option(DateTime),
  )
  ChatUpdated(
    id: String,
    chat_id: Uuid,
    updated_at: DateTime,
    changes: List(ChatChange),
  )
  TypingStarted(id: String, chat_id: Uuid, user_id: Uuid, started_at: DateTime)
  TypingStopped(id: String, chat_id: Uuid, user_id: Uuid)
  ErrorMessage(
    id: String,
    ref_id: Option(String),
    code: OutboundErrorCode,
    message: String,
  )
  Pong(id: String, server_time: DateTime)
}

pub type Message {
  Message(
    id: Uuid,
    chat_id: Uuid,
    sender_id: Uuid,
    content_type: ContentType,
    text: Option(String),
    attachment: Option(json.Json),
    reply_to_id: Option(Uuid),
    sequence_number: Int,
    is_edited: Bool,
    edited_at: Option(DateTime),
    created_at: DateTime,
  )
}

pub type ContentType {
  ContentTypeText
  ContentTypeImage
  ContentTypeFile
  ContentTypeAudio
  ContentTypeVideo
  ContentTypeCustom(String)
}

pub type Receipt {
  ReceiptDelivered(
    chat_id: Uuid,
    message_id: Uuid,
    user_id: Uuid,
    updated_at: DateTime,
  )
  ReceiptRead(
    chat_id: Uuid,
    user_id: Uuid,
    last_read_sequence_number: Int,
    updated_at: DateTime,
  )
}

pub type PresenceStatus {
  PresenceOnline
  PresenceOffline
}

pub type ChatChange {
  ChatTitleChanged
  ChatAvatarChanged
  ChatMembersChanged
  ChatChangeCustom(String)
}

pub type OutboundErrorCode {
  InvalidPayloadError
  UnknownTypeError
  UnauthorizedError
  RateLimitedError
  InternalError
  OutboundErrorCodeCustom(String)
}

const message_type_connected = "connected"

const message_type_message_new = "message.new"

const message_type_message_updated = "message.updated"

const message_type_message_deleted = "message.deleted"

const message_type_receipt_updated = "receipt.updated"

const message_type_presence_updated = "presence.updated"

const message_type_chat_updated = "chat.updated"

const message_type_typing_started = "typing.started"

const message_type_typing_stopped = "typing.stopped"

const message_type_error = "error"

const message_type_pong = "pong"

const receipt_status_delivered = "delivered"

const receipt_status_read = "read"

const presence_status_online = "online"

const presence_status_offline = "offline"

const chat_change_title = "title"

const chat_change_avatar = "avatar"

const chat_change_members = "members"

const error_code_invalid_payload = "invalid_payload"

const error_code_unknown_type = "unknown_type"

const error_code_unauthorized = "unauthorized"

const error_code_rate_limited = "rate_limited"

const error_code_internal = "internal"

pub fn encode(message: OutboundMessage) -> String {
  message
  |> to_json
  |> json.to_string
}

pub fn to_json(message: OutboundMessage) -> json.Json {
  case message {
    Connected(id, connection_id, user_id, server_time) ->
      envelope(
        message_type_connected,
        id,
        json.object([
          #("connection_id", json.string(connection_id)),
          #("user_id", encode_uuid(user_id)),
          #("server_time", encode_datetime(server_time)),
        ]),
      )

    MessageNew(id, message) ->
      envelope(
        message_type_message_new,
        id,
        json.object([#("message", encode_message(message))]),
      )

    MessageUpdated(id, message) ->
      envelope(
        message_type_message_updated,
        id,
        json.object([#("message", encode_message(message))]),
      )

    MessageDeleted(id, chat_id, message_id, sequence_number, deleted_at) ->
      envelope(
        message_type_message_deleted,
        id,
        json.object([
          #("chat_id", encode_uuid(chat_id)),
          #("message_id", encode_uuid(message_id)),
          #("sequence_number", json.int(sequence_number)),
          #("deleted_at", encode_datetime(deleted_at)),
        ]),
      )

    ReceiptUpdated(id, receipt) ->
      envelope(message_type_receipt_updated, id, encode_receipt(receipt))

    PresenceUpdated(id, user_id, status, last_seen_at) ->
      envelope(
        message_type_presence_updated,
        id,
        json.object([
          #("user_id", encode_uuid(user_id)),
          #("status", encode_presence_status(status)),
          #("last_seen_at", json.nullable(last_seen_at, encode_datetime)),
        ]),
      )

    ChatUpdated(id, chat_id, updated_at, changes) ->
      envelope(
        message_type_chat_updated,
        id,
        json.object([
          #("chat_id", encode_uuid(chat_id)),
          #("updated_at", encode_datetime(updated_at)),
          #("changes", json.array(changes, encode_chat_change)),
        ]),
      )

    TypingStarted(id, chat_id, user_id, started_at) ->
      envelope(
        message_type_typing_started,
        id,
        json.object([
          #("chat_id", encode_uuid(chat_id)),
          #("user_id", encode_uuid(user_id)),
          #("started_at", encode_datetime(started_at)),
        ]),
      )

    TypingStopped(id, chat_id, user_id) ->
      envelope(
        message_type_typing_stopped,
        id,
        json.object([
          #("chat_id", encode_uuid(chat_id)),
          #("user_id", encode_uuid(user_id)),
        ]),
      )

    ErrorMessage(id, ref_id, code, message) ->
      envelope(
        message_type_error,
        id,
        json.object([
          #("ref_id", json.nullable(ref_id, json.string)),
          #("code", encode_error_code(code)),
          #("message", json.string(message)),
        ]),
      )

    Pong(id, server_time) ->
      envelope(
        message_type_pong,
        id,
        json.object([#("server_time", encode_datetime(server_time))]),
      )
  }
}

fn envelope(message_type: String, id: String, payload: json.Json) -> json.Json {
  json.object([
    #("type", json.string(message_type)),
    #("id", json.string(id)),
    #("payload", payload),
  ])
}

fn encode_message(message: Message) -> json.Json {
  let Message(
    id: id,
    chat_id: chat_id,
    sender_id: sender_id,
    content_type: content_type,
    text: text,
    attachment: attachment,
    reply_to_id: reply_to_id,
    sequence_number: sequence_number,
    is_edited: is_edited,
    edited_at: edited_at,
    created_at: created_at,
  ) = message

  json.object([
    #("id", encode_uuid(id)),
    #("chat_id", encode_uuid(chat_id)),
    #("sender_id", encode_uuid(sender_id)),
    #("content_type", encode_content_type(content_type)),
    #("text", json.nullable(text, json.string)),
    #("attachment", json.nullable(attachment, fn(item) { item })),
    #("reply_to_id", json.nullable(reply_to_id, encode_uuid)),
    #("sequence_number", json.int(sequence_number)),
    #("is_edited", json.bool(is_edited)),
    #("created_at", encode_datetime(created_at)),
    ..optional_datetime_field("edited_at", edited_at)
  ])
}

fn optional_datetime_field(
  key: String,
  value: Option(DateTime),
) -> List(#(String, json.Json)) {
  case value {
    Some(datetime) -> [#(key, encode_datetime(datetime))]
    None -> []
  }
}

fn encode_receipt(receipt: Receipt) -> json.Json {
  case receipt {
    ReceiptDelivered(chat_id, message_id, user_id, updated_at) ->
      json.object([
        #("chat_id", encode_uuid(chat_id)),
        #("message_id", encode_uuid(message_id)),
        #("user_id", encode_uuid(user_id)),
        #("status", json.string(receipt_status_delivered)),
        #("updated_at", encode_datetime(updated_at)),
      ])

    ReceiptRead(chat_id, user_id, last_read_sequence_number, updated_at) ->
      json.object([
        #("chat_id", encode_uuid(chat_id)),
        #("message_id", json.null()),
        #("user_id", encode_uuid(user_id)),
        #("status", json.string(receipt_status_read)),
        #("last_read_sequence_number", json.int(last_read_sequence_number)),
        #("updated_at", encode_datetime(updated_at)),
      ])
  }
}

fn encode_content_type(content_type: ContentType) -> json.Json {
  case content_type {
    ContentTypeText -> json.string("text")
    ContentTypeImage -> json.string("image")
    ContentTypeFile -> json.string("file")
    ContentTypeAudio -> json.string("audio")
    ContentTypeVideo -> json.string("video")
    ContentTypeCustom(value) -> json.string(value)
  }
}

fn encode_presence_status(status: PresenceStatus) -> json.Json {
  case status {
    PresenceOnline -> json.string(presence_status_online)
    PresenceOffline -> json.string(presence_status_offline)
  }
}

fn encode_chat_change(change: ChatChange) -> json.Json {
  case change {
    ChatTitleChanged -> json.string(chat_change_title)
    ChatAvatarChanged -> json.string(chat_change_avatar)
    ChatMembersChanged -> json.string(chat_change_members)
    ChatChangeCustom(value) -> json.string(value)
  }
}

fn encode_error_code(code: OutboundErrorCode) -> json.Json {
  case code {
    InvalidPayloadError -> json.string(error_code_invalid_payload)
    UnknownTypeError -> json.string(error_code_unknown_type)
    UnauthorizedError -> json.string(error_code_unauthorized)
    RateLimitedError -> json.string(error_code_rate_limited)
    InternalError -> json.string(error_code_internal)
    OutboundErrorCodeCustom(value) -> json.string(value)
  }
}

fn encode_uuid(value: Uuid) -> json.Json {
  value
  |> uuid.to_string
  |> json.string
}

fn encode_datetime(value: DateTime) -> json.Json {
  value
  |> datetime.format(ISO8601Seconds)
  |> json.string
}
