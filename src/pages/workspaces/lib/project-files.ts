import type { ProjectFileEntry } from '../api'

export function fileExt(name: string): string {
  const base = name.trim()
  const i = base.lastIndexOf('.')
  if (i <= 0 || i === base.length - 1) return ''
  return base.slice(i + 1).toLowerCase()
}

export function formatBytes(n?: number, empty = '—'): string {
  if (n == null || !Number.isFinite(n) || n < 0) return empty
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = n / 1024
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i += 1
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[i]}`
}

export function keepDirsFirst(left: ProjectFileEntry, right: ProjectFileEntry): number {
  if (left.is_dir !== right.is_dir) return left.is_dir ? -1 : 1
  return left.name.localeCompare(right.name, undefined, { sensitivity: 'base' })
}
