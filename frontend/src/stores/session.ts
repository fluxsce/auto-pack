import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchMe, logout as apiLogout } from '@/request/api/auth'
import { getToken } from '@/request/utils'
import type { UserView } from '@/types'

export const useSessionStore = defineStore('session', () => {
  const currentUser = ref<UserView | null>(null)
  const loading = ref(false)

  const isAuthenticated = computed(() => Boolean(getToken() && currentUser.value))

  async function loadSession(): Promise<UserView | null> {
    if (!getToken()) {
      currentUser.value = null
      return null
    }
    loading.value = true
    try {
      currentUser.value = await fetchMe()
      return currentUser.value
    } catch {
      logout()
      return null
    } finally {
      loading.value = false
    }
  }

  function setUser(user: UserView | null): void {
    currentUser.value = user
  }

  function logout(): void {
    apiLogout()
    currentUser.value = null
  }

  return {
    currentUser,
    loading,
    isAuthenticated,
    loadSession,
    setUser,
    logout,
  }
})
