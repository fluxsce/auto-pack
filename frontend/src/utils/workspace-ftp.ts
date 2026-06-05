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

/** Appends a new manifest block after existing updateNote.txt content. */
export function mergeUpdateNotePreview(existing: string, manifest: string): string {
  const m = manifest.replace(/\r\n/g, '\n').replace(/\n+$/, '')
  const e = existing.replace(/\r\n/g, '\n').replace(/\n+$/, '')
  if (!m) return e ? `${e}\n` : ''
  if (!e) return `${m}\n`
  return `${e}\n${m}\n`
}

export function buildManifestPreview(
  pushTime: Date,
  pusherName: string,
  taskRefText: string,
  backupPathText: string,
  updatePathText: string,
  detailText: string,
): string {
  const dateYmd = formatManifestDateStamp(pushTime)
  const lines = [
    `${dateYmd} ${pusherName.trim()} ${taskRefText.trim()}`,
    `备份包路径:${backupPathText.trim()}`,
    `更新包路径:${updatePathText.trim()}`,
  ]
  const detail = detailText.replace(/\r\n/g, '\n').trimEnd()
  if (detail.trim()) {
    lines.push(detail)
  }
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
