/** 工作空间 FTP 远程推送：路径布局与 updateNote 预览（对齐 1.0）。 */

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

export function formatBackupDirStamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  const ms = String(d.getMilliseconds()).padStart(3, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}${ms}`
}

export function formatManifestDateStamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}`
}

export function mergeUpdateNotePreview(existing: string, manifest: string): string {
  const m = manifest.replace(/\r\n/g, '\n').replace(/\n+$/, '')
  const e = existing.replace(/\r\n/g, '\n').replace(/\n+$/, '')
  if (!m) return e ? `${e}\n` : ''
  if (!e) return `${m}\n`
  return `${e}\n\n${m}\n`
}

export type ManifestPathPair = {
  backupPath: string
  updatePath: string
}

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
  const lines = [`${dateYmd} ${pusherName.trim()} ${taskRefText.trim()}`]
  if (backups.length) lines.push(`备份包路径:${backups.join(';')}`)
  if (updates.length) lines.push(`更新包路径:${updates.join(';')}`)
  const detail = formatManifestDetail(detailText)
  if (detail) lines.push(detail)
  return `${lines.join('\n')}\n`
}

function joinRemote(dir: string, name: string): string {
  const base = normalizeRemoteDir(dir)
  const leaf = name.replace(/^\/+/, '')
  return base === '/' ? `/${leaf}` : `${base}/${leaf}`
}

export function defaultBackupDir(pushPathRaw: string, fileName: string, at = new Date()): string {
  const layout = resolvePushLayout(pushPathRaw, fileName)
  return joinRemote(layout.pushDir, `BAK/${formatBackupDirStamp(at)}`)
}

export function backupFilePath(pushPathRaw: string, fileName: string, backupPathRaw: string): string {
  const backupDir = backupPathRaw.trim() || defaultBackupDir(pushPathRaw, fileName)
  return joinRemote(backupDir, fileName)
}

export function uploadFileName(name: string, isDir: boolean): string {
  return isDir ? `${name}.zip` : name
}

const SCE_PREFIX = 'sce'
const SCE_VDATAHUB = 'sce-vdatahub'

function sceDirStem(name: string): string {
  const n = name.trim().toLowerCase()
  return n.endsWith('.zip') ? n.slice(0, -4) : n
}

/** 目录名是否为前端静态资源树（sce 开头）。 */
export function isSceStaticDirName(name: string): boolean {
  return sceDirStem(name).startsWith(SCE_PREFIX)
}

function isVdatahubStaticDirName(name: string): boolean {
  const n = sceDirStem(name)
  return n === SCE_VDATAHUB || n.startsWith(`${SCE_VDATAHUB}-`) || n.startsWith(`${SCE_VDATAHUB}_`)
}

/** 两个及以上 sce 目录合并后的 zip 文件名（不含路径）。 */
export function sceMergeZipName(names: string[]): string {
  if (names.length && names.every(isVdatahubStaticDirName)) return `${SCE_VDATAHUB}.zip`
  return `${SCE_PREFIX}.zip`
}

export function isSceStaticDirEntry(entry: { name: string; is_dir: boolean }): boolean {
  return entry.is_dir && isSceStaticDirName(entry.name)
}

/** 把 2 个及以上 sce 目录收成一项，供预览 / 备份 / 成功列表使用。 */
export function mergeScePushEntries<T extends { name: string; path: string; is_dir: boolean }>(entries: T[]): T[] {
  const sce = entries.filter(isSceStaticDirEntry)
  if (sce.length < 2) return entries
  const stem = sceDirStem(sceMergeZipName(sce.map((item) => item.name)))
  const merged = { ...sce[0], name: stem, path: `${stem}.zip`, is_dir: true }
  const scePaths = new Set(sce.map((item) => item.path))
  const out: T[] = []
  let placed = false
  for (const item of entries) {
    if (!scePaths.has(item.path)) {
      out.push(item)
      continue
    }
    if (!placed) {
      out.push(merged)
      placed = true
    }
  }
  return out
}
