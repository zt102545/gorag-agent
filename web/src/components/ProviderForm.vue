<template>
  <el-form v-loading="loading" label-width="120px" style="max-width: 640px">
    <el-form-item label="Provider">
      <el-input v-model="form.provider" placeholder="openai" />
    </el-form-item>
    <el-form-item label="Base URL">
      <el-input v-model="form.base_url" placeholder="可选，兼容代理地址" />
    </el-form-item>
    <el-form-item label="Model">
      <el-input v-model="form.model_name" />
    </el-form-item>
    <el-form-item label="API Key">
      <el-input v-model="form.api_key" type="password" show-password :placeholder="maskHint" />
    </el-form-item>
    <el-form-item v-if="kind === 'chat'" label="Temperature">
      <el-input-number v-model="form.temperature" :min="0" :max="2" :step="0.1" />
    </el-form-item>
    <el-form-item v-if="kind === 'chat'" label="Max Tokens">
      <el-input-number v-model="form.max_tokens" :min="1" :step="128" />
    </el-form-item>
    <el-form-item v-if="kind === 'embedding'" label="Dimensions">
      <el-input-number v-model="form.dimensions" :min="1" :step="64" />
    </el-form-item>
    <el-button type="primary" :loading="saving" @click="save">保存</el-button>
  </el-form>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as api from '@/api/admin'

const props = defineProps<{ kind: 'chat' | 'embedding' }>()
const loading = ref(false)
const saving = ref(false)
const masked = ref('')
const form = reactive({
  provider: 'openai',
  base_url: '',
  model_name: '',
  api_key: '',
  temperature: 0.7,
  max_tokens: 2048,
  dimensions: 1536,
})

const maskHint = computed(() => (masked.value ? `当前: ${masked.value}（留空不修改）` : '未配置'))

async function load() {
  loading.value = true
  try {
    const { data } = await api.getProvider(props.kind)
    form.provider = data.provider
    form.base_url = data.base_url || ''
    form.model_name = data.model_name
    form.api_key = ''
    form.temperature = data.temperature ?? 0.7
    form.max_tokens = data.max_tokens ?? 2048
    form.dimensions = data.dimensions ?? 1536
    masked.value = data.api_key_masked || ''
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const payload: any = { ...form }
    if (!payload.api_key) delete payload.api_key
    await api.updateProvider(props.kind, payload)
    ElMessage.success('已保存（后续请求使用新配置）')
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
