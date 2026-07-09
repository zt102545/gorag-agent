<template>
  <div v-loading="loading">
    <el-alert type="info" :closable="false" style="margin-bottom: 12px">
      <template #title>
        轻量规则会在模型检索前优先执行，适合处理寒暄、感谢、明确转人工和固定拒答等高频场景。
      </template>
    </el-alert>

    <div class="toolbar">
      <el-button type="primary" @click="openCreate">新增规则</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-table :data="items" stripe>
      <el-table-column prop="name" label="规则名称" min-width="160" />
      <el-table-column prop="priority" label="优先级" width="90" />
      <el-table-column prop="match_type" label="匹配方式" width="120" />
      <el-table-column prop="pattern" label="匹配内容" min-width="220" show-overflow-tooltip />
      <el-table-column prop="action_type" label="命中动作" width="130" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updated_at" label="更新时间" width="180" />
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link @click="toggleEnabled(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editing?.id ? '编辑轻量规则' : '新增轻量规则'" width="720px">
      <el-form label-width="120px">
        <el-form-item label="规则名称">
          <el-input v-model="form.name" maxlength="128" />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="form.priority" :min="0" :step="10" />
        </el-form-item>
        <el-form-item label="匹配方式">
          <el-select v-model="form.match_type" style="width: 100%">
            <el-option label="精确匹配" value="exact" />
            <el-option label="包含匹配" value="contains" />
            <el-option label="正则匹配" value="regex" />
            <el-option label="关键词组" value="keywords" />
          </el-select>
        </el-form-item>
        <el-form-item label="匹配内容">
          <el-input
            v-model="form.pattern"
            type="textarea"
            :rows="3"
            :placeholder="patternPlaceholder"
          />
        </el-form-item>
        <el-form-item label="命中动作">
          <el-select v-model="form.action_type" style="width: 100%">
            <el-option label="直接回复" value="reply" />
            <el-option label="转人工" value="handoff" />
            <el-option label="拒答" value="reject" />
            <el-option label="继续检索" value="continue_search" />
          </el-select>
        </el-form-item>
        <el-form-item label="回复内容">
          <el-input
            v-model="form.response_text"
            type="textarea"
            :rows="4"
            :disabled="form.action_type === 'continue_search'"
            :placeholder="responsePlaceholder"
          />
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

const loading = ref(false)
const saving = ref(false)
const dialog = ref(false)
const items = ref<api.ChatLightRule[]>([])
const editing = ref<api.ChatLightRule | null>(null)
const form = reactive<Omit<api.ChatLightRule, 'id' | 'created_at' | 'updated_at'>>({
  name: '',
  enabled: true,
  priority: 100,
  match_type: 'keywords',
  pattern: '',
  action_type: 'reply',
  response_text: '',
})

const patternPlaceholder = computed(() => {
  if (form.match_type === 'keywords') return '多个关键词请用英文逗号分隔，例如：你好,您好,在吗'
  if (form.match_type === 'regex') return '请输入正则表达式'
  return '请输入匹配内容'
})

const responsePlaceholder = computed(() => {
  if (form.action_type === 'handoff') return '可选，不填则使用系统默认转人工提示'
  if (form.action_type === 'reject') return '可选，不填则使用系统默认拒答提示'
  if (form.action_type === 'continue_search') return '继续检索无需填写回复内容'
  return '请输入规则命中后的回复内容'
})

async function load() {
  loading.value = true
  try {
    const { data } = await api.listChatLightRules()
    items.value = data || []
  } finally {
    loading.value = false
  }
}

function resetForm() {
  Object.assign(form, {
    name: '',
    enabled: true,
    priority: 100,
    match_type: 'keywords',
    pattern: '',
    action_type: 'reply',
    response_text: '',
  })
}

function openCreate() {
  editing.value = null
  resetForm()
  dialog.value = true
}

function openEdit(row: api.ChatLightRule) {
  editing.value = row
  Object.assign(form, {
    name: row.name,
    enabled: row.enabled,
    priority: row.priority,
    match_type: row.match_type,
    pattern: row.pattern,
    action_type: row.action_type,
    response_text: row.response_text || '',
  })
  dialog.value = true
}

async function save() {
  saving.value = true
  try {
    const payload = { ...form }
    if (payload.action_type === 'continue_search') {
      payload.response_text = ''
    }
    if (editing.value?.id) {
      await api.updateChatLightRule(editing.value.id, payload)
    } else {
      await api.createChatLightRule(payload)
    }
    ElMessage.success('已保存')
    dialog.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(row: api.ChatLightRule) {
  await api.updateChatLightRule(row.id, {
    name: row.name,
    enabled: !row.enabled,
    priority: row.priority,
    match_type: row.match_type,
    pattern: row.pattern,
    action_type: row.action_type,
    response_text: row.response_text || '',
  })
  ElMessage.success(row.enabled ? '已停用' : '已启用')
  await load()
}

async function remove(row: api.ChatLightRule) {
  await ElMessageBox.confirm(`确认删除规则 ${row.name}？`, '提示', { type: 'warning' })
  await api.deleteChatLightRule(row.id)
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
