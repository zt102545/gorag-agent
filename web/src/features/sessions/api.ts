import http from '@/shared/api/http'
import type { SessionDetail, SessionListResult } from './types'

export function listSessions(params: Record<string, string | number>) {
  return http.get<SessionListResult>('/sessions', { params })
}

export function getSession(id: string) {
  return http.get<SessionDetail>(`/sessions/${id}`)
}
