import { ElMessage } from 'element-plus'
import type { MessageApi } from './types'

export const message: MessageApi = {
  success(text: string) {
    ElMessage.success(text)
  },
  error(text: string) {
    ElMessage.error(text)
  },
  warning(text: string) {
    ElMessage.warning(text)
  },
  info(text: string) {
    ElMessage.info(text)
  },
}
