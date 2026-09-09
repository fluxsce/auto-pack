import type { ComposerTranslation } from 'vue-i18n'
import type { PackBuildStatus } from '@/types'

export function isActiveBuildStatus(status: PackBuildStatus | null | undefined): boolean {
  return status === 'pending' || status === 'running'
}

export function buildStatusTagType(
  status: PackBuildStatus | null | undefined,
): 'success' | 'warning' | 'danger' | 'info' {
  if (!status) return 'info'
  if (isActiveBuildStatus(status)) return 'warning'
  switch (status) {
    case 'failed':
      return 'danger'
    case 'success':
      return 'success'
    case 'cancelled':
      return 'info'
    default:
      return 'info'
  }
}

export function buildStatusLabel(
  t: ComposerTranslation,
  status: PackBuildStatus | null | undefined,
): string {
  if (!status) return t('projectBuildStatus_none')
  return t(`projectBuildStatus_${status}`)
}

/** List column: only success / failed; other states return null. */
export function listBuildOutcome(
  status: PackBuildStatus | null | undefined,
): 'success' | 'failed' | null {
  if (status === 'success' || status === 'failed') return status
  return null
}
