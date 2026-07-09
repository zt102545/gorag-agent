<template>
  <el-drawer :model-value="modelValue" title="会话详情" size="980px" @close="emit('update:modelValue', false)">
    <template v-if="detail">
      <div class="detail-grid">
        <div><b>ID:</b> {{ detail.session.id }}</div>
        <div><b>标题:</b> {{ detail.session.title || '-' }}</div>
        <div><b>调用方用户:</b> {{ detail.session.external_user_id || '-' }}</div>
        <div><b>渠道:</b> {{ detail.session.channel_id || '-' }}</div>
        <div><b>租户:</b> {{ detail.session.tenant_id || '-' }}</div>
        <div><b>状态:</b> {{ detail.session.status }}</div>
        <div><b>调用方 Token:</b> {{ detail.session.caller_token_id || '-' }}</div>
        <div><b>消息数:</b> {{ detail.message_count }}</div>
        <div><b>创建时间:</b> {{ detail.session.created_at }}</div>
        <div><b>更新时间:</b> {{ detail.session.updated_at }}</div>
      </div>

      <div class="meta-block">
        <div class="section-title">会话上下文 Meta</div>
        <pre>{{ formatJson(detail.session.meta || {}) }}</pre>
      </div>

      <el-divider />
      <div class="section-title">聊天上下文与执行流程</div>

      <div v-for="message in detail.messages" :key="message.id" class="msg" :class="message.role">
        <div class="msg-head">
          <span class="role">{{ roleLabel(message.role) }}</span>
          <span class="time">{{ message.created_at }}</span>
        </div>
        <div class="msg-meta">
          <span v-if="message.request_id">RequestID: {{ message.request_id }}</span>
          <span v-if="message.client_message_id">ClientMessageID: {{ message.client_message_id }}</span>
        </div>
        <div class="content">{{ message.content || '-' }}</div>

        <div v-if="traceSteps(message).length" class="trace-block">
          <div class="section-title small">执行流程</div>
          <el-timeline>
            <el-timeline-item
              v-for="(step, index) in traceSteps(message)"
              :key="`${message.id}-${index}`"
              :type="timelineType(step.status)"
              :timestamp="traceTimestamp(step)"
            >
              <div class="trace-title">
                <span>{{ step.label || stepLabel(step.name) }}</span>
                <el-tag size="small" :type="tagType(step.status)">{{ statusLabel(step.status) }}</el-tag>
              </div>
              <div v-if="step.detail" class="trace-detail">{{ step.detail }}</div>
              <div v-if="step.duration_ms !== undefined" class="trace-duration">耗时：{{ step.duration_ms }}ms</div>
              <div v-if="step.error" class="trace-error">错误：{{ step.error }}</div>
              <div v-if="hasObject(step.input) || hasObject(step.output)" class="trace-json">
                <pre v-if="hasObject(step.input)">输入：{{ formatJson(step.input) }}</pre>
                <pre v-if="hasObject(step.output)">输出：{{ formatJson(step.output) }}</pre>
              </div>
            </el-timeline-item>
          </el-timeline>
        </div>

        <div v-if="toolCalls(message).length" class="extra-block">
          <div class="section-title small">工具调用</div>
          <pre>{{ formatJson(toolCalls(message)) }}</pre>
        </div>

        <div v-if="otherExtra(message)" class="extra-block">
          <div class="section-title small">其他附加信息</div>
          <pre>{{ formatJson(otherExtra(message)) }}</pre>
        </div>
      </div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import type { SessionDetail, SessionMessage } from '../types'

interface TraceStep {
  name?: string
  label?: string
  status?: string
  detail?: string
  input?: Record<string, unknown>
  output?: Record<string, unknown>
  error?: string
  duration_ms?: number
  created_at?: string
}

defineProps<{
  modelValue: boolean
  detail: SessionDetail | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function roleLabel(role: string) {
  if (role === 'user') return '用户'
  if (role === 'assistant') return '助手'
  if (role === 'tool') return '工具'
  if (role === 'system') return '系统'
  return role
}

function stepLabel(name?: string) {
  const labels: Record<string, string> = {
    agent_runner: 'Agent 执行',
    context_limit: '上下文长度检查',
    hit_evaluation: '命中质量评估',
    intent_analyzer: '意图分析',
    knowledge_search: '知识库检索',
    light_rules: '轻量规则匹配',
    query_rewrite: '检索 Query 改写',
    route: '路由决策',
    tool_call: '工具调用',
  }
  return name ? labels[name] || name : '流程步骤'
}

function statusLabel(status?: string) {
  const labels: Record<string, string> = {
    blocked: '阻断',
    error: '失败',
    finished: '完成',
    matched: '命中',
  }
  return status ? labels[status] || status : '完成'
}

function tagType(status?: string) {
  if (status === 'error' || status === 'blocked') return 'danger'
  if (status === 'matched') return 'success'
  return 'info'
}

function timelineType(status?: string) {
  if (status === 'error' || status === 'blocked') return 'danger'
  if (status === 'matched') return 'success'
  return 'primary'
}

function traceTimestamp(step: TraceStep) {
  return step.created_at || ''
}

function traceSteps(message: SessionMessage): TraceStep[] {
  const trace = message.extra?.trace
  return Array.isArray(trace) ? (trace as TraceStep[]) : []
}

function toolCalls(message: SessionMessage) {
  const calls = message.extra?.tool_calls
  return Array.isArray(calls) ? calls : []
}

function otherExtra(message: SessionMessage) {
  if (!message.extra) return null
  const rest = Object.fromEntries(Object.entries(message.extra).filter(([key]) => key !== 'trace' && key !== 'tool_calls'))
  return Object.keys(rest).length ? rest : null
}

function hasObject(value: unknown) {
  return !!value && typeof value === 'object' && Object.keys(value as Record<string, unknown>).length > 0
}

function formatJson(value: unknown) {
  return JSON.stringify(value, null, 2)
}
</script>

<style scoped>
.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 16px;
  margin-bottom: 12px;
}
.section-title {
  font-size: 13px;
  font-weight: 600;
  color: #374151;
  margin-bottom: 8px;
}
.section-title.small {
  font-size: 12px;
  margin-top: 10px;
}
.meta-block pre,
.extra-block pre,
.trace-json pre {
  margin: 0;
  padding: 12px;
  border-radius: 8px;
  background: #111827;
  color: #e5e7eb;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
}
.msg {
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: 8px;
  background: #f3f4f6;
}
.msg.assistant {
  background: #eff6ff;
}
.msg.user {
  background: #eefbf3;
}
.msg.system {
  background: #fef3c7;
}
.msg-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 4px;
}
.role {
  font-size: 12px;
  color: #6b7280;
}
.time,
.msg-meta,
.trace-duration {
  font-size: 12px;
  color: #6b7280;
}
.msg-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 6px;
}
.content {
  white-space: pre-wrap;
  line-height: 1.6;
}
.trace-block,
.extra-block {
  margin-top: 8px;
}
.trace-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}
.trace-detail {
  margin-top: 4px;
  color: #4b5563;
}
.trace-error {
  margin-top: 4px;
  color: #dc2626;
}
.trace-json {
  display: grid;
  gap: 6px;
  margin-top: 8px;
}
</style>
