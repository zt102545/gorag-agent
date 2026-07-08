import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadRequestOptions } from 'element-plus'
import * as api from './api'
import type { QAFormState, QAItem, QAListQuery } from './types'

export function statusLabel(status: string) {
  return (
    {
      none: '无需补全',
      pending: '待补全',
      processing: '处理中',
      completed: '已补全',
      failed: '失败',
    } as Record<string, string>
  )[status] || status
}

export function statusType(status: string) {
  return (
    {
      none: 'info',
      pending: 'warning',
      processing: '',
      completed: 'success',
      failed: 'danger',
    } as Record<string, string>
  )[status] || 'info'
}

export function useQAManagement() {
  const loading = ref(false)
  const saving = ref(false)
  const importing = ref(false)
  const items = ref<QAItem[]>([])
  const total = ref(0)
  const categories = ref<string[]>([])
  const tags = ref<string[]>([])
  const drawer = ref(false)
  const detailDrawer = ref(false)
  const editing = ref<QAItem | null>(null)
  const detail = ref<QAItem | null>(null)
  const selectedIds = ref<string[]>([])
  const query = reactive<QAListQuery>({ q: '', category: '', tag: '', classify_status: '', page: 1, size: 20 })
  const form = reactive<QAFormState>({ question: '', answer: '', category: '', tags: [] })

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
    const [categoryResp, tagResp] = await Promise.all([api.listCategories(), api.listTags()])
    categories.value = categoryResp.data || []
    tags.value = tagResp.data || []
  }

  function openCreate() {
    editing.value = null
    Object.assign(form, { question: '', answer: '', category: '', tags: [] })
    drawer.value = true
  }

  function openEdit(row: QAItem) {
    editing.value = row
    Object.assign(form, {
      question: row.question,
      answer: row.answer,
      category: row.category || '',
      tags: [...(row.tags || [])],
    })
    drawer.value = true
  }

  async function openDetail(row: QAItem) {
    const { data } = await api.getQA(row.id)
    detail.value = data
    detailDrawer.value = true
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

  async function onDelete(row: QAItem) {
    await ElMessageBox.confirm('确认删除该 QA？', '提示', { type: 'warning' })
    await api.deleteQA(row.id)
    ElMessage.success('已删除')
    await load()
  }

  function onSelectionChange(rows: QAItem[]) {
    selectedIds.value = rows.map((row) => row.id)
  }

  async function onReclassify(row: QAItem) {
    await api.reclassifyQA(row.id)
    ElMessage.success('已重新入队分类')
    await load()
  }

  async function onBatchDelete() {
    await ElMessageBox.confirm(`确认删除选中的 ${selectedIds.value.length} 条 QA？`, '提示', { type: 'warning' })
    for (const id of selectedIds.value) {
      await api.deleteQA(id)
    }
    ElMessage.success('已批量删除')
    selectedIds.value = []
    await load()
  }

  async function onBatchReclassify() {
    for (const id of selectedIds.value) {
      await api.reclassifyQA(id)
    }
    ElMessage.success('已批量加入重分类队列')
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

  return {
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
  }
}
