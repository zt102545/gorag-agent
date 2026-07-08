<template>
  <div class="page-card">
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">创建 API Key</el-button>
    </div>
    <el-table :data="items" v-loading="loading">
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="prefix" label="前缀" width="120" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" />
      <el-table-column prop="created_at" label="创建时间" width="180" />
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <el-button link type="warning" @click="toggle(row)">{{ row.enabled ? '禁用' : '启用' }}</el-button>
          <el-button link type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" title="创建 Biz API Key" width="480px">
      <el-form label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
      </el-form>
      <el-alert
        v-if="createdKey"
        type="success"
        :closable="false"
        title="请立即复制保存，明文仅展示一次"
        :description="createdKey"
        style="margin-top: 8px"
      />
      <template #footer>
        <el-button @click="dialog = false">关闭</el-button>
        <el-button v-if="!createdKey" type="primary" :loading="saving" @click="onCreate">创建</el-button>
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
const createdKey = ref('')
const form = reactive({ name: '', remark: '' })

async function load() {
  loading.value = true
  try {
    const { data } = await api.listAPIKeys()
    items.value = data || []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  createdKey.value = ''
  Object.assign(form, { name: '', remark: '' })
  dialog.value = true
}

async function onCreate() {
  saving.value = true
  try {
    const { data } = await api.createAPIKey(form)
    createdKey.value = data.api_key
    ElMessage.success('已创建')
    await load()
  } finally {
    saving.value = false
  }
}

async function toggle(row: any) {
  await api.setAPIKeyEnabled(row.id, !row.enabled)
  ElMessage.success('已更新')
  await load()
}

async function onDelete(row: any) {
  await ElMessageBox.confirm('确认删除该 API Key？', '提示', { type: 'warning' })
  await api.deleteAPIKey(row.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>
