/** Format ISO build timestamp for display in project list / detail. */
export function formatBuildTime(iso?: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

/** Milliseconds for sorting; missing/invalid timestamps sort before valid ones. */
export function lastBuildAtSortKey(iso?: unknown): number {
  if (iso == null) return 0
  if (iso instanceof Date) {
    const ms = iso.getTime()
    return Number.isNaN(ms) ? 0 : ms
  }
  if (typeof iso !== 'string' && typeof iso !== 'number') return 0
  const raw = String(iso)
  if (!raw.trim()) return 0
  const ms = new Date(raw).getTime()
  return Number.isNaN(ms) ? 0 : ms
}
