import type { PackWorkspace } from '@/types'
import { effectiveWorkspaceRoot } from './workspace/workspace-paths'

/** Derive path-safe directory segment from project name (matches backend projectcfg.SlugFromProjectName). */
export function slugFromProjectName(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return ''

  let slug = trimmed.replace(/\s+/g, '-').replace(/[^a-zA-Z0-9_.-]/g, '')

  if (!slug) {
    let hash = 0
    for (let i = 0; i < trimmed.length; i++) {
      hash = (hash * 31 + trimmed.charCodeAt(i)) >>> 0
    }
    slug = `p${(hash % 1_000_000_000).toString(36)}`
  }

  if (!/^[a-zA-Z]/.test(slug)) slug = `p${slug}`

  return slug.slice(0, 64)
}

/** Relative project path under workspace root: {slug(name)}. */
export function derivedProjectWorkDir(projectName: string): string {
  return slugFromProjectName(projectName)
}

/** Display absolute/resolved path for work directory hint. */
export function resolvedWorkPath(
  ws: Pick<PackWorkspace, 'root_dir' | 'effective_root_dir' | 'workspace_code'> | null | undefined,
  workDir: string,
): string {
  const rel = workDir.trim()
  if (!rel) return ''
  const root = ws ? effectiveWorkspaceRoot(ws) : 'data'
  return `${root}/${rel}`.replace(/\/+/g, '/')
}
