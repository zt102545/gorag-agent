<template>
  <div class="page-card" v-loading="loading">
    <el-tabs v-model="active" tab-position="left" style="min-height: 420px">
      <el-tab-pane v-for="p in prompts" :key="p.key" :label="p.key" :name="p.key">
        <el-input v-model="editors[p.key]" type="textarea" :rows="16" />
        <div style="margin-top: 12px">
          <el-button v-if="auth.isAdmin" type="primary" :loading="saving === p.key" @click="save(p.key)">保存</el-button>
          <el-tag v-else type="info">仅管理员可修改</el-tag>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import * as api from '@/api/admin'

const auth = useAuthStore()
const loading = ref(false)
const saving = ref('')
const prompts = ref<any[]>([])
const editors = reactive<Record<string, string>>({})
const active = ref('')

async function load() {
  loading.value = true
  try {
    const { data } = await api.listPrompts()
    prompts.value = data || []
    for (const p of prompts.value) {
      editors[p.key] = p.content
    }
    if (!active.value && prompts.value.length) {
      active.value = prompts.value[0].key
    }
  } finally {
    loading.value = false
  }
}

async function save(key: string) {
  saving.value = key
  try {
    await api.updatePrompt(key, editors[key] || '')
    ElMessage.success('已保存')
  } finally {
    saving.value = ''
  }
}

onMounted(load)
</script>
