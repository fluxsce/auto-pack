import { inject } from 'vue'
import { message } from './api'
import { messageKey, MessagePlugin } from './plugin'
import type { MessageApi } from './types'

export { message, MessagePlugin, messageKey }
export type { MessageApi }

/** 在 setup 中使用全局 message（需已 app.use(MessagePlugin)） */
export function useMessage(): MessageApi {
  return inject(messageKey, message)
}

export default MessagePlugin
