<template>
  <el-drawer :model-value="modelValue" title="QA 详情" size="520px" @close="emit('update:modelValue', false)">
    <template v-if="detail">
      <p><b>问题:</b> {{ detail.question }}</p>
      <p><b>答案:</b> {{ detail.answer }}</p>
      <p><b>分类:</b> {{ detail.category || '-' }}</p>
      <p><b>标签:</b> {{ (detail.tags || []).join('、') || '-' }}</p>
      <p><b>补全状态:</b> {{ statusLabel(detail.classify_status) }}</p>
      <p><b>补全错误:</b> {{ detail.classify_error || '-' }}</p>
      <p><b>补全完成时间:</b> {{ detail.classified_at || '-' }}</p>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import type { QAItem } from '../types'

defineProps<{
  modelValue: boolean
  detail: QAItem | null
  statusLabel: (status: string) => string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()
</script>
