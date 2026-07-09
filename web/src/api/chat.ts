export interface SSEEvent {
  type: string
  agent_name?: string
  content?: string
  action_type?: string
  interrupt_id?: string
  error?: string
}

type EventHandler = (ev: SSEEvent) => void

async function readSSE(res: Response, onEvent: EventHandler) {
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const body = await res.json()
      msg = body.message || msg
    } catch {
      /* ignore */
    }
    throw new Error(msg)
  }
  if (!res.body) {
    throw new Error('浏览器不支持流式读取')
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split('\n\n')
    buffer = parts.pop() || ''
    for (const part of parts) {
      const lines = part.split('\n')
      for (const line of lines) {
        const trimmed = line.trim()
        if (!trimmed.startsWith('data:')) continue
        const payload = trimmed.slice(5).trim()
        if (!payload || payload === '[DONE]') continue
        try {
          onEvent(JSON.parse(payload) as SSEEvent)
        } catch {
          /* skip malformed chunk */
        }
      }
    }
  }
}

function authHeaders(): HeadersInit {
  const token = localStorage.getItem('gm_token') || ''
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  }
}

interface CallerContext {
  token: string
  callerUserId?: string
  channelId?: string
  tenantId?: string
}

function callerHeaders(ctx: CallerContext): HeadersInit {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Caller-Token': ctx.token,
  }
  if (ctx.callerUserId) headers['X-Caller-User-Id'] = ctx.callerUserId
  if (ctx.channelId) headers['X-Channel-Id'] = ctx.channelId
  if (ctx.tenantId) headers['X-Tenant-Id'] = ctx.tenantId
  return headers
}

export async function streamPlaygroundChat(sessionId: string, message: string, onEvent: EventHandler) {
  const res = await fetch(`/api/v1/admin/playground/sessions/${sessionId}/chat`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ message }),
  })
  await readSSE(res, onEvent)
}

export async function streamPlaygroundApprove(
  sessionId: string,
  interruptId: string,
  approved: boolean,
  onEvent: EventHandler,
) {
  const res = await fetch(`/api/v1/admin/playground/sessions/${sessionId}/approve`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ interrupt_id: interruptId, approved }),
  })
  await readSSE(res, onEvent)
}

export async function createBizSession(ctx: CallerContext, payload: Record<string, unknown>) {
  const res = await fetch('/api/v1/biz/sessions', {
    method: 'POST',
    headers: callerHeaders(ctx),
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw new Error(await readError(res))
  return readData(res)
}

export async function getBizSession(ctx: CallerContext, sessionId: string) {
  const res = await fetch(`/api/v1/biz/sessions/${sessionId}`, {
    headers: callerHeaders(ctx),
  })
  if (!res.ok) throw new Error(await readError(res))
  return readData(res)
}

export async function getBizHistory(ctx: CallerContext, sessionId: string) {
  const res = await fetch(`/api/v1/biz/sessions/${sessionId}/history`, {
    headers: callerHeaders(ctx),
  })
  if (!res.ok) throw new Error(await readError(res))
  return readData(res)
}

export async function closeBizSession(ctx: CallerContext, sessionId: string) {
  const res = await fetch(`/api/v1/biz/sessions/${sessionId}`, {
    method: 'DELETE',
    headers: callerHeaders(ctx),
  })
  if (!res.ok) throw new Error(await readError(res))
  return readData(res)
}

export async function listBizSessions(ctx: CallerContext, callerUserId: string, params: Record<string, string | number>) {
  const query = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== '') query.set(key, String(value))
  })
  const res = await fetch(`/api/v1/biz/users/${encodeURIComponent(callerUserId)}/sessions?${query.toString()}`, {
    headers: callerHeaders(ctx),
  })
  if (!res.ok) throw new Error(await readError(res))
  return readData(res)
}

export async function streamBizChat(ctx: CallerContext, sessionId: string, message: string, onEvent: EventHandler) {
  const res = await fetch(`/api/v1/biz/sessions/${sessionId}/chat`, {
    method: 'POST',
    headers: callerHeaders(ctx),
    body: JSON.stringify({ message }),
  })
  await readSSE(res, onEvent)
}

export async function streamBizApprove(
  ctx: CallerContext,
  sessionId: string,
  interruptId: string,
  approved: boolean,
  onEvent: EventHandler,
) {
  const res = await fetch(`/api/v1/biz/sessions/${sessionId}/approve`, {
    method: 'POST',
    headers: callerHeaders(ctx),
    body: JSON.stringify({ interrupt_id: interruptId, approved }),
  })
  await readSSE(res, onEvent)
}

async function readError(res: Response) {
  try {
    const body = await res.json()
    return body.message || `HTTP ${res.status}`
  } catch {
    return `HTTP ${res.status}`
  }
}

async function readData(res: Response) {
  const body = await res.json()
  return body.data
}
