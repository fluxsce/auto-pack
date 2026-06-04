import type { App } from 'vue'
import AppPopconfirm from './AppPopconfirm.vue'

export const PopconfirmPlugin = {
  install(app: App) {
    app.component('AppPopconfirm', AppPopconfirm)
  },
}
