import gleam/http/request.{type Request}
import gleam/list
import gleam/option.{type Option, None, Some}
import gleam/result
import gleam/string

pub type TokenExtractionError {
  MissingToken
  InvalidAuthorizationHeader
}

pub fn from_request(
  request: Request(body),
) -> Result(String, TokenExtractionError) {
  from_parts(
    query_token: from_query(request) |> result_to_option,
    authorization_header: request.get_header(request, "authorization")
      |> result_to_option,
  )
}

pub fn from_parts(
  query_token query_token: Option(String),
  authorization_header authorization_header: Option(String),
) -> Result(String, TokenExtractionError) {
  case query_token {
    Some(token) ->
      case non_empty_option(token) {
        Some(token) -> Ok(token)
        None -> from_authorization_header_value(authorization_header)
      }

    None -> from_authorization_header_value(authorization_header)
  }
}

pub fn from_query(request: Request(body)) -> Result(String, Nil) {
  use query <- result.try(request.get_query(request))
  use token <- result.try(list.key_find(query, "token"))
  non_empty(token)
}

pub fn from_authorization_header(
  request: Request(body),
) -> Result(String, TokenExtractionError) {
  from_authorization_header_value(
    request.get_header(request, "authorization") |> result_to_option,
  )
}

pub fn from_authorization_header_value(
  authorization_header: Option(String),
) -> Result(String, TokenExtractionError) {
  case authorization_header {
    None -> Error(MissingToken)
    Some("Bearer " <> token) -> parse_bearer_token(token)
    Some("bearer " <> token) -> parse_bearer_token(token)
    _ -> Error(InvalidAuthorizationHeader)
  }
}

fn parse_bearer_token(token: String) -> Result(String, TokenExtractionError) {
  token
  |> string.trim
  |> non_empty
  |> result.replace_error(InvalidAuthorizationHeader)
}

fn non_empty(value: String) -> Result(String, Nil) {
  case string.trim(value) {
    "" -> Error(Nil)
    trimmed -> Ok(trimmed)
  }
}

fn non_empty_option(value: String) -> Option(String) {
  non_empty(value) |> result_to_option
}

fn result_to_option(result: Result(a, e)) -> Option(a) {
  case result {
    Ok(value) -> Some(value)
    Error(_) -> None
  }
}
