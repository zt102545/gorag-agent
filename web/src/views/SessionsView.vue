<template>
  <div class="page-card">
    <div class="toolbar">
      <el-select v-model="status" clearable placeholder="状态" style="width: 140px" @change="load">
        <el-option label="进行中" value="active" />
        <el-option label="已关闭" value="closed" />
      </el-select>
      <el-button @click="load">刷新</el-button>
    </div>
    <el-table :data="items" v-loading="loading" @row-click="openDetail">
      <el-table-column prop="id" label="会话 ID" min-width="240" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="created_at" label="创建时间" width="180" />
      <el-table-column prop="updated_at" label="更新时间" width="180" />
    </el-table>
    <div class="pager">
      <el-pagination
        background
        layout="total, prev, pager, next"
        :total="total"
        v-model:current-page="page"
        :page-size="size"
        @current-change="load"
      />
    </div>

    <el-drawer v-model="drawer" title="会话详情" size="520px">
      <template v-if="detail">
        <p><b>ID:</b> {{ detail.session.id }}</p>
        <p><b>状态:</b> {{ detail.session.status }}</p>
        <el-divider />
        <div v-for="(m, i) in detail.messages" :key="i" class="msg" :class="m.role">
          <div class="role">{{ m.role }}</div>
          <div class="content">{{ m.content }}</div>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import * as api from '@/api/admin'

const loading = ref(false)
const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const size = 20
const status = ref('')
const drawer = ref(false)
const detail = ref<any>(null)

async function load() {
  loading.value = true
  try {
    const { data } = await api.listSessions({ page: page.value, size, status: status.value })
    items.value = data.items || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function openDetail(row: any) {
  const { data } = await api.getSession(row.id)
  detail.value = data
  drawer.value = true
}

onMounted(load)
</script>

<style scoped>
.pager {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
.msg {
  margin-bottom: 12px;
  padding: 10px 12px;
  border-radius: 8px;
  background: #f3f4f6;
}
.msg.assistant {
  background: #eff6ff;
}
.role {
  font-size: 12px;
  color: #6b7280;
  margin-bottom: 4px;
}
.content {
  white-space: pre-wrap;
}
</style>
