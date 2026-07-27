/** Shared helpers for workspace FTP remote push (environment auto-update). */

export function normalizeRemoteDir(p: string): string {
  let s = p.trim().replaceAll('\\', '/').replace(/\/+$/, '')
  if (!s) return '/'
  if (!s.startsWith('/')) s = `/${s}`
  return s
}

export function resolvePushLayout(
  pushPathRaw: string,
  fileName: string,
): { pushDir: string; updatePath: string } {
  const trimmed = pushPathRaw.trim().replaceAll('\\', '/').replace(/\/+$/, '') || '/'
  const normalized = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  const base = normalized.split('/').pop() ?? ''
  if (fileName && base === fileName) {
    const parent = normalized.slice(0, normalized.length - base.length).replace(/\/+$/, '') || '/'
    return { pushDir: normalizeRemoteDir(parent), updatePath: normalized }
  }
  const pushDir = normalizeRemoteDir(pushPathRaw)
  return { pushDir, updatePath: `${pushDir}/${fileName}` }
}

/** BAK/{stamp} directory name: YYYYMMDDHHmmssSSS (local time). */
export function formatBackupDirStamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  const ms = String(d.getMilliseconds()).padStart(3, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}${ms}`
}

/** updateNote.txt line 1 date prefix: YYYYMMDD. */
export function formatManifestDateStamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}`
}

/** Appends a new manifest block after existing updateNote.txt content (blank line before new block). */
export function mergeUpdateNotePreview(existing: string, manifest: string): string {
  const m = manifest.replace(/\r\n/g, '\n').replace(/\n+$/, '')
  const e = existing.replace(/\r\n/g, '\n').replace(/\n+$/, '')
  if (!m) return e ? `${e}\n` : ''
  if (!e) return `${m}\n`
  return `${e}\n\n${m}\n`
}

export interface ManifestPathPair {
  backupPath: string
  updatePath: string
}

export function buildManifestPreview(
  pushTime: Date,
  pusherName: string,
  taskRefText: string,
  backupPathText: string,
  updatePathText: string,
  detailText: string,
): string {
  return buildManifestPreviewPairs(
    pushTime,
    pusherName,
    taskRefText,
    [{ backupPath: backupPathText, updatePath: updatePathText }],
    detailText,
  )
}

/** Indent each detail line with 4 spaces (ops updateNote convention). */
export function formatManifestDetail(raw: string): string {
  const text = raw.replace(/\r\n/g, '\n').replace(/\n+$/, '')
  if (!text.trim()) return ''
  return text
    .split('\n')
    .map((line) => {
      const trimmedRight = line.replace(/[ \t]+$/, '')
      if (!trimmedRight.trim()) return ''
      if (trimmedRight.startsWith('    ') || trimmedRight.startsWith('\t')) return trimmedRight
      return `    ${trimmedRight.replace(/^[ \t]+/, '')}`
    })
    .filter(Boolean)
    .join('\n')
}

/** One updateNote block; multiple packages join paths with ";". */
export function buildManifestPreviewPairs(
  pushTime: Date,
  pusherName: string,
  taskRefText: string,
  pairs: ManifestPathPair[],
  detailText: string,
): string {
  const dateYmd = formatManifestDateStamp(pushTime)
  const backups: string[] = []
  const updates: string[] = []
  for (const p of pairs) {
    const backup = p.backupPath.trim()
    const update = p.updatePath.trim()
    if (!backup && !update) continue
    if (backup) backups.push(backup)
    if (update) updates.push(update)
  }
  const lines = [
    `${dateYmd} ${pusherName.trim()} ${taskRefText.trim()}`,
    `备份包路径:${backups.join(';')}`,
    `更新包路径:${updates.join(';')}`,
  ]
  const detail = formatManifestDetail(detailText)
  if (detail) lines.push(detail)
  return `${lines.join('\n')}\n`
}

export function defaultBackupDir(pushPathRaw: string, fileName: string, at = new Date()): string {
  const layout = resolvePushLayout(pushPathRaw, fileName)
  return `${layout.pushDir}/BAK/${formatBackupDirStamp(at)}`
}

export function backupFilePath(
  pushPathRaw: string,
  fileName: string,
  backupPathRaw: string,
): string {
  const backupDir = backupPathRaw.trim() || defaultBackupDir(pushPathRaw, fileName)
  return `${normalizeRemoteDir(backupDir)}/${fileName}`
}
