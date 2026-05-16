import dream/http/header
import dream/http/request.{type Request}
import dream/http/response.{type Response, json_response}
import dream/servers/mist/websocket
import gleam/option.{None, Some}
import gleam/result
import infra/auth.{authenticate as authenticate_token}
import infra/config
import infra/redis
import kafka/client as kafka
import ws/connection
import ws/handshake.{
  type HandshakeError, AuthenticationFailed, TokenExtractionFailed,
}
import ws/handshake_http
import ws/token

pub fn handle_upgrade(request: Request, _context, _services) -> Response {
  case authorize_request(request) {
    Ok(dependencies) ->
      websocket.upgrade_websocket(
        request,
        dependencies: dependencies,
        on_init: connection.on_init,
        on_message: connection.on_message,
        on_close: connection.on_close,
      )

    Error(error) -> unauthorized_response(error)
  }
}

fn authorize_request(
  request: Request,
) -> Result(connection.Dependencies, HandshakeError) {
  use access_token <- result.try(
    token.from_parts(
      query_token: request.get_query_param(request.query, "token"),
      authorization_header: header.get_header(request.headers, "authorization"),
    )
    |> result.map_error(TokenExtractionFailed),
  )

  use auth_token <- result.try(
    authenticate_token(access_token)
    |> result.map_error(AuthenticationFailed),
  )

  let cfg = config.load()

  let redis_client = case redis.connect(cfg.redis_url) {
    Ok(client) -> Some(client)
    Error(_) -> None
  }

  let kafka_producer = case kafka.new_producer(cfg.kafka_brokers) {
    Ok(producer) -> Some(producer)
    Error(_) -> None
  }

  Ok(connection.Dependencies(
    access_token: auth_token,
    redis: redis_client,
    kafka: kafka_producer,
    node_id: cfg.node_id,
  ))
}

fn unauthorized_response(error: HandshakeError) -> Response {
  let response = handshake_http.unauthorized_response(error)
  json_response(response.status, response.body)
}
