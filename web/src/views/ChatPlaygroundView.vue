<template>
  <div class="page-card chat-page">
    <div class="chat-toolbar">
      <div class="meta">
        <span v-if="sessionId">会话：{{ sessionId }}</span>
        <el-tag v-else type="info" size="small">尚未开始</el-tag>
      </div>
      <div class="actions">
        <el-button @click="startNewChat" :loading="creating">新对话</el-button>
        <el-button v-if="sessionId" @click="endChat" :disabled="sending">结束会话</el-button>
      </div>
    </div>

    <div ref="listRef" class="chat-list">
      <div v-if="messages.length === 0" class="empty">
        点击「新对话」后即可测试客服回复（会走真实 Agent + QA 检索）。
      </div>
      <div v-for="(m, i) in messages" :key="i" class="bubble" :class="m.role">
        <div class="role">{{ roleLabel(m.role) }}</div>
        <div class="content">{{ m.content }}</div>
        <div v-if="m.role === 'tool' && m.qaHits?.length" class="qa-debug">
          <div class="qa-debug-title">命中 QA</div>
          <div v-for="(hit, hitIndex) in m.qaHits" :key="hitIndex" class="qa-hit">
            <div class="qa-hit-meta">
              <el-tag v-if="hit.category" size="small">{{ hit.category }}</el-tag>
              <el-tag v-for="tag in hit.tags" :key="tag" size="small" type="info">{{ tag }}</el-tag>
            </div>
            <div class="qa-hit-question">Q: {{ hit.question }}</div>
            <div class="qa-hit-answer">A: {{ hit.answer }}</div>
          </div>
        </div>
      </div>
      <div v-if="sending" class="bubble assistant typing">
        <div class="role">助手</div>
        <div class="content">{{ streaming || '思考中…' }}</div>
      </div>
    </div>

    <div v-if="pendingInterrupt" class="interrupt-bar">
      <span>需要人工确认（{{ pendingInterrupt.id }}）</span>
      <el-button type="success" size="small" :loading="approving" @click="approve(true)">同意继续</el-button>
      <el-button type="danger" size="small" :loading="approving" @click="approve(false)">拒绝</el-button>
    </div>

    <div class="composer">
      <el-input
        v-model="input"
        type="textarea"
        :rows="3"
        placeholder="输入测试问题，例如：如何申请退款？"
        :disabled="!sessionId || sending"
        @keydown.enter.exact.prevent="send"
      />
      <el-button type="primary" :disabled="!sessionId || !input.trim()" :loading="sending" @click="send">
        发送
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as api from '@/api/admin'
import { streamPlaygroundChat, streamPlaygroundApprove, type SSEEvent } from '@/api/chat'

interface ChatMsg {
  role: string
  content: string
  qaHits?: QADebugHit[]
}

interface QADebugHit {
  category: string
  tags: string[]
  question: string
  answer: string
}

const sessionId = ref('')
const messages = ref<ChatMsg[]>([])
const input = ref('')
const creating = ref(false)
const sending = ref(false)
const approving = ref(false)
const streaming = ref('')
const listRef = ref<HTMLElement | null>(null)
const pendingInterrupt = ref<{ id: string; content: string } | null>(null)

function roleLabel(role: string) {
  if (role === 'user') return '我'
  if (role === 'assistant') return '助手'
  if (role === 'tool') return '工具'
  return role
}

async function scrollBottom() {
  await nextTick()
  if (listRef.value) {
    listRef.value.scrollTop = listRef.value.scrollHeight
  }
}

async function startNewChat() {
  creating.value = true
  try {
    const { data } = await api.createPlaygroundSession()
    sessionId.value = data.id
    messages.value = []
    streaming.value = ''
    pendingInterrupt.value = null
    input.value = ''
    ElMessage.success('已创建测试会话')
  } finally {
    creating.value = false
  }
}

async function endChat() {
  if (!sessionId.value) return
  await api.closePlaygroundSession(sessionId.value)
  ElMessage.success('会话已结束（消息已留存）')
  sessionId.value = ''
  pendingInterrupt.value = null
}

function handleSSE(ev: SSEEvent) {
  if (ev.type === 'error') {
    ElMessage.error(ev.error || '对话出错')
    return
  }
  if (ev.type === 'stream_chunk' || ev.type === 'message') {
    if (ev.content) {
      streaming.value += ev.content
      void scrollBottom()
    }
    return
  }
  if (ev.type === 'tool_result' || ev.type === 'tool_result_chunk') {
    if (ev.content) {
      messages.value.push({ role: 'tool', content: ev.content, qaHits: parseQAHits(ev.content) })
      void scrollBottom()
    }
    return
  }
  if (ev.type === 'action' && ev.action_type === 'interrupted' && ev.interrupt_id) {
    pendingInterrupt.value = { id: ev.interrupt_id, content: ev.content || '' }
    messages.value.push({
      role: 'assistant',
      content: `【转人工中断】${ev.content || ev.interrupt_id}`,
    })
  }
}

async function send() {
  const text = input.value.trim()
  if (!sessionId.value || !text || sending.value) return

  messages.value.push({ role: 'user', content: text })
  input.value = ''
  sending.value = true
  streaming.value = ''
  pendingInterrupt.value = null
  await scrollBottom()

  try {
    await streamPlaygroundChat(sessionId.value, text, handleSSE)
    if (streaming.value.trim()) {
      messages.value.push({ role: 'assistant', content: streaming.value })
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '发送失败')
  } finally {
    streaming.value = ''
    sending.value = false
    await scrollBottom()
  }
}

async function approve(ok: boolean) {
  if (!sessionId.value || !pendingInterrupt.value) return
  approving.value = true
  sending.value = true
  streaming.value = ''
  try {
    const interruptId = pendingInterrupt.value.id
    pendingInterrupt.value = null
    await streamPlaygroundApprove(sessionId.value, interruptId, ok, handleSSE)
    if (streaming.value.trim()) {
      messages.value.push({ role: 'assistant', content: streaming.value })
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '审批失败')
  } finally {
    streaming.value = ''
    sending.value = false
    approving.value = false
    await scrollBottom()
  }
}

function parseQAHits(content: string): QADebugHit[] {
  const blocks = content
    .split(/\n\s*\n/)
    .map((block) => block.trim())
    .filter(Boolean)

  const hits: QADebugHit[] = []
  for (const block of blocks) {
    const lines = block.split('\n').map((line) => line.trim())
    if (lines.length < 3 || !lines[0].startsWith('[')) continue

    const metaLine = lines[0]
    const questionLine = lines.find((line) => line.startsWith('Q:'))
    const answerLine = lines.find((line) => line.startsWith('A:'))
    if (!questionLine || !answerLine) continue

    const categoryMatch = metaLine.match(/分类:\s*(.*?)(?=\s+标签:|$)/)
    const tagsMatch = metaLine.match(/标签:\s*(.+)$/)
    const tags = tagsMatch?.[1]
      ? tagsMatch[1]
          .split(',')
          .map((tag) => tag.trim())
          .filter(Boolean)
      : []

    hits.push({
      category: categoryMatch?.[1]?.trim() || '',
      tags,
      question: questionLine.replace(/^Q:\s*/, ''),
      answer: answerLine.replace(/^A:\s*/, ''),
    })
  }
  return hits
}
</script>

<style scoped>
.chat-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 120px);
  min-height: 520px;
}
.chat-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  gap: 12px;
}
.meta {
  color: #6b7280;
  font-size: 13px;
  word-break: break-all;
}
.actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
.chat-list {
  flex: 1;
  overflow: auto;
  background: #f8fafc;
  border-radius: 10px;
  padding: 16px;
  border: 1px solid #e5e7eb;
}
.empty {
  color: #9ca3af;
  text-align: center;
  margin-top: 80px;
}
.bubble {
  max-width: 82%;
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  background: #fff;
  border: 1px solid #e5e7eb;
}
.bubble.user {
  margin-left: auto;
  background: #dbeafe;
  border-color: #bfdbfe;
}
.bubble.assistant {
  background: #fff;
}
.bubble.tool {
  background: #f3f4f6;
  font-size: 13px;
}
.qa-debug {
  margin-top: 10px;
  border-top: 1px dashed #d1d5db;
  padding-top: 10px;
}
.qa-debug-title {
  font-size: 12px;
  color: #4b5563;
  margin-bottom: 8px;
}
.qa-hit {
  padding: 8px 10px;
  border-radius: 8px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  margin-bottom: 8px;
}
.qa-hit-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 6px;
}
.qa-hit-question,
.qa-hit-answer {
  white-space: pre-wrap;
  line-height: 1.5;
}
.qa-hit-question {
  font-weight: 600;
  margin-bottom: 4px;
}
.bubble.typing {
  opacity: 0.9;
}
.role {
  font-size: 12px;
  color: #6b7280;
  margin-bottom: 4px;
}
.content {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.5;
}
.interrupt-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  padding: 8px 12px;
  background: #fff7ed;
  border: 1px solid #fed7aa;
  border-radius: 8px;
  font-size: 13px;
}
.composer {
  display: flex;
  gap: 12px;
  margin-top: 12px;
  align-items: flex-end;
}
.composer :deep(.el-textarea) {
  flex: 1;
}
</style>
