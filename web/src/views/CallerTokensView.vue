<template>
  <div class="page-card">
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">创建调用方 Token</el-button>
    </div>
    <el-table :data="items" v-loading="loading">
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column prop="prefix" label="前缀" width="120" />
      <el-table-column prop="channel_id" label="渠道" width="140" />
      <el-table-column prop="tenant_id" label="租户" width="140" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="last_used_at" label="最近使用" width="180" />
      <el-table-column prop="expires_at" label="过期时间" width="180" />
      <el-table-column prop="remark" label="备注" min-width="160" />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="warning" @click="toggle(row)">{{ row.enabled ? '禁用' : '启用' }}</el-button>
          <el-button link type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editing ? '编辑调用方 Token' : '创建调用方 Token'" width="520px">
      <el-form label-width="96px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="渠道"><el-input v-model="form.channel_id" /></el-form-item>
        <el-form-item label="租户"><el-input v-model="form.tenant_id" /></el-form-item>
        <el-form-item label="过期时间">
          <el-date-picker
            v-model="form.expires_at"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            placeholder="可空"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <el-alert
        v-if="createdToken"
        type="success"
        :closable="false"
        title="请立即复制保存，明文仅展示一次"
        :description="createdToken"
        style="margin-top: 8px"
      />
      <template #footer>
        <el-button @click="dialog = false">关闭</el-button>
        <el-button type="primary" :loading="saving" @click="onSave">{{ editing ? '保存' : '创建' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as api from '@/api/admin'

const loading = ref(false)
const saving = ref(false)
const items = ref<any[]>([])
const dialog = ref(false)
const editing = ref<any>(null)
const createdToken = ref('')
const form = reactive({
  name: '',
  channel_id: '',
  tenant_id: '',
  expires_at: '',
  remark: '',
})

async function load() {
  loading.value = true
  try {
    const { data } = await api.listCallerTokens()
    items.value = data || []
  } finally {
    loading.value = false
  }
}

function resetForm() {
  Object.assign(form, { name: '', channel_id: '', tenant_id: '', expires_at: '', remark: '' })
}

function openCreate() {
  editing.value = null
  createdToken.value = ''
  resetForm()
  dialog.value = true
}

function openEdit(row: any) {
  editing.value = row
  createdToken.value = ''
  Object.assign(form, {
    name: row.name || '',
    channel_id: row.channel_id || '',
    tenant_id: row.tenant_id || '',
    expires_at: row.expires_at || '',
    remark: row.remark || '',
  })
  dialog.value = true
}

async function onSave() {
  saving.value = true
  try {
    const payload = {
      name: form.name,
      channel_id: form.channel_id,
      tenant_id: form.tenant_id,
      expires_at: form.expires_at || null,
      remark: form.remark,
    }
    if (editing.value) {
      await api.updateCallerToken(editing.value.id, payload)
      ElMessage.success('已保存')
      dialog.value = false
    } else {
      const { data } = await api.createCallerToken(payload)
      createdToken.value = data.caller_token
      ElMessage.success('已创建')
    }
    await load()
  } finally {
    saving.value = false
  }
}

async function toggle(row: any) {
  await api.updateCallerToken(row.id, { enabled: !row.enabled })
  ElMessage.success('已更新')
  await load()
}

async function onDelete(row: any) {
  await ElMessageBox.confirm('确认删除该调用方 Token？', '提示', { type: 'warning' })
  await api.deleteCallerToken(row.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>
