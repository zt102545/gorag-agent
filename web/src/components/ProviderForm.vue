<template>
  <div v-loading="loading">
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">新增配置</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="profiles">
      <el-table-column prop="name" label="名称" min-width="180" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.active ? 'success' : 'info'">{{ row.active ? '当前启用' : '未启用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="provider" label="Provider" width="120" />
      <el-table-column prop="model_name" label="Model" min-width="180" />
      <el-table-column prop="updated_at" label="更新时间" width="180" />
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="success" :disabled="row.active" @click="activate(row)">切换启用</el-button>
          <el-button link type="danger" :disabled="row.active" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editing?.id ? '编辑模型配置' : '新增模型配置'" width="640px">
      <el-form label-width="120px">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="例如：默认 Chat / 备用 Embedding" />
        </el-form-item>
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
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as api from '@/api/admin'

const props = defineProps<{ kind: 'chat' | 'embedding' }>()
const loading = ref(false)
const saving = ref(false)
const dialog = ref(false)
const profiles = ref<api.ProviderProfile[]>([])
const editing = ref<api.ProviderProfile | null>(null)
const masked = ref('')
const form = reactive({
  name: '',
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
    const { data } = await api.listProviderProfiles(props.kind)
    profiles.value = data || []
  } finally {
    loading.value = false
  }
}

function resetForm() {
  Object.assign(form, {
    name: '',
    provider: 'openai',
    base_url: '',
    model_name: '',
    api_key: '',
    temperature: 0.7,
    max_tokens: 2048,
    dimensions: 1536,
  })
  masked.value = ''
}

function openCreate() {
  editing.value = null
  resetForm()
  dialog.value = true
}

function openEdit(row: api.ProviderProfile) {
  editing.value = row
  Object.assign(form, {
    name: row.name || '',
    provider: row.provider || 'openai',
    base_url: row.base_url || '',
    model_name: row.model_name || '',
    api_key: '',
    temperature: row.temperature ?? 0.7,
    max_tokens: row.max_tokens ?? 2048,
    dimensions: row.dimensions ?? 1536,
  })
  masked.value = row.api_key_masked || ''
  dialog.value = true
}

async function save() {
  saving.value = true
  try {
    const payload: any = { ...form }
    if (!payload.api_key) delete payload.api_key
    if (editing.value?.id) {
      await api.updateProviderProfile(props.kind, editing.value.id, payload)
    } else {
      await api.createProviderProfile(props.kind, payload)
    }
    ElMessage.success('已保存，重启服务后生效')
    dialog.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function activate(row: api.ProviderProfile) {
  if (!row.id) return
  await api.activateProviderProfile(props.kind, row.id)
  ElMessage.success('已切换启用配置')
  await load()
}

async function remove(row: api.ProviderProfile) {
  if (!row.id) return
  await ElMessageBox.confirm(`确认删除配置 ${row.name || row.model_name}？`, '提示', { type: 'warning' })
  await api.deleteProviderProfile(props.kind, row.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 12px;
}
</style>
