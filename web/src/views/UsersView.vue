<template>
  <div class="page-card">
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">新建用户</el-button>
    </div>
    <el-table :data="items" v-loading="loading">
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="role" label="角色" width="120" />
      <el-table-column prop="created_at" label="创建时间" width="180" />
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editing ? '编辑用户' : '新建用户'" width="420px">
      <el-form label-width="80px">
        <el-form-item v-if="!editing" label="用户名">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role" style="width: 100%">
            <el-option label="admin" value="admin" />
            <el-option label="editor" value="editor" />
            <el-option label="viewer" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item :label="editing ? '新密码' : '密码'">
          <el-input v-model="form.password" type="password" show-password :placeholder="editing ? '留空不修改' : ''" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
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
const form = reactive({ username: '', password: '', role: 'viewer' })

async function load() {
  loading.value = true
  try {
    const { data } = await api.listUsers()
    items.value = data || []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  Object.assign(form, { username: '', password: '', role: 'viewer' })
  dialog.value = true
}
function openEdit(row: any) {
  editing.value = row
  Object.assign(form, { username: row.username, password: '', role: row.role })
  dialog.value = true
}

async function onSave() {
  saving.value = true
  try {
    if (editing.value) {
      const payload: any = { role: form.role }
      if (form.password) payload.password = form.password
      await api.updateUser(editing.value.id, payload)
    } else {
      await api.createUser(form)
    }
    ElMessage.success('已保存')
    dialog.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function onDelete(row: any) {
  await ElMessageBox.confirm(`确认删除用户 ${row.username}？`, '提示', { type: 'warning' })
  await api.deleteUser(row.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>
