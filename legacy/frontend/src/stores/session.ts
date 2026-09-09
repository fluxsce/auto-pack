import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchMe, logout as apiLogout } from '@/request/api/auth'
import { getToken } from '@/request/utils'
import type { UserView } from '@/types'

/** 并发 401 时只触发一次跳转登录 */
let reLoginPromise: Promise<void> | null = null

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

  /** Token 过期/无效时清空会话并跳转登录页，供用户重新登录 */
  function forceReLogin(redirectPath?: string): Promise<void> {
    if (reLoginPromise) return reLoginPromise

    logout()

    reLoginPromise = (async () => {
      try {
        const { router } = await import('@/router')
        const current = router.currentRoute.value
        if (current.name === 'home') return

        const redirect = redirectPath ?? current.fullPath
        await router.replace({
          name: 'home',
          query: redirect && redirect !== '/' ? { redirect } : {},
        })
      } finally {
        reLoginPromise = null
      }
    })()

    return reLoginPromise
  }

  return {
    currentUser,
    loading,
    isAuthenticated,
    loadSession,
    setUser,
    logout,
    forceReLogin,
  }
})
