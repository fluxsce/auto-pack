import type { RouteLocationNormalized, Router } from 'vue-router'
import { getToken } from '@/request/utils/token'
import { useSessionStore } from '@/stores/session'

function requiresAuth(to: RouteLocationNormalized): boolean {
  return to.matched.some((record) => record.meta.requiresAuth === true)
}

function isGuestRoute(to: RouteLocationNormalized): boolean {
  return to.matched.some((record) => record.meta.guest === true)
}

/** 仅允许站内相对路径，避免开放重定向 */
export function resolveRedirect(target: unknown): string | null {
  if (typeof target !== 'string' || !target.startsWith('/') || target.startsWith('//')) {
    return null
  }
  return target
}

export function setupRouterGuards(router: Router): void {
  router.beforeEach(async (to) => {
    const session = useSessionStore()
    const token = getToken()

    if (requiresAuth(to)) {
      if (!token) {
        return {
          name: 'home',
          query: { redirect: to.fullPath },
        }
      }
      if (!session.currentUser) {
        const user = await session.loadSession()
        if (!user) {
          return {
            name: 'home',
            query: { redirect: to.fullPath },
          }
        }
      }
      return
    }

    if (isGuestRoute(to) && token) {
      if (!session.currentUser) {
        const user = await session.loadSession()
        if (!user) {
          // token 已失效，留在登录页重新登录
          return
        }
      }
      const redirect = resolveRedirect(to.query.redirect)
      return redirect ?? { name: 'dashboard' }
    }
  })
}
