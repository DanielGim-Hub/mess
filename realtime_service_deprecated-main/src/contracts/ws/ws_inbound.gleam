import contracts/ws/ws_error.{
  type WsDecodeError, InvalidEnvelope, InvalidPayload, UnknownMessageType,
}
import gleam/dynamic.{type Dynamic}
import gleam/dynamic/decode
import gleam/json
import gleam/result
import tempo.{type DateTime, ISO8601Seconds}
import tempo/datetime
import youid/uuid.{type Uuid}

pub type InboundMessage {
  AckMessage(id: String, event_id: String)
  TypingStartMessage(id: String, chat_id: Uuid)
  TypingStopMessage(id: String, chat_id: Uuid)
  PingMessage(id: String, client_time: DateTime)
}

type RawInboundEnvelope {
  RawInboundEnvelope(message_type: String, id: String, payload: Dynamic)
}

const message_type_ack = "ack"

const message_type_typing_start = "typing.start"

const message_type_typing_stop = "typing.stop"

const message_type_ping = "ping"

const payload_field_event_id = "event_id"

const payload_field_chat_id = "chat_id"

const payload_field_client_time = "client_time"

pub fn decode(message: String) -> Result(InboundMessage, WsDecodeError) {
  use envelope <- result.try(
    json.parse(message, envelope_decoder())
    |> result.map_error(InvalidEnvelope),
  )
  decode_envelope(envelope)
}

fn decode_envelope(
  envelope: RawInboundEnvelope,
) -> Result(InboundMessage, WsDecodeError) {
  let id = envelope.id
  case envelope.message_type {
    message_type if message_type == message_type_ack ->
      decode_ack(id, envelope.payload)
    message_type if message_type == message_type_typing_start ->
      decode_typing_start(id, envelope.payload)
    message_type if message_type == message_type_typing_stop ->
      decode_typing_stop(id, envelope.payload)
    message_type if message_type == message_type_ping ->
      decode_ping(id, envelope.payload)
    unknown -> Error(UnknownMessageType(unknown))
  }
}

fn decode_ack(
  id: String,
  payload: Dynamic,
) -> Result(InboundMessage, WsDecodeError) {
  use event_id <- result.try(payload_field(
    payload,
    message_type_ack,
    payload_field_event_id,
  ))
  Ok(AckMessage(id: id, event_id: event_id))
}

fn decode_typing_start(
  id: String,
  payload: Dynamic,
) -> Result(InboundMessage, WsDecodeError) {
  use chat_id <- result.try(payload_uuid_field(
    payload,
    message_type_typing_start,
    payload_field_chat_id,
  ))
  Ok(TypingStartMessage(id: id, chat_id: chat_id))
}

fn decode_typing_stop(
  id: String,
  payload: Dynamic,
) -> Result(InboundMessage, WsDecodeError) {
  use chat_id <- result.try(payload_uuid_field(
    payload,
    message_type_typing_stop,
    payload_field_chat_id,
  ))
  Ok(TypingStopMessage(id: id, chat_id: chat_id))
}

fn decode_ping(
  id: String,
  payload: Dynamic,
) -> Result(InboundMessage, WsDecodeError) {
  use client_time <- result.try(payload_datetime_field(
    payload,
    message_type_ping,
    payload_field_client_time,
  ))
  Ok(PingMessage(id: id, client_time: client_time))
}

fn envelope_decoder() -> decode.Decoder(RawInboundEnvelope) {
  use message_type <- decode.field("type", decode.string)
  use id <- decode.field("id", decode.string)
  use payload <- decode.field("payload", decode.dynamic)
  decode.success(RawInboundEnvelope(message_type:, id:, payload:))
}

fn payload_field(
  payload: Dynamic,
  message_type: String,
  field: String,
) -> Result(String, WsDecodeError) {
  decode.run(payload, decode.field(field, decode.string, decode.success))
  |> result.map_error(fn(errors) { InvalidPayload(message_type:, errors:) })
}

fn payload_uuid_field(
  payload: Dynamic,
  message_type: String,
  field: String,
) -> Result(Uuid, WsDecodeError) {
  use raw <- result.try(payload_field(payload, message_type, field))
  parse_uuid(raw, message_type)
}

fn payload_datetime_field(
  payload: Dynamic,
  message_type: String,
  field: String,
) -> Result(DateTime, WsDecodeError) {
  use raw <- result.try(payload_field(payload, message_type, field))
  parse_datetime(raw, message_type)
}

fn parse_uuid(raw: String, message_type: String) -> Result(Uuid, WsDecodeError) {
  uuid.from_string(raw)
  |> result.map_error(fn(_) { InvalidPayload(message_type:, errors: []) })
}

fn parse_datetime(
  raw: String,
  message_type: String,
) -> Result(DateTime, WsDecodeError) {
  datetime.parse(raw, ISO8601Seconds)
  |> result.map_error(fn(_) { InvalidPayload(message_type:, errors: []) })
}
