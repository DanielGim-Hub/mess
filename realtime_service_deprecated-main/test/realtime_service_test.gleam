import contracts/ws/ws_inbound
import gleam/http/request
import gleam/http/response as http_response
import gleam/json
import gleam/list
import gleam/option.{None}
import gleam/time/timestamp
import gleeunit
import infra/auth
import ws/handshake.{TokenExtractionFailed}
import ws/handshake_http
import ws/token
import youid/uuid
import ywt
import ywt/algorithm
import ywt/claim
import ywt/verify_key

pub fn main() -> Nil {
  gleeunit.main()
}

pub fn token_from_query_test() {
  let request =
    request.new()
    |> request.set_query([#("token", "access-token")])

  let assert Ok("access-token") = token.from_request(request)
}

pub fn token_from_authorization_header_fallback_test() {
  let request =
    request.new()
    |> request.set_header("authorization", "Bearer access-token")

  let assert Ok("access-token") = token.from_request(request)
}

pub fn token_missing_test() {
  let request = request.new()

  let assert Error(token.MissingToken) = token.from_request(request)
}

pub fn token_invalid_authorization_header_test() {
  let request =
    request.new()
    |> request.set_header("authorization", "Token nope")

  let assert Error(token.InvalidAuthorizationHeader) =
    token.from_authorization_header(request)
}

pub fn validate_access_token_success_test() {
  let #(jwt, keys) = valid_access_token()
  let config = test_auth_config()

  let assert Ok(auth.AuthenticatedAccessToken(user_id:, roles:, token_id:, ..)) =
    auth.validate_access_token(jwt, keys, config)

  assert roles == ["user"]
  assert token_id == "550e8400-e29b-41d4-a716-446655440000"
  assert user_id
    |> uuid.to_string
    == "7b9c1d2e-3f4a-5b6c-7d8e-9f0a1b2c3d4e"
}

pub fn validate_access_token_rejects_refresh_token_test() {
  let #(jwt, keys) = token_with_overrides([#("type", json.string("refresh"))])
  let config = test_auth_config()

  let assert Error(auth.UnsupportedTokenType) =
    auth.validate_access_token(jwt, keys, config)
}

pub fn validate_access_token_requires_jti_and_iat_test() {
  let #(without_jti, keys) =
    token_with_payload([
      #("sub", json.string("7b9c1d2e-3f4a-5b6c-7d8e-9f0a1b2c3d4e")),
      #("iat", json.int(now_seconds())),
      #("exp", json.int(now_seconds() + 3600)),
      #("roles", json.array(["user"], json.string)),
      #("type", json.string("access")),
    ])
  let config = test_auth_config()

  let assert Error(auth.MissingClaim("jti")) =
    auth.validate_access_token(without_jti, keys, config)

  let #(without_iat, keys_again) =
    token_with_payload([
      #("sub", json.string("7b9c1d2e-3f4a-5b6c-7d8e-9f0a1b2c3d4e")),
      #("exp", json.int(now_seconds() + 3600)),
      #("jti", json.string("550e8400-e29b-41d4-a716-446655440000")),
      #("roles", json.array(["user"], json.string)),
      #("type", json.string("access")),
    ])

  let assert Error(auth.MissingClaim("iat")) =
    auth.validate_access_token(without_iat, keys_again, config)
}

pub fn validate_access_token_rejects_invalid_roles_test() {
  let #(jwt, keys) =
    token_with_overrides([
      #("roles", json.array(["intruder"], json.string)),
    ])
  let config = test_auth_config()

  let assert Error(auth.InvalidRoles) =
    auth.validate_access_token(jwt, keys, config)
}

pub fn ws_ack_decode_accepts_string_ids_test() {
  let message =
    "{\"type\":\"ack\",\"id\":\"msg_client_001\",\"payload\":{\"event_id\":\"evt_a1b2c3d4\"}}"

  let assert Ok(ws_inbound.AckMessage(
    id: "msg_client_001",
    event_id: "evt_a1b2c3d4",
  )) = ws_inbound.decode(message)
}

pub fn ws_ping_decode_accepts_string_id_test() {
  let message =
    "{\"type\":\"ping\",\"id\":\"msg_client_004\",\"payload\":{\"client_time\":\"2024-06-01T10:00:00Z\"}}"

  let assert Ok(ws_inbound.PingMessage(id: "msg_client_004", ..)) =
    ws_inbound.decode(message)
}

pub fn unauthorized_response_for_missing_token_test() {
  let http_response =
    handshake_http.unauthorized_response(TokenExtractionFailed(
      token.MissingToken,
    ))

  assert http_response.status == 401
  let assert Ok("application/json; charset=utf-8") =
    http_response.get_header(http_response, "content-type")
}

fn valid_access_token() -> #(String, List(verify_key.VerifyKey)) {
  token_with_payload(base_payload())
}

fn token_with_overrides(
  overrides: List(#(String, json.Json)),
) -> #(String, List(verify_key.VerifyKey)) {
  let payload =
    base_payload()
    |> apply_overrides(overrides)

  token_with_payload(payload)
}

fn token_with_payload(
  payload: List(#(String, json.Json)),
) -> #(String, List(verify_key.VerifyKey)) {
  let sign_key = ywt.generate_key(algorithm.rs256)
  let verify_key = verify_key.derived(sign_key)
  let jwt =
    ywt.encode(payload: payload, claims: [claim.typ("JWT")], key: sign_key)

  #(jwt, [verify_key])
}

fn base_payload() -> List(#(String, json.Json)) {
  [
    #("sub", json.string("7b9c1d2e-3f4a-5b6c-7d8e-9f0a1b2c3d4e")),
    #("iat", json.int(now_seconds())),
    #("exp", json.int(now_seconds() + 3600)),
    #("jti", json.string("550e8400-e29b-41d4-a716-446655440000")),
    #("roles", json.array(["user"], json.string)),
    #("type", json.string("access")),
  ]
}

fn apply_overrides(
  payload: List(#(String, json.Json)),
  overrides: List(#(String, json.Json)),
) -> List(#(String, json.Json)) {
  case overrides {
    [] -> payload
    [#(key, value), ..rest] ->
      apply_overrides(list.key_set(payload, key, value), rest)
  }
}

fn test_auth_config() -> auth.AuthConfig {
  auth.AuthConfig(
    jwks_url: "https://auth.example.com/.well-known/jwks.json",
    issuer: None,
    leeway_seconds: 5,
    http_timeout_ms: 1000,
  )
}

fn now_seconds() -> Int {
  let #(seconds, _nanoseconds) =
    timestamp.system_time()
    |> timestamp.to_unix_seconds_and_nanoseconds

  seconds
}
