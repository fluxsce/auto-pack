import type { App, InjectionKey } from 'vue'
import { message } from './api'
import type { MessageApi } from './types'

export const messageKey: InjectionKey<MessageApi> = Symbol('message')

export const MessagePlugin = {
  install(app: App) {
    app.provide(messageKey, message)
    app.config.globalProperties.$message = message
  },
}
