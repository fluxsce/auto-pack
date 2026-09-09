/** Path-safe workspace code: letter first; then letters, digits, `.`, `_`, `-`; no `..`. */
export const WORKSPACE_CODE_PATTERN = /^[a-zA-Z][a-zA-Z0-9._-]{0,31}$/

export function isValidWorkspaceCode(code: string): boolean {
  const trimmed = code.trim()
  if (!WORKSPACE_CODE_PATTERN.test(trimmed)) return false
  return !trimmed.includes('..')
}

/** Suggest a new workspace code when cloning from an existing one. */
export function suggestCloneWorkspaceCode(code: string): string {
  const base = code.trim()
  if (!base) return ''
  for (const candidate of [`${base}-copy`, `${base}.copy`, `${base}_copy`, `${base}2`]) {
    if (isValidWorkspaceCode(candidate)) return candidate
  }
  const trimmed = base.slice(0, 24)
  const fallback = `${trimmed}-copy`
  return isValidWorkspaceCode(fallback) ? fallback : trimmed
}
