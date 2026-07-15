/** Strip refs/tags/ prefixes (aligned with backend git.NormalizeTagName). */
/** Canonical repo_branch for a version tag (refs/tags/NAME). */
export function formatTagRef(raw: string): string {
  const name = normalizeGitTagName(raw)
  return name ? `refs/tags/${name}` : ''
}

export function normalizeGitTagName(raw: string): string {
  let s = raw.trim()
  for (let i = 0; i < 8; i++) {
    const before = s
    for (const prefix of ['refs/tags/', 'tags/', 'tag/']) {
      if (s.startsWith(prefix)) {
        s = s.slice(prefix.length).trim()
      }
    }
    if (s.startsWith('/')) s = s.slice(1).trim()
    if (s === before) break
  }
  return s.trim()
}

/**
 * Lightweight git tag name check (mirrors `git check-ref-format --allow-onelevel`).
 * Backend re-validates on submit.
 */
export function isValidGitTagName(raw: string): boolean {
  const tag = normalizeGitTagName(raw)
  if (!tag) return false
  if (tag.includes('..') || tag.includes('@{')) return false
  if (tag.startsWith('.') || tag.endsWith('.') || tag.endsWith('.lock') || tag.endsWith('/')) {
    return false
  }
  if (/[\x00-\x1f\x7f~^:?*[\]\\ ]/.test(tag)) return false
  return true
}
