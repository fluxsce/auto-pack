import type { SyncSourceItem } from '../api/release'

type Translate = (key: string, values?: Record<string, unknown>) => string

export function formatSyncItems(items: SyncSourceItem[] | undefined, t: Translate): string[] {
  if (!items?.length) return []
  const lines: string[] = []
  for (const item of items) {
    lines.push(formatSyncItem(item, t))
    if (item.log?.trim()) {
      for (const row of item.log.split('\n')) {
        const text = row.trimEnd()
        if (text) lines.push(`  ${text}`)
      }
    }
  }
  return lines
}

function formatSyncItem(item: SyncSourceItem, t: Translate): string {
  const name = item.project_name || '—'
  if (item.outcome === 'skipped') {
    return t('workspaces.autoBuildSyncSkip', { name, reason: skipReason(item.message, t) })
  }
  if (item.outcome === 'failed') {
    return t('workspaces.autoBuildSyncFail', { name, msg: item.message || '—' })
  }
  return t('workspaces.autoBuildSyncOk', {
    name,
    action: actionLabel(item.action, t),
    branch: item.branch || 'HEAD',
    tip: item.message || '',
  })
}

function actionLabel(action: string | undefined, t: Translate): string {
  if (action === 'clone') return t('workspaces.autoBuildSyncActionClone')
  return t('workspaces.autoBuildSyncActionFetch')
}

function skipReason(code: string | undefined, t: Translate): string {
  switch (code) {
    case 'local':
      return t('workspaces.autoBuildSyncSkipLocal')
    case 'no_repo':
      return t('workspaces.autoBuildSyncSkipNoRepo')
    case 'disabled':
      return t('workspaces.autoBuildSyncSkipDisabled')
    default:
      return code || '—'
  }
}
