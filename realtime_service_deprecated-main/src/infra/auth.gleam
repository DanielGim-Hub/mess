import envie
import envie/decode as env_decode
import envie/error.{DecodeError, InvalidValue, Missing}
import gleam/dynamic/decode
import gleam/http/request as http_request
import gleam/http/response.{Response}
import gleam/httpc
import gleam/json
import gleam/list
import gleam/option.{type Option, None, Some}
import gleam/result
import gleam/time/duration
import gleam/time/timestamp.{type Timestamp}
import gleam/uri
import youid/uuid.{type Uuid, from_string}
import ywt.{type ParseError, decode as decode_token}
import ywt/claim.{
  type Claim, custom, expires_at, issuer as claim_issuer, numeric_date_decoder,
  typ,
}
import ywt/verify_key.{type VerifyKey, set_decoder}

pub type AuthConfig {
  AuthConfig(
    jwks_url: String,
    issuer: Option(String),
    leeway_seconds: Int,
    http_timeout_ms: Int,
  )
}

pub type AuthenticatedAccessToken {
  AuthenticatedAccessToken(
    user_id: Uuid,
    roles: List(String),
    token_id: String,
    expires_at: Timestamp,
  )
}

type AccessClaims {
  AccessClaims(
    subject: String,
    expires_at: Timestamp,
    issued_at: Option(Timestamp),
    token_id: Option(String),
    roles: List(String),
    token_type: String,
    issuer: Option(String),
  )
}

pub type AuthError {
  MissingConfig(field: String)
  InvalidConfig(field: String, message: String)
  InvalidJwksUrl
  JwksHttpError(httpc.HttpError)
  JwksUnexpectedStatus(status: Int)
  JwksDecodeError(json.DecodeError)
  InvalidToken
  MalformedToken
  InvalidSignature
  NoMatchingKey
  TokenExpired(expires_at: Timestamp)
  TokenNotYetValid(not_before: Timestamp)
  InvalidIssuer
  UnsupportedTokenType
  MissingClaim(claim_name: String)
  InvalidClaims
  InvalidUserId
  InvalidRoles
}

const jwks_url_env = "AUTH_JWKS_URL"

const issuer_env = "AUTH_JWT_ISSUER"

const leeway_env = "AUTH_JWT_LEEWAY_SECONDS"

const timeout_env = "AUTH_JWKS_TIMEOUT_MS"

const default_leeway_seconds = 5

const default_http_timeout_ms = 5000

const allowed_access_roles = ["user", "admin", "moderator"]

pub fn load_config() -> Result(AuthConfig, AuthError) {
  use jwks_url <- result.try(load_jwks_url())
  use issuer <- result.try(load_optional_issuer())

  Ok(AuthConfig(
    jwks_url: jwks_url,
    issuer: issuer,
    leeway_seconds: envie.get_int(leeway_env, default_leeway_seconds),
    http_timeout_ms: envie.get_int(timeout_env, default_http_timeout_ms),
  ))
}

pub fn fetch_jwks(config: AuthConfig) -> Result(List(VerifyKey), AuthError) {
  use request <- result.try(
    http_request.to(config.jwks_url)
    |> result.replace_error(InvalidJwksUrl),
  )

  let request =
    request
    |> http_request.set_header("accept", "application/json")

  let http_config =
    httpc.configure()
    |> httpc.timeout(config.http_timeout_ms)
    |> httpc.follow_redirects(True)

  use response <- result.try(
    httpc.dispatch(http_config, request)
    |> result.map_error(JwksHttpError),
  )

  let Response(status:, body:, ..) = response
  case status {
    200 ->
      json.parse(body, using: set_decoder())
      |> result.map_error(JwksDecodeError)

    _ -> Error(JwksUnexpectedStatus(status))
  }
}

pub fn validate_access_token(
  token: String,
  keys: List(VerifyKey),
  config: AuthConfig,
) -> Result(AuthenticatedAccessToken, AuthError) {
  let claims = build_claims(config)

  use payload <- result.try(
    decode_token(
      token,
      using: access_claims_decoder(),
      claims: claims,
      keys: keys,
    )
    |> result.map_error(map_parse_error),
  )

  use user_id <- result.try(
    from_string(payload.subject)
    |> result.replace_error(InvalidUserId),
  )

  use roles <- result.try(validate_roles(payload.roles))
  use token_id <- result.try(require_token_id(payload.token_id))
  use _issued_at <- result.try(require_issued_at(payload.issued_at))

  Ok(AuthenticatedAccessToken(
    user_id: user_id,
    roles: roles,
    token_id: token_id,
    expires_at: payload.expires_at,
  ))
}

pub fn authenticate(
  token: String,
) -> Result(AuthenticatedAccessToken, AuthError) {
  use config <- result.try(load_config())
  use keys <- result.try(fetch_jwks(config))

  case validate_access_token(token, keys, config) {
    Error(NoMatchingKey) -> {
      use refreshed_keys <- result.try(fetch_jwks(config))
      validate_access_token(token, refreshed_keys, config)
    }

    result -> result
  }
}

fn load_jwks_url() -> Result(String, AuthError) {
  case envie.require(jwks_url_env, env_decode.web_url()) {
    Ok(url) -> Ok(uri.to_string(url))
    Error(Missing(_)) -> Error(MissingConfig(jwks_url_env))
    Error(InvalidValue(reason: reason, ..)) ->
      Error(InvalidConfig(jwks_url_env, reason))
    Error(DecodeError(details: details, ..)) ->
      Error(InvalidConfig(jwks_url_env, details))
  }
}

fn load_optional_issuer() -> Result(Option(String), AuthError) {
  case envie.optional(issuer_env, env_decode.non_empty_string()) {
    Ok(issuer) -> Ok(issuer)
    Error(InvalidValue(reason: reason, ..)) ->
      Error(InvalidConfig(issuer_env, reason))
    Error(Missing(_)) -> Ok(None)
    Error(DecodeError(details: details, ..)) ->
      Error(InvalidConfig(issuer_env, details))
  }
}

fn build_claims(config: AuthConfig) -> List(Claim) {
  let base_claims = [
    typ("JWT"),
    expires_at(
      max_age: duration.hours(1),
      leeway: duration.seconds(config.leeway_seconds),
    ),
    custom(
      name: "type",
      value: "access",
      encode: json.string,
      decoder: decode.string,
    ),
  ]

  case config.issuer {
    Some(issuer) -> [claim_issuer(issuer, []), ..base_claims]
    None -> base_claims
  }
}

fn access_claims_decoder() -> decode.Decoder(AccessClaims) {
  use subject <- decode.field("sub", decode.string)
  use expires_at <- decode.field("exp", numeric_date_decoder())
  use issued_at <- decode.optional_field(
    "iat",
    None,
    decode.map(numeric_date_decoder(), Some),
  )
  use token_id <- decode.optional_field(
    "jti",
    None,
    decode.map(decode.string, Some),
  )
  use roles <- decode.field("roles", decode.list(decode.string))
  use token_type <- decode.field("type", decode.string)
  use issuer <- decode.optional_field(
    "iss",
    None,
    decode.map(decode.string, Some),
  )

  decode.success(AccessClaims(
    subject: subject,
    expires_at: expires_at,
    issued_at: issued_at,
    token_id: token_id,
    roles: roles,
    token_type: token_type,
    issuer: issuer,
  ))
}

fn validate_roles(roles: List(String)) -> Result(List(String), AuthError) {
  case roles {
    [] -> Error(InvalidRoles)
    _ ->
      case
        list.all(roles, fn(role) { list.contains(allowed_access_roles, role) })
      {
        True -> Ok(roles)
        False -> Error(InvalidRoles)
      }
  }
}

fn require_token_id(token_id: Option(String)) -> Result(String, AuthError) {
  case token_id {
    Some(token_id) -> Ok(token_id)
    None -> Error(MissingClaim("jti"))
  }
}

fn require_issued_at(
  issued_at: Option(Timestamp),
) -> Result(Timestamp, AuthError) {
  case issued_at {
    Some(issued_at) -> Ok(issued_at)
    None -> Error(MissingClaim("iat"))
  }
}

fn map_parse_error(error: ParseError) -> AuthError {
  case error {
    ywt.MalformedToken -> MalformedToken
    ywt.InvalidSignature -> InvalidSignature
    ywt.NoMatchingKey -> NoMatchingKey
    ywt.TokenExpired(expires_at) -> TokenExpired(expires_at)
    ywt.TokenNotYetValid(not_before) -> TokenNotYetValid(not_before)
    ywt.InvalidIssuer(..) -> InvalidIssuer
    ywt.MissingClaim(claim_name) -> MissingClaim(claim_name)
    ywt.InvalidCustomClaim(claim_name) if claim_name == "type" ->
      UnsupportedTokenType
    ywt.InvalidCustomClaim(..) -> InvalidClaims
    ywt.ClaimDecodingError(..) -> InvalidClaims
    ywt.PayloadDecodingError(..) -> InvalidClaims
    ywt.InvalidHeaderEncoding
    | ywt.InvalidPayloadEncoding
    | ywt.InvalidSignatureEncoding
    | ywt.InvalidHeaderJson(_)
    | ywt.InvalidPayloadJson(_)
    | ywt.InvalidAudience(..)
    | ywt.InvalidSubject(..)
    | ywt.InvalidId(..) -> InvalidToken
  }
}
