<template>
  <div class="page-card">
    <div class="toolbar">
      <el-input v-model="sessionId" clearable placeholder="会话 ID" style="width: 220px" @keyup.enter="load" />
      <el-input v-model="keyword" clearable placeholder="搜消息/用户/渠道" style="width: 200px" @keyup.enter="load" />
      <el-select v-model="status" clearable placeholder="状态" style="width: 140px" @change="load">
        <el-option label="进行中" value="active" />
        <el-option label="已关闭" value="closed" />
      </el-select>
      <el-input v-model="callerUserId" clearable placeholder="调用方用户 ID" style="width: 180px" @keyup.enter="load" />
      <el-input v-model="channelId" clearable placeholder="渠道 ID" style="width: 160px" @keyup.enter="load" />
      <el-input v-model="tenantId" clearable placeholder="租户 ID" style="width: 160px" @keyup.enter="load" />
      <el-date-picker
        v-model="createdRange"
        type="datetimerange"
        value-format="YYYY-MM-DDTHH:mm:ssZ"
        range-separator="至"
        start-placeholder="创建开始"
        end-placeholder="创建结束"
        style="width: 360px"
      />
      <el-button @click="load">刷新</el-button>
    </div>
    <el-table :data="items" v-loading="loading" @row-click="openDetail">
      <el-table-column prop="id" label="会话 ID" min-width="240" />
      <el-table-column prop="title" label="标题" min-width="180" show-overflow-tooltip />
      <el-table-column prop="external_user_id" label="调用方用户" width="150" />
      <el-table-column prop="channel_id" label="渠道" width="130" />
      <el-table-column prop="tenant_id" label="租户" width="130" />
      <el-table-column prop="message_count" label="消息数" width="90" />
      <el-table-column prop="last_message_role" label="最后角色" width="90" />
      <el-table-column prop="last_message_preview" label="最后一条内容" min-width="260" show-overflow-tooltip />
      <el-table-column prop="last_message_at" label="最后消息时间" width="180" />
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

    <SessionDetailDrawer v-model="drawer" :detail="detail" />
  </div>
</template>

<script setup lang="ts">
import SessionDetailDrawer from '@/features/sessions/components/SessionDetailDrawer.vue'
import { useSessionsViewModel } from '@/features/sessions/composables'

const { callerUserId, channelId, createdRange, detail, drawer, items, keyword, load, loading, openDetail, page, sessionId, size, status, tenantId, total } =
  useSessionsViewModel()
</script>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}
.pager {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>
