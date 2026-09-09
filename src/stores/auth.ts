import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

export type Me = { id: string; login_name: string; display_name: string }

export const useAuthStore = defineStore('auth', () => {
  const me = ref<Me | null>(null)
  const ready = ref(false)
  const signedIn = computed(() => me.value != null)

  async function hydrate(): Promise<void> {
    if (!localStorage.getItem('ap_token')) {
      ready.value = true
      return
    }
    try {
      me.value = await request<Me>({ method: 'GET', url: apiPath('auth', 'me') })
    } catch {
      localStorage.removeItem('ap_token')
      me.value = null
    } finally {
      ready.value = true
    }
  }

  async function login(username: string, password: string): Promise<void> {
    const res = await request<{ token: string; user: Me }>({
      method: 'POST',
      url: apiPath('auth', 'login'),
      data: { username, password },
    })
    localStorage.setItem('ap_token', res.token)
    me.value = res.user
    ready.value = true
  }

  function expire(): void {
    localStorage.removeItem('ap_token')
    me.value = null
    ready.value = true
  }

  return { me, ready, signedIn, hydrate, login, expire }
})
