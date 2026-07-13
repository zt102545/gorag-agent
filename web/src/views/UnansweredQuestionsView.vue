<template>
  <div class="page-card">
    <el-alert
      title="知识库无命中或命中置信度过低的问题会自动收集；重复问题累计出现次数。"
      type="info"
      :closable="false"
      class="tip"
    />
    <div class="toolbar">
      <el-input
        v-model="q"
        clearable
        placeholder="搜索问题、检索词或答案"
        style="width: 300px"
        @keyup.enter="load"
      /><el-select
        v-model="status"
        clearable
        placeholder="全部状态"
        style="width: 160px"
        @change="load"
        ><el-option label="待处理" value="pending" /><el-option
          label="已转正式 QA"
          value="converted" /><el-option
          label="已忽略"
          value="ignored" /></el-select
      ><el-button @click="load">刷新</el-button>
    </div>
    <el-table v-loading="loading" :data="items"
      ><el-table-column
        prop="question"
        label="客户问题"
        min-width="280"
        show-overflow-tooltip
      /><el-table-column
        prop="search_query"
        label="实际检索词"
        min-width="220"
        show-overflow-tooltip
      /><el-table-column
        prop="occurrences"
        label="出现次数"
        width="90"
      /><el-table-column label="状态" width="120"
        ><template #default="{ row }"
          ><el-tag
            :type="
              row.status === 'converted'
                ? 'success'
                : row.status === 'ignored'
                  ? 'info'
                  : 'warning'
            "
            >{{ statusLabel(row.status) }}</el-tag
          ></template
        ></el-table-column
      ><el-table-column
        prop="caller_user_id"
        label="最近用户"
        width="140"
      /><el-table-column
        prop="channel_id"
        label="渠道"
        width="120"
      /><el-table-column
        prop="last_seen_at"
        label="最近出现"
        width="180"
      /><el-table-column label="操作" width="190" fixed="right"
        ><template #default="{ row }"
          ><el-button link type="primary" @click="open(row)">处理</el-button
          ><el-button
            v-if="auth.canEdit && row.status === 'pending'"
            link
            type="warning"
            @click="ignore(row)"
            >忽略</el-button
          ><el-button
            v-if="auth.canEdit"
            link
            type="danger"
            @click="remove(row)"
            >删除</el-button
          ></template
        ></el-table-column
      ></el-table
    >
    <div class="pager">
      <el-pagination
        v-model:current-page="page"
        :page-size="size"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>
    <el-drawer v-model="visible" title="处理未找到答案的问题" size="620px"
      ><template v-if="editing"
        ><el-descriptions :column="1" border class="detail"
          ><el-descriptions-item label="客户问题">{{
            editing.question
          }}</el-descriptions-item
          ><el-descriptions-item label="检索词">{{
            editing.search_query || "-"
          }}</el-descriptions-item
          ><el-descriptions-item label="出现次数">{{
            editing.occurrences
          }}</el-descriptions-item
          ><el-descriptions-item label="会话 / Request"
            >{{ editing.session_id || "无状态调用" }} /
            {{ editing.request_id || "-" }}</el-descriptions-item
          ></el-descriptions
        ><el-form label-position="top"
          ><el-form-item label="标准答案" required
            ><el-input
              v-model="form.answer"
              type="textarea"
              :rows="8" /></el-form-item
          ><el-form-item label="分类"
            ><el-input
              v-model="form.category"
              placeholder="可留空，由 QA 自动分类" /></el-form-item
          ><el-form-item label="标签"
            ><el-select
              v-model="form.tags"
              multiple
              filterable
              allow-create
              default-first-option
              style="width: 100%" /></el-form-item
          ><el-form-item label="状态"
            ><el-radio-group v-model="form.status"
              ><el-radio value="pending">待处理</el-radio
              ><el-radio value="ignored">忽略</el-radio></el-radio-group
            ></el-form-item
          ></el-form
        ></template
      ><template #footer
        ><el-button @click="visible = false">关闭</el-button
        ><el-button v-if="auth.canEdit" :loading="saving" @click="save"
          >仅保存</el-button
        ><el-button
          v-if="auth.canEdit"
          type="primary"
          :loading="saving"
          @click="convert"
          >保存并加入正式 QA</el-button
        ></template
      ></el-drawer
    >
  </div>
</template>
<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import * as api from "@/api/admin";
import { useAuthStore } from "@/stores/auth";
const auth = useAuthStore(),
  loading = ref(false),
  saving = ref(false),
  visible = ref(false),
  items = ref<any[]>([]),
  total = ref(0),
  page = ref(1),
  size = ref(20),
  q = ref(""),
  status = ref("pending"),
  editing = ref<any>();
const form = reactive({
  answer: "",
  category: "",
  tags: [] as string[],
  status: "pending",
});
const statusLabel = (s: string) =>
  ({ pending: "待处理", converted: "已转正式 QA", ignored: "已忽略" })[s] || s;
async function load() {
  loading.value = true;
  try {
    const { data } = await api.listUnansweredQuestions({
      q: q.value,
      status: status.value,
      page: page.value,
      size: size.value,
    });
    items.value = data.items || [];
    total.value = data.total || 0;
  } finally {
    loading.value = false;
  }
}
function open(row: any) {
  editing.value = row;
  Object.assign(form, {
    answer: row.answer || "",
    category: row.category || "",
    tags: row.tags || [],
    status: row.status === "converted" ? "pending" : row.status,
  });
  visible.value = true;
}
async function save() {
  if (!editing.value) return;
  saving.value = true;
  try {
    await api.updateUnansweredQuestion(editing.value.id, { ...form });
    ElMessage.success("已保存");
    visible.value = false;
    await load();
  } finally {
    saving.value = false;
  }
}
async function convert() {
  if (!editing.value || !form.answer.trim())
    return ElMessage.warning("请先填写标准答案");
  saving.value = true;
  try {
    await api.updateUnansweredQuestion(editing.value.id, {
      ...form,
      status: "pending",
    });
    await api.convertUnansweredQuestion(editing.value.id);
    ElMessage.success("已加入正式 QA");
    visible.value = false;
    await load();
  } finally {
    saving.value = false;
  }
}
async function ignore(row: any) {
  await api.updateUnansweredQuestion(row.id, {
    answer: row.answer || "",
    category: row.category || "",
    tags: row.tags || [],
    status: "ignored",
  });
  await load();
}
async function remove(row: any) {
  await ElMessageBox.confirm("确认删除该问题？", "提示", { type: "warning" });
  await api.deleteUnansweredQuestion(row.id);
  await load();
}
onMounted(load);
</script>
<style scoped>
.tip,
.detail {
  margin-bottom: 14px;
}
.toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 14px;
}
.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
