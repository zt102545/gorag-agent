<template>
  <div class="playground">
    <section class="config-panel">
      <div class="panel-head">
        <div>
          <h3>测试上下文</h3>
          <p>使用真实调用方 Token 走业务接口，便于复现外部调用场景。</p>
        </div>
        <el-button :loading="tokenLoading" @click="loadTokens">刷新 Token</el-button>
      </div>

      <el-form label-width="96px" class="config-form">
        <el-form-item label="调用 Token" required>
          <el-select v-model="selectedTokenId" filterable clearable placeholder="选择已保存明文的 Token" @change="applySelectedToken">
            <el-option
              v-for="token in tokens"
              :key="token.id"
              :label="`${token.name || token.prefix} (${token.prefix})`"
              :value="token.id"
              :disabled="!token.caller_token || !token.enabled"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Token 明文" required>
          <el-input v-model="callerToken" show-password placeholder="可直接粘贴调用方 Token" />
        </el-form-item>
        <el-form-item label="调用方用户" required>
          <el-input v-model="callerUserId" placeholder="例如 user_001，用于创建和查询历史会话" />
        </el-form-item>
        <el-form-item label="渠道">
          <el-input v-model="channelId" placeholder="为空时使用 Token 默认渠道" />
        </el-form-item>
        <el-form-item label="租户">
          <el-input v-model="tenantId" placeholder="为空时使用 Token 默认租户" />
        </el-form-item>
        <el-form-item label="Meta">
          <el-input v-model="metaText" type="textarea" :rows="4" placeholder='例如 {"source":"gm","order_id":"123"}' />
        </el-form-item>
      </el-form>
    </section>

    <section class="workbench">
      <aside class="session-panel">
        <div class="panel-head compact">
          <h3>历史会话</h3>
          <el-button size="small" :loading="historyLoading" @click="loadSessionList">查询</el-button>
        </div>
        <div class="session-load">
          <el-input v-model="manualSessionId" clearable placeholder="输入会话 ID 继续" @keyup.enter="loadSessionById" />
          <el-button @click="loadSessionById">加载</el-button>
        </div>
        <div v-if="sessionItems.length === 0" class="empty small">暂无历史会话</div>
        <button
          v-for="item in sessionItems"
          :key="item.id"
          class="session-item"
          :class="{ active: item.id === sessionId }"
          @click="selectSession(item.id)"
        >
          <span class="session-title">{{ item.title || item.id }}</span>
          <span class="session-sub">{{ item.last_message_preview || '无消息' }}</span>
        </button>
      </aside>

      <main class="chat-page">
        <div class="chat-toolbar">
          <div class="meta">
            <span v-if="sessionId">会话：{{ sessionId }}</span>
            <el-tag v-else type="info" size="small">尚未开始</el-tag>
          </div>
          <div class="actions">
            <el-button type="primary" @click="startNewChat" :loading="creating">新会话</el-button>
            <el-button v-if="sessionId" @click="reloadCurrentHistory" :loading="historyLoading">刷新历史</el-button>
            <el-button v-if="sessionId" @click="endChat" :disabled="sending">结束会话</el-button>
          </div>
        </div>

        <div ref="listRef" class="chat-list">
          <div v-if="messages.length === 0" class="empty">选择 Token 并创建或加载会话后即可测试客服回复。</div>
          <div v-for="(m, i) in messages" :key="i" class="bubble" :class="m.role">
            <div class="role">{{ roleLabel(m.role) }}</div>
            <div class="content" v-html="sanitizeRichText(m.content)"></div>
          </div>
          <div v-if="sending" class="bubble assistant typing">
            <div class="role">助手</div>
            <div class="content" v-html="sanitizeRichText(streaming || '思考中...')"></div>
          </div>
        </div>

        <div v-if="pendingInterrupt" class="interrupt-bar">
          <span>需要人工确认：{{ pendingInterrupt.content || pendingInterrupt.id }}</span>
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
          <el-button type="primary" :disabled="!sessionId || !input.trim()" :loading="sending" @click="send">发送</el-button>
        </div>
      </main>
    </section>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as adminApi from '@/api/admin'
import { sanitizeRichText } from '@/utils/richText'
import {
  closeBizSession,
  createBizSession,
  getBizHistory,
  getBizSession,
  listBizSessions,
  streamBizApprove,
  streamBizChat,
  type SSEEvent,
} from '@/api/chat'

interface ChatMsg {
  role: string
  content: string
}

interface CallerTokenRow {
  id: string
  name?: string
  caller_token?: string
  prefix?: string
  channel_id?: string
  tenant_id?: string
  enabled?: boolean
}

interface SessionListItem {
  id: string
  title?: string
  last_message_preview?: string
}

const tokens = ref<CallerTokenRow[]>([])
const selectedTokenId = ref('')
const callerToken = ref('')
const callerUserId = ref('test_user')
const channelId = ref('')
const tenantId = ref('')
const metaText = ref('{\n  "source": "gm_playground"\n}')

const sessionId = ref('')
const manualSessionId = ref('')
const sessionItems = ref<SessionListItem[]>([])
const messages = ref<ChatMsg[]>([])
const input = ref('')

const tokenLoading = ref(false)
const historyLoading = ref(false)
const creating = ref(false)
const sending = ref(false)
const approving = ref(false)
const streaming = ref('')
const listRef = ref<HTMLElement | null>(null)
const pendingInterrupt = ref<{ id: string; content: string } | null>(null)

function callerContext() {
  const token = callerToken.value.trim()
  if (!token) throw new Error('请先选择或填写调用方 Token')
  return {
    token,
    callerUserId: callerUserId.value.trim(),
    channelId: channelId.value.trim(),
    tenantId: tenantId.value.trim(),
  }
}

function parseMeta() {
  const raw = metaText.value.trim()
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error('Meta 必须是 JSON 对象')
    }
    return parsed
  } catch (err: any) {
    throw new Error(err?.message || 'Meta JSON 格式不正确')
  }
}

function roleLabel(role: string) {
  if (role === 'user') return '用户'
  if (role === 'assistant') return '助手'
  if (role === 'tool') return '工具'
  if (role === 'system') return '系统'
  return role
}

function normalizeMessage(raw: any): ChatMsg | null {
  const role = String(raw?.role || raw?.Role || '').toLowerCase()
  const content = String(raw?.content || raw?.Content || '').trim()
  if (!content || (role !== 'user' && role !== 'assistant')) return null
  return { role, content }
}

async function scrollBottom() {
  await nextTick()
  if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
}

async function loadTokens() {
  tokenLoading.value = true
  try {
    const { data } = await adminApi.listCallerTokens()
    tokens.value = data || []
    if (!callerToken.value && tokens.value.length) {
      const first = tokens.value.find((token) => token.enabled && token.caller_token)
      if (first) {
        selectedTokenId.value = first.id
        applySelectedToken()
      }
    }
  } catch (err: any) {
    tokens.value = []
    ElMessage.info(err?.message || '无法读取 Token 列表，可手动粘贴调用方 Token')
  } finally {
    tokenLoading.value = false
  }
}

function applySelectedToken() {
  const row = tokens.value.find((token) => token.id === selectedTokenId.value)
  if (!row) return
  callerToken.value = row.caller_token || ''
  if (!channelId.value) channelId.value = row.channel_id || ''
  if (!tenantId.value) tenantId.value = row.tenant_id || ''
}

async function loadSessionList() {
  try {
    const ctx = callerContext()
    if (!ctx.callerUserId) {
      ElMessage.warning('请填写调用方用户后再查询历史会话')
      return
    }
    historyLoading.value = true
    const data = await listBizSessions(ctx, ctx.callerUserId, {
      page: 1,
      size: 20,
      channel_id: ctx.channelId || '',
      tenant_id: ctx.tenantId || '',
    })
    sessionItems.value = data.items || []
  } catch (err: any) {
    ElMessage.error(err?.message || '查询历史会话失败')
  } finally {
    historyLoading.value = false
  }
}

async function startNewChat() {
  creating.value = true
  try {
    const ctx = callerContext()
    const meta = parseMeta()
    const data = await createBizSession(ctx, {
      caller_user_id: ctx.callerUserId,
      channel_id: ctx.channelId,
      tenant_id: ctx.tenantId,
      meta,
    })
    sessionId.value = data.id
    manualSessionId.value = data.id
    messages.value = []
    streaming.value = ''
    pendingInterrupt.value = null
    input.value = ''
    ElMessage.success('已创建测试会话')
    await loadSessionList()
  } catch (err: any) {
    ElMessage.error(err?.message || '创建会话失败')
  } finally {
    creating.value = false
  }
}

async function loadSessionById() {
  const id = manualSessionId.value.trim()
  if (!id) return
  await selectSession(id)
}

async function selectSession(id: string) {
  try {
    const ctx = callerContext()
    historyLoading.value = true
    await getBizSession(ctx, id)
    sessionId.value = id
    manualSessionId.value = id
    await reloadCurrentHistory()
    ElMessage.success('已加载会话历史')
  } catch (err: any) {
    ElMessage.error(err?.message || '加载会话失败')
  } finally {
    historyLoading.value = false
  }
}

async function reloadCurrentHistory() {
  if (!sessionId.value) return
  const ctx = callerContext()
  const data = await getBizHistory(ctx, sessionId.value)
  messages.value = (data.messages || []).map(normalizeMessage).filter(Boolean) as ChatMsg[]
  pendingInterrupt.value = null
  await scrollBottom()
}

async function endChat() {
  if (!sessionId.value) return
  try {
    await closeBizSession(callerContext(), sessionId.value)
    ElMessage.success('会话已结束，消息已留存')
    sessionId.value = ''
    pendingInterrupt.value = null
    await loadSessionList()
  } catch (err: any) {
    ElMessage.error(err?.message || '结束会话失败')
  }
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
  if (ev.type === 'tool_result' || ev.type === 'tool_result_chunk' || ev.type === 'tool_calls') return
  if (ev.type === 'action' && ev.action_type === 'interrupted' && ev.interrupt_id) {
    pendingInterrupt.value = { id: ev.interrupt_id, content: ev.content || '' }
    messages.value.push({
      role: 'assistant',
      content: `【转人工确认】${ev.content || ev.interrupt_id}`,
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
    await streamBizChat(callerContext(), sessionId.value, text, handleSSE)
    if (streaming.value.trim()) messages.value.push({ role: 'assistant', content: streaming.value })
    await loadSessionList()
  } catch (err: any) {
    ElMessage.error(err?.message || '发送失败')
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
    await streamBizApprove(callerContext(), sessionId.value, interruptId, ok, handleSSE)
    if (streaming.value.trim()) messages.value.push({ role: 'assistant', content: streaming.value })
    await loadSessionList()
  } catch (err: any) {
    ElMessage.error(err?.message || '审批失败')
  } finally {
    streaming.value = ''
    sending.value = false
    approving.value = false
    await scrollBottom()
  }
}

onMounted(loadTokens)
</script>

<style scoped>
.playground {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.config-panel,
.session-panel,
.chat-page {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}
.config-panel {
  padding: 14px 16px;
}
.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.panel-head.compact {
  align-items: center;
}
.panel-head h3 {
  margin: 0;
  font-size: 15px;
}
.panel-head p {
  margin: 4px 0 0;
  color: #6b7280;
  font-size: 13px;
}
.config-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 12px;
}
.config-form :deep(.el-form-item:nth-child(2)),
.config-form :deep(.el-form-item:nth-child(6)) {
  grid-column: 1 / -1;
}
.workbench {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 14px;
  min-height: 560px;
}
.session-panel {
  padding: 12px;
  overflow: auto;
}
.session-load {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}
.session-item {
  display: block;
  width: 100%;
  text-align: left;
  border: 1px solid #e5e7eb;
  background: #fff;
  border-radius: 8px;
  padding: 9px 10px;
  margin-bottom: 8px;
  cursor: pointer;
}
.session-item.active {
  border-color: #409eff;
  background: #eff6ff;
}
.session-title,
.session-sub {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.session-title {
  font-size: 13px;
  color: #111827;
}
.session-sub {
  margin-top: 4px;
  font-size: 12px;
  color: #6b7280;
}
.chat-page {
  display: flex;
  flex-direction: column;
  min-height: 560px;
  padding: 12px;
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
  border-radius: 8px;
  padding: 16px;
  border: 1px solid #e5e7eb;
}
.empty {
  color: #9ca3af;
  text-align: center;
  margin-top: 80px;
}
.empty.small {
  margin: 24px 0;
  font-size: 13px;
}
.bubble {
  max-width: 82%;
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: 8px;
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
.content :deep(img) {
  display: block;
  max-width: 100%;
  max-height: 280px;
  margin: 8px 0;
  border-radius: 6px;
}
.content :deep(a) {
  color: #2563eb;
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
@media (max-width: 1100px) {
  .workbench,
  .config-form {
    grid-template-columns: 1fr;
  }
  .config-form :deep(.el-form-item) {
    grid-column: 1 / -1;
  }
}
</style>
