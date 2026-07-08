<template>
  <div class="page-card">
    <div class="toolbar">
      <el-input v-model="query.q" placeholder="搜索问题/答案" clearable style="width: 220px" @keyup.enter="load" />
      <el-select v-model="query.category" clearable placeholder="分类" style="width: 160px" @change="load">
        <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
      </el-select>
      <el-select v-model="query.classify_status" clearable placeholder="补全状态" style="width: 160px" @change="load">
        <el-option label="无需补全" value="none" />
        <el-option label="待补全" value="pending" />
        <el-option label="处理中" value="processing" />
        <el-option label="已补全" value="completed" />
        <el-option label="失败" value="failed" />
      </el-select>
      <el-button @click="load">查询</el-button>
      <el-button v-if="auth.canEdit" type="primary" @click="openCreate">新建</el-button>
      <el-upload
        v-if="auth.canEdit"
        :show-file-list="false"
        :http-request="onImport"
        accept=".csv,text/csv"
        :disabled="importing"
      >
        <el-button :loading="importing">导入 CSV</el-button>
      </el-upload>
    </div>

    <el-alert v-if="auth.canEdit" type="info" :closable="false" class="import-tip">
      <template #title>
        支持直接上传 CSV。表头至少包含 <code>question</code>、<code>answer</code>，可选列为
        <code>category</code>、<code>tags</code>。
      </template>
      <template #default>
        <div class="tip-body">
          <div>1. 第一行必须是表头，列名不区分大小写。</div>
          <div>2. <code>tags</code> 多个标签请用英文逗号分隔；若单元格内包含逗号，请用双引号包起来，例如 <code>"退款,售后,时效"</code>。</div>
          <div>3. 建议使用 UTF-8 编码的 CSV 文件。</div>
          <pre class="csv-example">question,answer,category,tags
退款多久到账,退款一般在 1-3 个工作日内原路退回,退款,"退款,时效"
发票怎么开,可在订单完成后前往订单详情页申请电子发票,发票,"发票,电子发票"</pre>
        </div>
      </template>
    </el-alert>

    <el-table :data="items" v-loading="loading" stripe>
      <el-table-column prop="question" label="问题" min-width="180" show-overflow-tooltip />
      <el-table-column prop="answer" label="答案" min-width="200" show-overflow-tooltip />
      <el-table-column prop="category" label="分类" width="120" />
      <el-table-column label="标签" width="160">
        <template #default="{ row }">
          <el-tag v-for="t in row.tags || []" :key="t" size="small" class="tag">{{ t }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="补全状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.classify_status)" size="small">{{ statusLabel(row.classify_status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button v-if="auth.canEdit" link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button v-if="auth.canEdit" link type="warning" @click="onReclassify(row)">重新分类</el-button>
          <el-button v-if="auth.canEdit" link type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        background
        layout="total, prev, pager, next"
        :total="total"
        v-model:current-page="query.page"
        :page-size="query.size"
        @current-change="load"
      />
    </div>

    <el-drawer v-model="drawer" :title="editing ? '编辑 QA' : '新建 QA'" size="480px">
      <el-form label-width="80px">
        <el-form-item label="问题"><el-input v-model="form.question" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="答案"><el-input v-model="form.answer" type="textarea" :rows="5" /></el-form-item>
        <el-form-item label="分类"><el-input v-model="form.category" placeholder="可空，空则异步 AI 补全" /></el-form-item>
        <el-form-item label="标签">
          <el-select v-model="form.tags" multiple filterable allow-create default-first-option style="width: 100%" />
        </el-form-item>
        <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
      </el-form>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadRequestOptions } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import * as api from '@/api/admin'

const auth = useAuthStore()
const loading = ref(false)
const saving = ref(false)
const importing = ref(false)
const items = ref<any[]>([])
const total = ref(0)
const categories = ref<string[]>([])
const drawer = ref(false)
const editing = ref<any>(null)
const query = reactive({ q: '', category: '', classify_status: '', page: 1, size: 20 })
const form = reactive({ question: '', answer: '', category: '', tags: [] as string[] })

function statusLabel(s: string) {
  return ({ none: '无需补全', pending: '待补全', processing: '处理中', completed: '已补全', failed: '失败' } as any)[s] || s
}
function statusType(s: string) {
  return ({ none: 'info', pending: 'warning', processing: '', completed: 'success', failed: 'danger' } as any)[s] || 'info'
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.listQA(query)
    items.value = data.items || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadMeta() {
  const [c] = await Promise.all([api.listCategories()])
  categories.value = c.data || []
}

function openCreate() {
  editing.value = null
  Object.assign(form, { question: '', answer: '', category: '', tags: [] })
  drawer.value = true
}
function openEdit(row: any) {
  editing.value = row
  Object.assign(form, {
    question: row.question,
    answer: row.answer,
    category: row.category || '',
    tags: [...(row.tags || [])],
  })
  drawer.value = true
}

async function onSave() {
  saving.value = true
  try {
    if (editing.value) {
      await api.updateQA(editing.value.id, form)
    } else {
      await api.createQA(form)
    }
    ElMessage.success('已保存')
    drawer.value = false
    await load()
    await loadMeta()
  } finally {
    saving.value = false
  }
}

async function onDelete(row: any) {
  await ElMessageBox.confirm('确认删除该 QA？', '提示', { type: 'warning' })
  await api.deleteQA(row.id)
  ElMessage.success('已删除')
  await load()
}

async function onReclassify(row: any) {
  await api.reclassifyQA(row.id)
  ElMessage.success('已重新入队分类')
  await load()
}

async function onImport(opt: UploadRequestOptions) {
  importing.value = true
  try {
    const res = await api.importQA(opt.file as File)
    const result = res.data
    const successText = `导入完成：成功 ${result.imported || 0} 条，失败 ${result.failed || 0} 条`
    if (result.failed && result.errors?.length) {
      ElMessageBox.alert(result.errors.join('\n'), successText, {
        type: 'warning',
        confirmButtonText: '知道了',
      })
    } else {
      ElMessage.success(successText)
    }
    await load()
    await loadMeta()
  } finally {
    importing.value = false
  }
}

onMounted(async () => {
  await loadMeta()
  await load()
})
</script>

<style scoped>
.import-tip {
  margin-bottom: 16px;
}
.tip-body {
  font-size: 13px;
  line-height: 1.7;
}
.csv-example {
  margin-top: 8px;
  padding: 10px 12px;
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  font-size: 12px;
  overflow-x: auto;
}
.pager {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
.tag {
  margin-right: 4px;
}
</style>
