<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="brand">客服 GM</div>
      <el-menu :default-active="route.path" router>
        <el-menu-item index="/qa">
          <el-icon><Document /></el-icon>
          <span>QA 知识库</span>
        </el-menu-item>
        <el-menu-item index="/chat">
          <el-icon><ChatLineRound /></el-icon>
          <span>测试客服</span>
        </el-menu-item>
        <el-menu-item index="/sessions">
          <el-icon><ChatDotRound /></el-icon>
          <span>会话留存</span>
        </el-menu-item>
        <el-menu-item index="/unanswered-questions">
          <el-icon><QuestionFilled /></el-icon>
          <span>未找到答案问题库</span>
        </el-menu-item>
        <el-menu-item v-if="auth.canEdit" index="/prompts">
          <el-icon><EditPen /></el-icon>
          <span>提示词</span>
        </el-menu-item>
        <el-menu-item v-if="auth.isAdmin" index="/providers">
          <el-icon><Cpu /></el-icon>
          <span>AI 提供商</span>
        </el-menu-item>
        <el-menu-item v-if="auth.isAdmin" index="/ai-errors">
          <el-icon><Warning /></el-icon>
          <span>错误日志</span>
          <el-badge
            v-if="unresolvedCount > 0"
            :value="unresolvedCount"
            class="menu-badge"
          />
        </el-menu-item>
        <el-menu-item v-if="auth.isAdmin" index="/caller-tokens">
          <el-icon><Key /></el-icon>
          <span>调用方 Token</span>
        </el-menu-item>
        <el-menu-item v-if="auth.isAdmin" index="/users">
          <el-icon><User /></el-icon>
          <span>用户管理</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="title">{{ (route.meta.title as string) || "管理台" }}</div>
        <div class="user">
          <el-badge
            v-if="auth.isAdmin && unresolvedCount > 0"
            :value="unresolvedCount"
            class="header-badge"
          >
            <span class="alert-link" @click="router.push('/ai-errors')"
              >错误日志待处理</span
            >
          </el-badge>
          <span>{{ auth.user?.username }} ({{ auth.user?.role }})</span>
          <el-button link type="danger" @click="onLogout">退出</el-button>
        </div>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  ChatDotRound,
  ChatLineRound,
  Cpu,
  Document,
  EditPen,
  Key,
  QuestionFilled,
  User,
  Warning,
} from "@element-plus/icons-vue";
import * as api from "@/api/admin";
import { useAuthStore } from "@/stores/auth";

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const unresolvedCount = ref(0);
let timer: number | undefined;

async function refreshErrors() {
  if (!auth.isAdmin) return;
  try {
    const { data } = await api.getAIErrorUnresolvedCount();
    unresolvedCount.value = data.count || 0;
  } catch {
    unresolvedCount.value = 0;
  }
}

function onLogout() {
  auth.logout();
  router.push("/login");
}

onMounted(() => {
  void refreshErrors();
  timer = window.setInterval(() => {
    void refreshErrors();
  }, 30000);
});

onBeforeUnmount(() => {
  if (timer) {
    window.clearInterval(timer);
  }
});
</script>

<style scoped>
.layout {
  min-height: 100vh;
}
.aside {
  background: #111827;
  color: #fff;
}
.brand {
  height: 56px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  font-weight: 700;
  letter-spacing: 0.5px;
}
.aside :deep(.el-menu) {
  border-right: none;
  background: transparent;
}
.aside :deep(.el-menu-item) {
  color: #d1d5db;
}
.aside :deep(.el-menu-item.is-active) {
  background: #1f2937;
  color: #fff;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}
.title {
  font-size: 18px;
  font-weight: 600;
}
.user {
  display: flex;
  gap: 12px;
  align-items: center;
  color: #4b5563;
}
.alert-link {
  cursor: pointer;
  color: #dc2626;
}
.menu-badge {
  margin-left: auto;
}
.header-badge {
  margin-right: 8px;
}
</style>
