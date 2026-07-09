<template>
  <div class="page-card">
    <div class="toolbar">
      <el-button @click="load">刷新</el-button>
    </div>
    <el-table :data="items" v-loading="loading">
      <el-table-column prop="created_at" label="时间" width="180" />
      <el-table-column prop="source" label="来源" width="140" />
      <el-table-column prop="stage" label="阶段" width="180" />
      <el-table-column prop="message" label="错误信息" min-width="280" show-overflow-tooltip />
      <el-table-column prop="request_id" label="Request ID" width="200" show-overflow-tooltip />
      <el-table-column prop="session_id" label="Session ID" width="200" show-overflow-tooltip />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.resolved ? 'success' : 'danger'">{{ row.resolved ? '已处理' : '未处理' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="详情" min-width="260" show-overflow-tooltip>
        <template #default="{ row }">
          <div class="details">{{ row.details || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" :disabled="row.resolved" @click="onResolve(row)">标记已处理</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as api from '@/api/admin'

const loading = ref(false)
const items = ref<api.AIErrorLog[]>([])

async function load() {
  loading.value = true
  try {
    const { data } = await api.listAIErrors()
    items.value = data || []
  } finally {
    loading.value = false
  }
}

async function onResolve(row: api.AIErrorLog) {
  await api.resolveAIError(row.id)
  ElMessage.success('已标记处理')
  await load()
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}
.details {
  white-space: pre-wrap;
  line-height: 1.5;
}
</style>
