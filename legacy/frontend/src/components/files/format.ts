export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB'] as const
  let value = bytes / 1024
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value >= 100 ? Math.round(value) : value.toFixed(value >= 10 ? 1 : 2)} ${units[unitIndex]}`
}

export function basenameFromPath(path: string): string {
  const parts = path.split('/').filter(Boolean)
  if (parts.length === 0) return path
  return parts[parts.length - 1]!
}

export function downloadFileName(path: string, isDir: boolean): string {
  const base = basenameFromPath(path) || 'download'
  return isDir ? `${base}.zip` : base
}
