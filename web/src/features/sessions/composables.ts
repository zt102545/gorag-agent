import { onMounted, ref } from 'vue'
import * as api from './api'
import type { SessionDetail, SessionListItem } from './types'

export function useSessionsViewModel() {
  const loading = ref(false)
  const items = ref<SessionListItem[]>([])
  const total = ref(0)
  const page = ref(1)
  const size = 20
  const sessionId = ref('')
  const keyword = ref('')
  const status = ref('')
  const callerUserId = ref('')
  const channelId = ref('')
  const tenantId = ref('')
  const createdRange = ref<string[]>([])
  const drawer = ref(false)
  const detail = ref<SessionDetail | null>(null)

  async function load() {
    loading.value = true
    try {
      const { data } = await api.listSessions({
        session_id: sessionId.value,
        q: keyword.value,
        page: page.value,
        size,
        status: status.value,
        caller_user_id: callerUserId.value,
        channel_id: channelId.value,
        tenant_id: tenantId.value,
        created_from: createdRange.value?.[0] || '',
        created_to: createdRange.value?.[1] || '',
      })
      items.value = data.items || []
      total.value = data.total || 0
    } finally {
      loading.value = false
    }
  }

  async function openDetail(row: SessionListItem) {
    const { data } = await api.getSession(row.id)
    detail.value = data
    drawer.value = true
  }

  onMounted(load)

  return {
    callerUserId,
    channelId,
    createdRange,
    detail,
    drawer,
    items,
    keyword,
    load,
    loading,
    openDetail,
    page,
    sessionId,
    size,
    status,
    tenantId,
    total,
  }
}
