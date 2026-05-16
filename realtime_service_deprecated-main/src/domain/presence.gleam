import youid/uuid.{type Uuid}

pub type PresenceStatus {
  Online
  Offline
}

pub type PresenceEvent {
  PresenceEvent(
    user_id: Uuid,
    status: PresenceStatus,
    node_id: String,
    timestamp: String,
  )
}

pub fn presence_key(user_id: String) -> String {
  "gw:presence:" <> user_id
}
