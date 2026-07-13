<template>
  <div class="page-card">
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">创建接入应用</el-button>
    </div>
    <el-alert
      type="info"
      :closable="false"
      class="tip"
      title="公开控件只需要渠道 ID 和用户 ID；App Secret 仅供服务端换票模式使用。"
    />

    <el-table :data="items" v-loading="loading">
      <el-table-column prop="name" label="应用名称" min-width="150" />
      <el-table-column prop="channel_id" label="渠道 ID" width="140" />
      <el-table-column label="公开控件" width="110">
        <template #default="{ row }">
          <el-tag :type="row.public_access ? 'success' : 'info'">
            {{ row.public_access ? '已开启' : '未开启' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="IP 限额" width="170">
        <template #default="{ row }">
          {{ row.ip_rate_per_minute || 60 }}/分钟 · {{ row.ip_daily_quota || 2000 }}/天
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">
            {{ row.enabled ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="last_used_at" label="最近使用" width="180" />
      <el-table-column prop="remark" label="备注" min-width="150" />
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="warning" @click="toggle(row)">
            {{ row.enabled ? '停用' : '启用' }}
          </el-button>
          <el-button link type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editing ? '编辑接入应用' : '创建接入应用'" width="620px">
      <el-form label-width="130px">
        <el-form-item label="应用名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="渠道 ID" required>
          <el-input v-model="form.channel_id" placeholder="例如 website-main；公开渠道不可重复" />
        </el-form-item>
        <el-form-item label="限定租户">
          <el-input v-model="form.tenant_id" placeholder="没有多租户时留空" />
        </el-form-item>
        <el-form-item label="公开控件">
          <el-switch v-model="form.public_access" />
          <span class="form-hint">开启后，前端无需换票接口和 App Secret</span>
        </el-form-item>

        <template v-if="form.public_access">
          <el-form-item label="允许来源域名">
            <el-input
              v-model="form.allowed_origins_text"
              type="textarea"
              :rows="3"
              placeholder="每行一个完整 Origin，例如：https://www.example.com；留空表示不限制"
            />
          </el-form-item>
          <el-form-item label="每 IP 每分钟">
            <el-input-number v-model="form.ip_rate_per_minute" :min="1" :max="10000" />
            <span class="form-hint">包含换 Token、会话和聊天请求</span>
          </el-form-item>
          <el-form-item label="每 IP 每日配额">
            <el-input-number v-model="form.ip_daily_quota" :min="1" :max="1000000" />
            <span class="form-hint">按 UTC 自然日统计，计数保存在 Redis</span>
          </el-form-item>
        </template>

        <el-form-item label="过期时间">
          <el-date-picker
            v-model="form.expires_at"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            placeholder="可留空"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>

      <el-alert
        v-if="created"
        type="success"
        :closable="false"
        :title="created.public_access ? '公开渠道已创建，前端只需使用渠道 ID' : '应用已创建，请立即保存服务端凭证'"
      >
        <template #default>
          <div class="credential">
            <div>渠道 ID：<code>{{ created.channel_id }}</code></div>
            <template v-if="!created.public_access">
              <div>App ID：<code>{{ created.app_id }}</code></div>
              <div>App Secret：<code>{{ created.app_secret }}</code></div>
              <el-button size="small" @click="copyCredential">复制凭证</el-button>
            </template>
          </div>
        </template>
      </el-alert>

      <template #footer>
        <el-button @click="dialog = false">关闭</el-button>
        <el-button type="primary" :loading="saving" @click="onSave">
          {{ editing ? '保存' : '创建' }}
        </el-button>
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
const created = ref<any>(null)

const form = reactive({
  name: '',
  channel_id: '',
  tenant_id: '',
  public_access: true,
  allowed_origins_text: '',
  ip_rate_per_minute: 60,
  ip_daily_quota: 2000,
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

function reset() {
  Object.assign(form, {
    name: '',
    channel_id: '',
    tenant_id: '',
    public_access: true,
    allowed_origins_text: '',
    ip_rate_per_minute: 60,
    ip_daily_quota: 2000,
    expires_at: '',
    remark: '',
  })
}

function openCreate() {
  editing.value = null
  created.value = null
  reset()
  dialog.value = true
}

function openEdit(row: any) {
  editing.value = row
  created.value = null
  Object.assign(form, {
    name: row.name || '',
    channel_id: row.channel_id || '',
    tenant_id: row.tenant_id || '',
    public_access: Boolean(row.public_access),
    allowed_origins_text: (row.allowed_origins || []).join('\n'),
    ip_rate_per_minute: row.ip_rate_per_minute || 60,
    ip_daily_quota: row.ip_daily_quota || 2000,
    expires_at: row.expires_at || '',
    remark: row.remark || '',
  })
  dialog.value = true
}

function payload() {
  return {
    name: form.name,
    channel_id: form.channel_id,
    tenant_id: form.tenant_id,
    public_access: form.public_access,
    allowed_origins: form.allowed_origins_text
      .split(/\r?\n/)
      .map((value) => value.trim())
      .filter(Boolean),
    ip_rate_per_minute: form.ip_rate_per_minute,
    ip_daily_quota: form.ip_daily_quota,
    expires_at: form.expires_at || null,
    remark: form.remark,
  }
}

async function onSave() {
  if (!form.name.trim() || !form.channel_id.trim()) {
    ElMessage.warning('请填写应用名称和渠道 ID')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await api.updateCallerToken(editing.value.id, payload())
      ElMessage.success('已保存')
      dialog.value = false
    } else {
      const { data } = await api.createCallerToken(payload())
      created.value = data
      ElMessage.success('应用已创建')
    }
    await load()
  } finally {
    saving.value = false
  }
}

async function toggle(row: any) {
  await api.updateCallerToken(row.id, { enabled: !row.enabled })
  await load()
}

async function onDelete(row: any) {
  await ElMessageBox.confirm('删除后该渠道签发的短期 Token 将立即失效，确认删除？', '提示', { type: 'warning' })
  await api.deleteCallerToken(row.id)
  await load()
}

async function copyCredential() {
  await navigator.clipboard.writeText(`APP_ID=${created.value.app_id}\nAPP_SECRET=${created.value.app_secret}`)
  ElMessage.success('凭证已复制')
}

onMounted(load)
</script>

<style scoped>
.tip,
.toolbar {
  margin-bottom: 12px;
}
.credential {
  line-height: 2;
  word-break: break-all;
}
.credential code {
  user-select: all;
}
.form-hint {
  margin-left: 10px;
  color: #909399;
  font-size: 12px;
}
</style>
