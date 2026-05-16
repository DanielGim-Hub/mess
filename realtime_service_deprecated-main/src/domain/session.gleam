import youid/uuid.{type Uuid}

pub type Session {
  Session(
    user_id: Uuid,
    connection_id: String,
    node_id: String,
    connected_at: String,
  )
}

pub fn session_key(user_id: String) -> String {
  "gw:session:" <> user_id
}

pub fn conn_key(connection_id: String) -> String {
  "gw:conn:" <> connection_id
}
