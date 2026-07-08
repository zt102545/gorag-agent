<template>
  <el-drawer :model-value="modelValue" :title="editing ? '编辑 QA' : '新建 QA'" size="480px" @close="emit('update:modelValue', false)">
    <el-form label-width="80px">
      <el-form-item label="问题"><el-input v-model="form.question" type="textarea" :rows="3" /></el-form-item>
      <el-form-item label="答案"><el-input v-model="form.answer" type="textarea" :rows="5" /></el-form-item>
      <el-form-item label="分类"><el-input v-model="form.category" placeholder="可空，空则异步 AI 补全" /></el-form-item>
      <el-form-item label="标签">
        <el-select v-model="form.tags" multiple filterable allow-create default-first-option style="width: 100%" />
      </el-form-item>
      <el-button type="primary" :loading="saving" @click="emit('save')">保存</el-button>
    </el-form>
  </el-drawer>
</template>

<script setup lang="ts">
import type { QAFormState, QAItem } from '../types'

defineProps<{
  modelValue: boolean
  editing: QAItem | null
  form: QAFormState
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'save'): void
}>()
</script>
