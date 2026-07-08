import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { UserInfo } from '@/api/admin'
import { login as loginApi, fetchMe } from '@/api/admin'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('gm_token') || '')
  const user = ref<UserInfo | null>(
    localStorage.getItem('gm_user') ? JSON.parse(localStorage.getItem('gm_user')!) : null,
  )

  const isLoggedIn = computed(() => !!token.value)
  const role = computed(() => user.value?.role || '')
  const isAdmin = computed(() => role.value === 'admin')
  const canEdit = computed(() => role.value === 'admin' || role.value === 'editor')

  async function login(username: string, password: string) {
    const { data } = await loginApi(username, password)
    token.value = data.token
    user.value = data.user
    localStorage.setItem('gm_token', data.token)
    localStorage.setItem('gm_user', JSON.stringify(data.user))
  }

  async function refreshMe() {
    if (!token.value) return
    const { data } = await fetchMe()
    user.value = data
    localStorage.setItem('gm_user', JSON.stringify(data))
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('gm_token')
    localStorage.removeItem('gm_user')
  }

  return { token, user, isLoggedIn, role, isAdmin, canEdit, login, refreshMe, logout }
})
