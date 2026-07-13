import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/login",
      name: "login",
      component: () => import("@/views/LoginView.vue"),
      meta: { public: true },
    },
    {
      path: "/",
      component: () => import("@/layouts/AdminLayout.vue"),
      redirect: "/qa",
      children: [
        {
          path: "qa",
          name: "qa",
          component: () => import("@/views/QAView.vue"),
          meta: { title: "QA 知识库" },
        },
        {
          path: "chat",
          name: "chat",
          component: () => import("@/views/ChatPlaygroundView.vue"),
          meta: { title: "测试客服" },
        },
        {
          path: "sessions",
          name: "sessions",
          component: () => import("@/views/SessionsView.vue"),
          meta: { title: "会话留存" },
        },
        {
          path: "unanswered-questions",
          name: "unanswered-questions",
          component: () => import("@/views/UnansweredQuestionsView.vue"),
          meta: { title: "未找到答案问题库" },
        },
        {
          path: "providers",
          name: "providers",
          component: () => import("@/views/ProvidersView.vue"),
          meta: { title: "AI 提供商", roles: ["admin"] },
        },
        {
          path: "ai-errors",
          name: "ai-errors",
          component: () => import("@/views/AIErrorsView.vue"),
          meta: { title: "错误日志", roles: ["admin"] },
        },
        {
          path: "prompts",
          name: "prompts",
          component: () => import("@/views/PromptsView.vue"),
          meta: { title: "提示词", roles: ["admin", "editor"] },
        },
        {
          path: "users",
          name: "users",
          component: () => import("@/views/UsersView.vue"),
          meta: { title: "用户管理", roles: ["admin"] },
        },
        {
          path: "caller-tokens",
          name: "caller-tokens",
          component: () => import("@/views/CallerTokensView.vue"),
          meta: { title: "调用方 Token", roles: ["admin"] },
        },
      ],
    },
  ],
});

router.beforeEach((to) => {
  const auth = useAuthStore();
  if (to.meta.public) {
    if (auth.isLoggedIn && to.path === "/login") return "/qa";
    return true;
  }
  if (!auth.isLoggedIn) {
    return { path: "/login", query: { redirect: to.fullPath } };
  }
  const roles = to.meta.roles as string[] | undefined;
  if (roles && !roles.includes(auth.role)) {
    return "/qa";
  }
  return true;
});

export default router;
