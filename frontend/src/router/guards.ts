import type { RouteLocationNormalized, Router } from 'vue-router'
import { getToken } from '@/request/utils'

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
  router.beforeEach((to) => {
    const token = getToken()
    const authed = Boolean(token)

    if (requiresAuth(to) && !authed) {
      return {
        name: 'home',
        query: { redirect: to.fullPath },
      }
    }

    if (isGuestRoute(to) && authed) {
      const redirect = resolveRedirect(to.query.redirect)
      return redirect ?? { name: 'env' }
    }
  })
}
