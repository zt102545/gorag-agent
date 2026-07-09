<template>
  <el-form v-loading="loading" label-width="160px" style="max-width: 760px">
    <el-form-item label="默认回答">
      <el-input
        v-model="form.default_reply"
        type="textarea"
        :rows="4"
        placeholder="当意图识别不走知识库，或上下文超过限制时，直接返回这段默认回答"
      />
    </el-form-item>
    <el-form-item label="最长上下文长度">
      <el-input-number v-model="form.max_context_chars" :min="1" :step="500" />
      <span class="hint">按累计字符数控制，超过后直接返回默认回答</span>
    </el-form-item>
    <el-form-item label="最长上下文条数">
      <el-input-number v-model="form.max_context_messages" :min="1" :step="1" />
      <span class="hint">按历史消息条数控制，超过后直接返回默认回答</span>
    </el-form-item>
    <el-button type="primary" :loading="saving" @click="save">保存</el-button>
  </el-form>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as api from '@/api/admin'

const loading = ref(false)
const saving = ref(false)
const form = reactive({
  default_reply: '',
  max_context_chars: 12000,
  max_context_messages: 20,
})

async function load() {
  loading.value = true
  try {
    const { data } = await api.getAgentConfig()
    form.default_reply = data.default_reply || ''
    form.max_context_chars = data.max_context_chars || 12000
    form.max_context_messages = data.max_context_messages || 20
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await api.updateAgentConfig({
      default_reply: form.default_reply,
      max_context_chars: form.max_context_chars,
      max_context_messages: form.max_context_messages,
    })
    ElMessage.success('已保存（后续请求使用新配置）')
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.hint {
  margin-left: 12px;
  color: #6b7280;
  font-size: 12px;
}
</style>
