<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import * as api from "@/api/admin";
import { useAuthStore } from "@/stores/auth";

const auth = useAuthStore();
const loading = ref(false), saving = ref(false), visible = ref(false);
const items = ref<any[]>([]), events = ref<any[]>([]), total = ref(0);
const stats = reactive({ total: 0, pending: 0, converted: 0, ignored: 0, events_7_days: 0 });
const query = reactive({ q: "", status: "pending", reason: "", tenant_id: "", channel_id: "", sort: "occurrences", page: 1, size: 20 });
const editing = ref<any>();
const form = reactive({ answer: "", category: "", tags: [] as string[], status: "pending" });
const reasons: Record<string, string> = { no_hit: "知识库无命中", low_confidence: "命中相关度低", empty_qa_answer: "QA 答案为空", model_empty: "模型未生成答案", model_unanswered: "模型拒答/无法回答" };
const statusLabel = (s: string) => ({ pending: "待处理", converted: "已转正式 QA", ignored: "已忽略" } as Record<string, string>)[s] || s;
const reasonLabel = (s: string) => reasons[s] || s || "未知";

async function load() {
  loading.value = true;
  try {
    const [{ data }, stat] = await Promise.all([api.listUnansweredQuestions(query), api.getUnansweredStats()]);
    items.value = data.items || []; total.value = data.total || 0;
    Object.assign(stats, stat.data || {});
  } finally { loading.value = false; }
}
async function open(row: any) {
  editing.value = row;
  Object.assign(form, { answer: row.answer || "", category: row.category || "", tags: row.tags || [], status: row.status });
  visible.value = true; events.value = [];
  const { data } = await api.listUnansweredEvents(row.id); events.value = data || [];
}
async function save() {
  if (!editing.value || editing.value.status === "converted") return;
  saving.value = true;
  try { await api.updateUnansweredQuestion(editing.value.id, { ...form }); ElMessage.success("已保存"); visible.value = false; await load(); }
  finally { saving.value = false; }
}
async function convert() {
  if (!editing.value || !form.answer.trim()) return ElMessage.warning("请先填写标准答案");
  saving.value = true;
  try { await api.updateUnansweredQuestion(editing.value.id, { ...form, status: "pending" }); await api.convertUnansweredQuestion(editing.value.id); ElMessage.success("已加入正式 QA"); visible.value = false; await load(); }
  finally { saving.value = false; }
}
async function ignore(row: any) { await api.updateUnansweredQuestion(row.id, { answer: row.answer || "", category: row.category || "", tags: row.tags || [], status: "ignored" }); await load(); }
async function remove(row: any) { await ElMessageBox.confirm("确认删除该问题及全部发生明细？", "提示", { type: "warning" }); await api.deleteUnansweredQuestion(row.id); await load(); }
onMounted(load);
</script>

<template>
  <div class="page-card">
    <el-alert title="问题会在知识库无命中、相关度不足，或模型最终无法回答时自动收集；每一次发生都会保留明细。" type="info" :closable="false" class="gap" />
    <el-row :gutter="12" class="gap">
      <el-col v-for="card in [{l:'待处理',v:stats.pending},{l:'7 日发生',v:stats.events_7_days},{l:'已转 QA',v:stats.converted},{l:'已忽略',v:stats.ignored}]" :key="card.l" :span="6"><el-card shadow="never"><el-statistic :title="card.l" :value="card.v" /></el-card></el-col>
    </el-row>
    <div class="toolbar">
      <el-input v-model="query.q" clearable placeholder="搜索问题、检索词或答案" style="width:260px" @keyup.enter="load" />
      <el-select v-model="query.status" clearable placeholder="全部状态" style="width:140px" @change="load"><el-option label="待处理" value="pending"/><el-option label="已转 QA" value="converted"/><el-option label="已忽略" value="ignored"/></el-select>
      <el-select v-model="query.reason" clearable placeholder="全部原因" style="width:170px" @change="load"><el-option v-for="(label,key) in reasons" :key="key" :label="label" :value="key"/></el-select>
      <el-input v-model="query.tenant_id" clearable placeholder="租户" style="width:130px" />
      <el-input v-model="query.channel_id" clearable placeholder="渠道" style="width:130px" />
      <el-button type="primary" @click="load">查询</el-button>
    </div>
    <el-table v-loading="loading" :data="items">
      <el-table-column prop="question" label="客户问题" min-width="260" show-overflow-tooltip />
      <el-table-column label="未回答原因" width="150"><template #default="{row}"><el-tag type="danger" effect="plain">{{ reasonLabel(row.reason) }}</el-tag></template></el-table-column>
      <el-table-column prop="occurrences" label="次数" width="75" sortable />
      <el-table-column label="状态" width="110"><template #default="{row}"><el-tag :type="row.status==='converted'?'success':row.status==='ignored'?'info':'warning'">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
      <el-table-column prop="tenant_id" label="租户" width="120" show-overflow-tooltip />
      <el-table-column prop="channel_id" label="渠道" width="110" />
      <el-table-column prop="last_seen_at" label="最近出现" width="180" />
      <el-table-column label="操作" width="190" fixed="right"><template #default="{row}"><el-button link type="primary" @click="open(row)">详情/处理</el-button><el-button v-if="auth.canEdit&&row.status==='pending'" link type="warning" @click="ignore(row)">忽略</el-button><el-button v-if="auth.canEdit" link type="danger" @click="remove(row)">删除</el-button></template></el-table-column>
    </el-table>
    <div class="pager"><el-pagination v-model:current-page="query.page" :page-size="query.size" :total="total" layout="total, prev, pager, next" @current-change="load" /></div>
    <el-drawer v-model="visible" title="问题详情与处理" size="760px">
      <template v-if="editing">
        <el-descriptions :column="2" border class="gap"><el-descriptions-item label="客户问题" :span="2">{{ editing.question }}</el-descriptions-item><el-descriptions-item label="原因">{{ reasonLabel(editing.reason) }}</el-descriptions-item><el-descriptions-item label="出现次数">{{ editing.occurrences }}</el-descriptions-item><el-descriptions-item label="检索词" :span="2">{{ editing.search_query||'-' }}</el-descriptions-item><el-descriptions-item v-if="editing.converted_qa_id" label="正式 QA ID" :span="2">{{ editing.converted_qa_id }}</el-descriptions-item></el-descriptions>
        <el-form label-position="top" :disabled="editing.status==='converted'"><el-form-item label="标准答案" required><el-input v-model="form.answer" type="textarea" :rows="6" /></el-form-item><el-form-item label="分类"><el-input v-model="form.category" placeholder="可留空，由 QA 自动分类" /></el-form-item><el-form-item label="标签"><el-select v-model="form.tags" multiple filterable allow-create default-first-option style="width:100%" /></el-form-item><el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio value="pending">待处理</el-radio><el-radio value="ignored">忽略</el-radio></el-radio-group></el-form-item></el-form>
        <h3>发生明细</h3><el-table :data="events" size="small" max-height="300"><el-table-column prop="created_at" label="时间" width="170"/><el-table-column prop="caller_user_id" label="用户" width="120"/><el-table-column label="原因" width="140"><template #default="{row}">{{ reasonLabel(row.reason) }}</template></el-table-column><el-table-column prop="final_answer" label="当时 AI 回复" min-width="220" show-overflow-tooltip/></el-table>
      </template>
      <template #footer><el-button @click="visible=false">关闭</el-button><el-button v-if="auth.canEdit&&editing?.status!=='converted'" :loading="saving" @click="save">仅保存</el-button><el-button v-if="auth.canEdit&&editing?.status!=='converted'" type="primary" :loading="saving" @click="convert">保存并加入正式 QA</el-button></template>
    </el-drawer>
  </div>
</template>

<style scoped>.gap{margin-bottom:14px}.toolbar{display:flex;gap:10px;flex-wrap:wrap;margin-bottom:14px}.pager{display:flex;justify-content:flex-end;margin-top:16px}h3{margin:18px 0 10px}</style>
