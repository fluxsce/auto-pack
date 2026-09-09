export type WorkspaceReleaseStatus = 'running' | 'success' | 'partial' | 'failed'

export function releaseStatusTagType(
  status: string,
): 'success' | 'danger' | 'warning' | 'info' {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'partial':
      return 'warning'
    case 'running':
      return 'info'
    default:
      return 'info'
  }
}
