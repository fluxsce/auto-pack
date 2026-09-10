import { syncWorkspaceSources, type SyncEvent, type SyncSourceItem, type SyncSourcesResult } from '../api/release'

type Translate = (key: string, values?: Record<string, unknown>) => string

export function formatSyncItemLines(item: SyncSourceItem, t: Translate): string[] {
  const lines = [formatSyncItem(item, t)]
  if (!item.log?.trim()) return lines
  for (const row of item.log.split('\n')) {
    const text = row.trimEnd()
    if (text) lines.push(`  ${text}`)
  }
  return lines
}

export function formatSyncItems(items: SyncSourceItem[] | undefined, t: Translate): string[] {
  if (!items?.length) return []
  const lines: string[] = []
  for (const item of items) {
    lines.push(...formatSyncItemLines(item, t))
  }
  return lines
}

export async function syncSourcesWithLog(
  workspaceId: string,
  t: Translate,
  append: (line: string) => void,
  onHint?: (hint: string) => void,
): Promise<SyncSourcesResult> {
  const stamp = (): string => new Date().toLocaleString()
  return syncWorkspaceSources(workspaceId, (ev: SyncEvent) => {
    if (ev.event === 'start') {
      append(`[${stamp()}] ${t('workspaces.autoBuildSyncStartCount', { total: ev.total ?? 0 })}`)
      onHint?.(t('workspaces.autoBuildSyncProgress', { done: 0, total: ev.total ?? 0, name: '—' }))
      return
    }
    if (ev.event === 'begin' && ev.item) {
      const hint = t('workspaces.autoBuildSyncProgress', {
        done: ev.index ?? 0,
        total: ev.total ?? 0,
        name: ev.item.project_name,
      })
      onHint?.(hint)
      append(`[${stamp()}] ${hint}`)
      return
    }
    if (ev.event === 'item' && ev.item) {
      for (const line of formatSyncItemLines(ev.item, t)) {
        append(line.startsWith('  ') ? line : `[${stamp()}] ${line}`)
      }
    }
  })
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
