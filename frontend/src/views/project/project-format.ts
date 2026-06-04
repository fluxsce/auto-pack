/** Format ISO build timestamp for display in project list / detail. */
export function formatBuildTime(iso?: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

/** Milliseconds for sorting; missing/invalid timestamps sort before valid ones. */
export function lastBuildAtSortKey(iso?: string | null): number {
  if (!iso) return 0
  const ms = new Date(iso).getTime()
  return Number.isNaN(ms) ? 0 : ms
}
