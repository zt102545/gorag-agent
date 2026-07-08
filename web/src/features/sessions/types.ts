export interface SessionListItem {
  id: string
  title?: string
  external_user_id?: string
  channel_id?: string
  tenant_id?: string
  caller_token_id?: string
  status: string
  message_count: number
  last_message_role?: string
  last_message_preview?: string
  last_message_at?: string
  created_at: string
  updated_at: string
  meta?: Record<string, unknown>
}

export interface SessionMessage {
  id: string
  role: string
  content: string
  request_id?: string
  client_message_id?: string
  extra?: Record<string, unknown>
  created_at: string
}

export interface SessionDetail {
  session: SessionListItem
  messages: SessionMessage[]
  message_count: number
}

export interface SessionListResult {
  total: number
  items: SessionListItem[]
}
