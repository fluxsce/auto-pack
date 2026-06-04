import type { Component } from 'vue'
import { Document, FolderOpened, Monitor, Setting } from '@element-plus/icons-vue'
import type { ProjectDetailSection } from '../useProjectDetail'

export interface ProjectDetailNavItem {
  id: ProjectDetailSection | 'configure'
  labelKey: string
  icon: Component
}

export const PROJECT_DETAIL_NAV: ProjectDetailNavItem[] = [
  { id: 'status', labelKey: 'projectDetailStatus', icon: Monitor },
  { id: 'log', labelKey: 'projectDetailLog', icon: Document },
  { id: 'workspace', labelKey: 'projectDetailProjectDir', icon: FolderOpened },
  { id: 'configure', labelKey: 'projectDetailConfigure', icon: Setting },
]

export function buildStatusTagType(status: string): 'success' | 'danger' | 'warning' | 'info' {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
    case 'pending':
      return 'warning'
    default:
      return 'info'
  }
}
