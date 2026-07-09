<template>
  <div class="page-card" v-loading="loading">
    <el-tabs v-model="active" tab-position="left" class="prompt-tabs">
      <el-tab-pane v-for="p in prompts" :key="p.key" :label="promptMeta(p.key).title" :name="p.key">
        <div class="prompt-header">
          <div>
            <h2>{{ promptMeta(p.key).title }}</h2>
            <div class="prompt-key">{{ p.key }}</div>
          </div>
          <el-tag>{{ promptMeta(p.key).scope }}</el-tag>
        </div>

        <el-descriptions :column="1" border class="prompt-desc">
          <el-descriptions-item label="用途">
            {{ promptMeta(p.key).usage }}
          </el-descriptions-item>
          <el-descriptions-item label="触发场景">
            {{ promptMeta(p.key).trigger }}
          </el-descriptions-item>
          <el-descriptions-item label="编辑建议">
            {{ promptMeta(p.key).tips }}
          </el-descriptions-item>
          <el-descriptions-item v-if="promptMeta(p.key).variables.length" label="可用变量">
            <el-tag v-for="item in promptMeta(p.key).variables" :key="item" class="var-tag" type="info">
              {{ item }}
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <el-input v-model="editors[p.key]" type="textarea" :rows="16" />
        <div class="actions">
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

interface PromptMeta {
  title: string
  scope: string
  usage: string
  trigger: string
  tips: string
  variables: string[]
}

const promptMetas: Record<string, PromptMeta> = {
  agent_instruction: {
    title: '客服 Agent 总指令',
    scope: '对话主流程',
    usage: '定义智能客服的整体角色、边界、工具使用策略和回答原则，是 Agent 每次对话时的基础系统提示词。',
    trigger: '用户进入正式客服对话或 GM 测试客服时加载，用来约束模型是否检索知识库、何时转人工、不能编造等行为。',
    tips: '适合写长期稳定的客服原则。不要放单个活动规则或临时话术；这些更适合写进 QA 或轻量规则。',
    variables: [],
  },
  chat_context_limit: {
    title: '上下文过长回复',
    scope: '固定回复',
    usage: '当当前会话上下文过长，系统不再继续拼接历史消息时，返回给用户的固定提示。',
    trigger: '对话消息数或上下文字符数超过 Agent 配置中的限制时触发。',
    tips: '建议明确告诉用户开启新会话或精简问题，避免承诺系统无法完成的自动摘要或自动迁移。',
    variables: [],
  },
  chat_handoff: {
    title: '转人工回复',
    scope: '固定回复',
    usage: '用户要求人工客服，或系统判断需要转人工时，用于告知用户已经记录转人工诉求。',
    trigger: '用户明确要求人工、投诉升级、问题无法自动处理，或 Agent 调用 transfer_human 工具时触发。',
    tips: '保持简短明确。可以说明等待人工处理，但不要承诺具体响应时间，除非业务确实有 SLA。',
    variables: [],
  },
  chat_no_hit: {
    title: '知识库无命中回复',
    scope: '固定回复',
    usage: '需要检索知识库但没有找到可用 QA，或检索结果不足以支撑答案时的兜底回复。',
    trigger: '意图判断需要 RAG，但知识库检索为空、相关性不足，或回答无法直接由知识库支持时触发。',
    tips: '建议引导用户补充订单号、问题细节或转人工。不要在这里写具体业务答案。',
    variables: [],
  },
  chat_out_of_scope: {
    title: '超出客服范围回复',
    scope: '固定回复',
    usage: '用户问题明显不属于客服业务范围时，用于礼貌拒答并引导回支持范围。',
    trigger: '闲聊之外的开放创作、无关咨询、非订单/退款/配送/售后等客服问题时触发。',
    tips: '说明当前支持范围即可，语气克制。避免过度解释模型能力。',
    variables: [],
  },
  chat_smalltalk: {
    title: '闲聊问候回复',
    scope: '固定回复',
    usage: '处理你好、在吗、谢谢、再见等简单寒暄，不进入完整 RAG 或 Agent 推理。',
    trigger: '轻量规则或意图判断识别为问候、感谢、结束语等低风险闲聊时触发。',
    tips: '可以简短介绍可咨询的问题类型，帮助用户进入有效客服问题。',
    variables: [],
  },
  intent_route: {
    title: '意图路由判断',
    scope: '模型判断',
    usage: '让模型判断用户问题是否需要检索 QA 知识库，并输出结构化 JSON 供后续流程决策。',
    trigger: '每次用户提问进入客服处理前，用于决定 need_search=true/false、意图类别和置信度。',
    tips: '必须保持“只输出 JSON”的约束，并保留 need_search 字段语义，否则后端解析和路由会受影响。',
    variables: ['{query}'],
  },
  qa_classify: {
    title: 'QA 自动分类打标',
    scope: '后台任务',
    usage: '导入或创建 QA 后，当分类/标签为空时，让模型根据问答内容生成 category 和 tags。',
    trigger: 'QA 条目处于待补全分类状态，后台分类 worker 消费任务时触发。',
    tips: '必须要求输出 JSON，字段应包含 category 和 tags。分类应尽量复用历史分类和标签。',
    variables: ['{categories}', '{tags}', '{question}', '{answer}'],
  },
  rag_answer: {
    title: '知识库答案生成',
    scope: 'RAG 回答',
    usage: '把检索到的 QA 知识库内容和用户问题组合成回答提示，要求模型严格基于知识库作答。',
    trigger: '知识库检索命中后，系统将检索上下文注入对话时使用。',
    tips: '应强调不得编造、知识库无依据时要说明无法确认。不要删除 context 和 query 变量。',
    variables: ['{context}', '{query}'],
  },
}

const fallbackMeta: PromptMeta = {
  title: '未登记提示词',
  scope: '其他',
  usage: '当前前端没有该 key 的说明，请结合后端使用位置确认用途。',
  trigger: '未知。',
  tips: '编辑前建议先确认调用链路。',
  variables: [],
}

const auth = useAuthStore()
const loading = ref(false)
const saving = ref('')
const prompts = ref<any[]>([])
const editors = reactive<Record<string, string>>({})
const active = ref('')

function promptMeta(key: string): PromptMeta {
  return promptMetas[key] || fallbackMeta
}

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

<style scoped>
.prompt-tabs {
  min-height: 520px;
}

.prompt-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  margin-bottom: 12px;
}

.prompt-header h2 {
  margin: 0 0 4px;
  font-size: 18px;
}

.prompt-key {
  color: #6b7280;
  font-family: ui-monospace, SFMono-Regular, Consolas, 'Liberation Mono', monospace;
  font-size: 13px;
}

.prompt-desc {
  margin-bottom: 12px;
}

.var-tag {
  margin-right: 8px;
}

.actions {
  margin-top: 12px;
}
</style>
