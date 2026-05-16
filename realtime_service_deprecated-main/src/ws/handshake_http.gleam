import gleam/http/response as http_response
import gleam/json
import infra/auth.{
  InvalidIssuer, InvalidRoles, InvalidSignature, InvalidUserId, MalformedToken,
  MissingClaim, NoMatchingKey, TokenExpired, TokenNotYetValid,
  UnsupportedTokenType,
}
import ws/handshake.{
  type HandshakeError, AuthenticationFailed, TokenExtractionFailed,
}
import ws/token.{InvalidAuthorizationHeader, MissingToken}

pub fn unauthorized_response(
  error: HandshakeError,
) -> http_response.Response(String) {
  let #(message, detail) = error_message(error)

  http_response.new(401)
  |> http_response.set_header("content-type", "application/json; charset=utf-8")
  |> http_response.set_body(
    json.object([
      #("code", json.string("unauthorized")),
      #("message", json.string(message)),
      #("detail", json.string(detail)),
    ])
    |> json.to_string,
  )
}

fn error_message(error: HandshakeError) -> #(String, String) {
  case error {
    TokenExtractionFailed(MissingToken) -> #(
      "Missing authentication token",
      "Provide JWT in ?token=... or Authorization: Bearer <token>",
    )

    TokenExtractionFailed(InvalidAuthorizationHeader) -> #(
      "Invalid authorization header",
      "Expected Authorization: Bearer <token>",
    )

    AuthenticationFailed(MalformedToken) -> #("Invalid token", "Malformed JWT")

    AuthenticationFailed(InvalidSignature) -> #(
      "Invalid token",
      "Signature verification failed",
    )

    AuthenticationFailed(TokenExpired(_)) -> #(
      "Token expired",
      "Refresh access token and reconnect",
    )

    AuthenticationFailed(TokenNotYetValid(_)) -> #(
      "Invalid token",
      "Token is not valid yet",
    )

    AuthenticationFailed(UnsupportedTokenType) -> #(
      "Invalid token type",
      "WebSocket handshake requires access token",
    )

    AuthenticationFailed(InvalidUserId) -> #(
      "Invalid token",
      "Subject claim is not a valid UUID",
    )

    AuthenticationFailed(InvalidRoles) -> #(
      "Invalid token",
      "Roles claim is invalid",
    )

    AuthenticationFailed(InvalidIssuer) -> #(
      "Invalid token",
      "Issuer claim does not match expected issuer",
    )

    AuthenticationFailed(MissingClaim(claim_name)) -> #(
      "Invalid token",
      "Missing required claim: " <> claim_name,
    )

    AuthenticationFailed(NoMatchingKey) -> #(
      "Invalid token",
      "No matching verification key found",
    )

    AuthenticationFailed(_) -> #("Unauthorized", "Token validation failed")
  }
}
