import gleam/dynamic/decode
import gleam/json

pub type WsDecodeError {
  InvalidEnvelope(json.DecodeError)
  UnknownMessageType(String)
  InvalidPayload(message_type: String, errors: List(decode.DecodeError))
}
