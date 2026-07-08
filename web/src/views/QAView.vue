<template>
  <div class="page-card">
    <div class="toolbar">
      <el-input v-model="query.q" placeholder="搜索问题/答案" clearable style="width: 220px" @keyup.enter="load" />
      <el-select v-model="query.category" clearable placeholder="分类" style="width: 160px" @change="load">
        <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
      </el-select>
      <el-select v-model="query.tag" clearable placeholder="标签" style="width: 160px" @change="load">
        <el-option v-for="t in tags" :key="t" :label="t" :value="t" />
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
      <el-button v-if="auth.canEdit" :disabled="selectedIds.length === 0" @click="onBatchReclassify">批量重分类</el-button>
      <el-button v-if="auth.canEdit" type="danger" plain :disabled="selectedIds.length === 0" @click="onBatchDelete">批量删除</el-button>
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

    <el-table :data="items" v-loading="loading" stripe @selection-change="onSelectionChange">
      <el-table-column v-if="auth.canEdit" type="selection" width="48" />
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
          <div class="status-cell">
            <el-tag :type="statusType(row.classify_status)" size="small">{{ statusLabel(row.classify_status) }}</el-tag>
            <div v-if="row.classify_error" class="status-error">{{ row.classify_error }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="classified_at" label="分类完成时间" width="180" />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link @click="openDetail(row)">详情</el-button>
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

    <QAFormDrawer v-model="drawer" :editing="editing" :form="form" :saving="saving" @save="onSave" />
    <QADetailDrawer v-model="detailDrawer" :detail="detail" :status-label="statusLabel" />
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import QADetailDrawer from '@/features/qa/components/QADetailDrawer.vue'
import QAFormDrawer from '@/features/qa/components/QAFormDrawer.vue'
import { useQAManagement } from '@/features/qa/composables'

const auth = useAuthStore()
const {
  categories,
  detail,
  detailDrawer,
  drawer,
  editing,
  form,
  importing,
  items,
  loading,
  onBatchDelete,
  onBatchReclassify,
  onDelete,
  onImport,
  onReclassify,
  onSave,
  onSelectionChange,
  openCreate,
  openDetail,
  openEdit,
  query,
  saving,
  selectedIds,
  statusLabel,
  statusType,
  tags,
  total,
  load,
} = useQAManagement()
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
.status-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.status-error {
  font-size: 12px;
  color: #dc2626;
  white-space: normal;
  line-height: 1.4;
}
</style>
