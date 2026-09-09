/// <reference types="vite/client" />

import type { MessageApi } from '@/components/feedback/message/types'

declare module 'vue' {
  interface ComponentCustomProperties {
    $message: MessageApi
  }

  export interface GlobalComponents {
    AppDialog: (typeof import('@/components/feedback/dialog/AppDialog.vue'))['default']
    AppPopconfirm: (typeof import('@/components/feedback/popconfirm/AppPopconfirm.vue'))['default']
    AppPopover: (typeof import('@/components/feedback/popover/AppPopover.vue'))['default']
  }
}

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    guest?: boolean
  }
}
