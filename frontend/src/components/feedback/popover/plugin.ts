import type { App } from 'vue'
import AppPopover from './AppPopover.vue'

export const PopoverPlugin = {
  install(app: App) {
    app.component('AppPopover', AppPopover)
  },
}
