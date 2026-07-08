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
