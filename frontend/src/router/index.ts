import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { APP_BASE } from '@/constants/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import EnvSettingsView from '@/views/env/EnvSettingsView.vue'
import ProjectManageView from '@/views/project/ProjectManageView.vue'
import UserManageView from '@/views/user/UserManageView.vue'
import OperationLogView from '@/views/audit/OperationLogView.vue'
import LoginView from '@/views/home/LoginView.vue'
import { setupRouterGuards } from './guards'

const appChildren: RouteRecordRaw[] = [
  {
    path: '',
    redirect: { name: 'env' },
  },
  {
    path: 'env',
    name: 'env',
    component: EnvSettingsView,
  },
  {
    path: 'projects',
    name: 'projects',
    component: ProjectManageView,
  },
  {
    path: 'users',
    name: 'users',
    component: UserManageView,
  },
  {
    path: 'operation-logs',
    name: 'operation-logs',
    component: OperationLogView,
  },
]

if (import.meta.env.DEV) {
  appChildren.push({
    path: 'dev-test',
    name: 'dev-test',
    component: () => import('@/views/dev/DevTestView.vue'),
  })
}

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'home',
        component: LoginView,
        meta: { guest: true },
      },
    ],
  },
  {
    path: '/',
    component: AppLayout,
    meta: { requiresAuth: true },
    children: appChildren,
  },
]

export const router = createRouter({
  history: createWebHistory(APP_BASE),
  routes,
})

setupRouterGuards(router)
