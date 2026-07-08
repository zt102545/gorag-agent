<template>
  <el-drawer :model-value="modelValue" title="会话详情" size="900px" @close="emit('update:modelValue', false)">
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
        <div class="meta-title">会话上下文 Meta</div>
        <pre>{{ formatJson(detail.session.meta || {}) }}</pre>
      </div>
      <el-divider />
      <div class="meta-title">聊天上下文</div>
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
        <div v-if="message.extra && Object.keys(message.extra).length" class="extra-block">
          <div class="extra-title">工具/附加信息</div>
          <pre>{{ formatJson(message.extra) }}</pre>
        </div>
      </div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import type { SessionDetail } from '../types'

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
.meta-title {
  font-size: 13px;
  font-weight: 600;
  color: #374151;
  margin-bottom: 8px;
}
.meta-block pre,
.extra-block pre {
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
.msg-meta {
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
.extra-block {
  margin-top: 8px;
}
.extra-title {
  font-size: 12px;
  color: #4b5563;
  margin-bottom: 6px;
}
</style>
