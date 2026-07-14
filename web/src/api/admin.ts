import http from "./http";

export interface UserInfo {
  id: string;
  username: string;
  role: string;
}

export interface ProviderProfile {
  id?: string;
  kind: string;
  name?: string;
  active?: boolean;
  provider: string;
  base_url: string;
  model_name: string;
  api_key_masked?: string;
  has_api_key?: boolean;
  temperature?: number;
  max_tokens?: number;
  dimensions?: number;
  updated_at?: string;
}

export function login(username: string, password: string) {
  return http.post<{ token: string; user: UserInfo }>("/auth/login", {
    username,
    password,
  });
}

export function fetchMe() {
  return http.get<UserInfo>("/auth/me");
}

export function listQA(params: Record<string, any>) {
  return http.get("/knowledge/qa", { params });
}

export function getQA(id: string) {
  return http.get(`/knowledge/qa/${id}`);
}

export function createQA(data: any) {
  return http.post("/knowledge/qa", data);
}

export function updateQA(id: string, data: any) {
  return http.put(`/knowledge/qa/${id}`, data);
}

export function deleteQA(id: string) {
  return http.delete(`/knowledge/qa/${id}`);
}

export function reclassifyQA(id: string) {
  return http.post(`/knowledge/qa/${id}/reclassify`);
}

export interface QAImportResult {
  imported: number;
  failed: number;
  errors?: string[];
}

export function importQA(file: File) {
  const form = new FormData();
  form.append("file", file);
  return http.post<QAImportResult>("/knowledge/qa/import", form, {
    headers: { "Content-Type": "multipart/form-data" },
  });
}

export function listCategories() {
  return http.get<string[]>("/knowledge/qa/categories");
}

export function listTags() {
  return http.get<string[]>("/knowledge/qa/tags");
}

export function getProvider(kind: "chat" | "embedding") {
  return http.get(`/providers/${kind}`);
}

export function updateProvider(kind: "chat" | "embedding", data: any) {
  return http.put(`/providers/${kind}`, data);
}

export function listProviderProfiles(kind: "chat" | "embedding") {
  return http.get<ProviderProfile[]>(`/providers/${kind}/profiles`);
}

export function createProviderProfile(
  kind: "chat" | "embedding",
  data: Partial<ProviderProfile> & { api_key?: string },
) {
  return http.post<ProviderProfile>(`/providers/${kind}/profiles`, data);
}

export function updateProviderProfile(
  kind: "chat" | "embedding",
  id: string,
  data: Partial<ProviderProfile> & { api_key?: string },
) {
  return http.put<ProviderProfile>(`/providers/${kind}/profiles/${id}`, data);
}

export function activateProviderProfile(
  kind: "chat" | "embedding",
  id: string,
) {
  return http.put(`/providers/${kind}/profiles/${id}/activate`, {});
}

export function deleteProviderProfile(kind: "chat" | "embedding", id: string) {
  return http.delete(`/providers/${kind}/profiles/${id}`);
}

export interface AgentConfig {
  key: string;
  default_reply: string;
  max_context_chars: number;
  max_context_messages: number;
  updated_at?: string;
}

export function getAgentConfig() {
  return http.get<AgentConfig>("/agent-config");
}

export function updateAgentConfig(data: Partial<AgentConfig>) {
  return http.put<AgentConfig>("/agent-config", data);
}

export interface ChatLightRule {
  id: string;
  name: string;
  enabled: boolean;
  priority: number;
  match_type: "exact" | "contains" | "regex" | "keywords";
  pattern: string;
  action_type: "reply" | "handoff" | "reject" | "continue_search";
  response_text?: string;
  created_at?: string;
  updated_at?: string;
}

export function listChatLightRules(enabledOnly = false) {
  return http.get<ChatLightRule[]>("/chat-light-rules", {
    params: enabledOnly ? { enabled: true } : {},
  });
}

export function createChatLightRule(
  data: Omit<ChatLightRule, "id" | "created_at" | "updated_at">,
) {
  return http.post<ChatLightRule>("/chat-light-rules", data);
}

export function updateChatLightRule(
  id: string,
  data: Omit<ChatLightRule, "id" | "created_at" | "updated_at">,
) {
  return http.put<ChatLightRule>(`/chat-light-rules/${id}`, data);
}

export function deleteChatLightRule(id: string) {
  return http.delete(`/chat-light-rules/${id}`);
}

export interface AIErrorLog {
  id: string;
  source: string;
  stage: string;
  message: string;
  request_id?: string;
  session_id?: string;
  details?: string;
  resolved: boolean;
  created_at: string;
  resolved_at?: string;
}

export function listAIErrors() {
  return http.get<AIErrorLog[]>("/ai-errors");
}

export function getAIErrorUnresolvedCount() {
  return http.get<{ count: number }>("/ai-errors/unresolved-count");
}

export function resolveAIError(id: string) {
  return http.put(`/ai-errors/${id}/resolve`, {});
}

export function listPrompts() {
  return http.get("/prompts");
}

export function updatePrompt(key: string, content: string) {
  return http.put(`/prompts/${key}`, { content });
}

export function listUsers() {
  return http.get("/users");
}

export function createUser(data: any) {
  return http.post("/users", data);
}

export function updateUser(id: string, data: any) {
  return http.put(`/users/${id}`, data);
}

export function deleteUser(id: string) {
  return http.delete(`/users/${id}`);
}

export function listCallerTokens() {
  return http.get("/caller-tokens");
}

export function createCallerToken(data: any) {
  return http.post("/caller-tokens", data);
}

export function updateCallerToken(id: string, data: any) {
  return http.put(`/caller-tokens/${id}`, data);
}

export function deleteCallerToken(id: string) {
  return http.delete(`/caller-tokens/${id}`);
}

export function listSessions(params: Record<string, any>) {
  return http.get("/sessions", { params });
}

export function getSession(id: string) {
  return http.get(`/sessions/${id}`);
}

export function listUnansweredQuestions(params: Record<string, any>) {
  return http.get("/unanswered-questions", { params });
}
export function getUnansweredStats() {
  return http.get("/unanswered-questions/stats");
}
export function listUnansweredEvents(id: string) {
  return http.get(`/unanswered-questions/${id}/events`);
}
export function updateUnansweredQuestion(id: string, data: any) {
  return http.put(`/unanswered-questions/${id}`, data);
}
export function convertUnansweredQuestion(id: string) {
  return http.post(`/unanswered-questions/${id}/convert`);
}
export function deleteUnansweredQuestion(id: string) {
  return http.delete(`/unanswered-questions/${id}`);
}

/** GM 测试客服 playground */
export function createPlaygroundSession() {
  return http.post<{
    id: string;
    status?: string;
    created_at: string;
    updated_at: string;
  }>("/playground/sessions");
}

export function getPlaygroundHistory(id: string) {
  return http.get(`/playground/sessions/${id}/history`);
}

export function closePlaygroundSession(id: string) {
  return http.delete(`/playground/sessions/${id}`);
}
