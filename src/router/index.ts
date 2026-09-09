import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { setUnauthorizedHandler } from '@/api/http'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/pages/auth/LoginPage.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/pages/workbench/WorkbenchPage.vue'),
    children: [
      { path: '', redirect: { name: 'jobs' } },
      { path: 'dashboard', name: 'dashboard', component: () => import('@/pages/dashboard/Page.vue') },
      { path: 'jobs', name: 'jobs', component: () => import('@/pages/jobs/Page.vue') },
      {
        path: 'workspaces',
        component: () => import('@/pages/workspaces/Page.vue'),
        children: [
          { path: '', name: 'workspaces', component: () => import('@/pages/workspaces/components/ProjectPane.vue') },
          {
            path: 'projects/new',
            name: 'workspace-project-new',
            component: () => import('@/pages/workspaces/components/ProjectEditor.vue'),
          },
          {
            path: 'projects/:id',
            name: 'workspace-project',
            component: () => import('@/pages/workspaces/components/ProjectDetail.vue'),
          },
          {
            path: 'projects/:id/edit',
            name: 'workspace-project-edit',
            component: () => import('@/pages/workspaces/components/ProjectEditor.vue'),
          },
        ],
      },
      { path: 'settings', name: 'settings', component: () => import('@/pages/settings/Page.vue') },
      { path: 'envs', name: 'envs', redirect: { name: 'settings', query: { section: 'sdk' } } },
      { path: 'users', name: 'users', component: () => import('@/pages/users/Page.vue') },
      { path: 'audit', name: 'audit', component: () => import('@/pages/audit/Page.vue') },
    ],
  },
]

export const router = createRouter({
  history: createWebHistory('/autopack/'),
  routes,
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) {
    await auth.hydrate()
  }
  if (to.meta.public) {
    return true
  }
  if (!auth.signedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

export function bindUnauthorizedRedirect(): void {
  setUnauthorizedHandler(() => {
    const auth = useAuthStore()
    auth.expire()
    if (router.currentRoute.value.name !== 'login') {
      void router.replace({ name: 'login', query: { reason: 'expired' } })
    }
  })
}
