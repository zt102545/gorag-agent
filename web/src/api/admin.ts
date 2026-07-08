import http from './http'

export interface UserInfo {
  id: string
  username: string
  role: string
}

export function login(username: string, password: string) {
  return http.post<{ token: string; user: UserInfo }>('/auth/login', { username, password })
}

export function fetchMe() {
  return http.get<UserInfo>('/auth/me')
}

export function listQA(params: Record<string, any>) {
  return http.get('/knowledge/qa', { params })
}

export function getQA(id: string) {
  return http.get(`/knowledge/qa/${id}`)
}

export function createQA(data: any) {
  return http.post('/knowledge/qa', data)
}

export function updateQA(id: string, data: any) {
  return http.put(`/knowledge/qa/${id}`, data)
}

export function deleteQA(id: string) {
  return http.delete(`/knowledge/qa/${id}`)
}

export function reclassifyQA(id: string) {
  return http.post(`/knowledge/qa/${id}/reclassify`)
}

export interface QAImportResult {
  imported: number
  failed: number
  errors?: string[]
}

export function importQA(file: File) {
  const form = new FormData()
  form.append('file', file)
  return http.post<QAImportResult>('/knowledge/qa/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export function listCategories() {
  return http.get<string[]>('/knowledge/qa/categories')
}

export function listTags() {
  return http.get<string[]>('/knowledge/qa/tags')
}

export function getProvider(kind: 'chat' | 'embedding') {
  return http.get(`/providers/${kind}`)
}

export function updateProvider(kind: 'chat' | 'embedding', data: any) {
  return http.put(`/providers/${kind}`, data)
}

export function listPrompts() {
  return http.get('/prompts')
}

export function updatePrompt(key: string, content: string) {
  return http.put(`/prompts/${key}`, { content })
}

export function listUsers() {
  return http.get('/users')
}

export function createUser(data: any) {
  return http.post('/users', data)
}

export function updateUser(id: string, data: any) {
  return http.put(`/users/${id}`, data)
}

export function deleteUser(id: string) {
  return http.delete(`/users/${id}`)
}

export function listAPIKeys() {
  return http.get('/api-keys')
}

export function createAPIKey(data: any) {
  return http.post('/api-keys', data)
}

export function setAPIKeyEnabled(id: string, enabled: boolean) {
  return http.put(`/api-keys/${id}`, { enabled })
}

export function deleteAPIKey(id: string) {
  return http.delete(`/api-keys/${id}`)
}

export function listCallerTokens() {
  return http.get('/caller-tokens')
}

export function createCallerToken(data: any) {
  return http.post('/caller-tokens', data)
}

export function updateCallerToken(id: string, data: any) {
  return http.put(`/caller-tokens/${id}`, data)
}

export function deleteCallerToken(id: string) {
  return http.delete(`/caller-tokens/${id}`)
}

export function listSessions(params: Record<string, any>) {
  return http.get('/sessions', { params })
}

export function getSession(id: string) {
  return http.get(`/sessions/${id}`)
}

/** GM 测试客服 playground */
export function createPlaygroundSession() {
  return http.post<{ id: string; status?: string; created_at: string; updated_at: string }>(
    '/playground/sessions',
  )
}

export function getPlaygroundHistory(id: string) {
  return http.get(`/playground/sessions/${id}/history`)
}

export function closePlaygroundSession(id: string) {
  return http.delete(`/playground/sessions/${id}`)
}
