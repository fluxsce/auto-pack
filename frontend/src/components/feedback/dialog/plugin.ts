import type { App } from 'vue'
import AppDialog from './AppDialog.vue'

export const DialogPlugin = {
  install(app: App) {
    app.component('AppDialog', AppDialog)
  },
}
