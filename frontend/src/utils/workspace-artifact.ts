import type { ProjectWorkspaceEntry } from '@/types'

const ARTIFACT_EXT = /\.(war|jar)$/i
const EXCLUDED_JAR = /-(sources|javadoc|original|tests)\.jar$/i

/** Zip root / folder name for merged DATAHUB static resources. */
export const VDATAHUB_STATIC_ROOT = 'sce-vdatahub'

/** Directory name is sce-vdatahub or sce-vdatahub-xxx / sce-vdatahub_xxx. */
export function isVdatahubStaticDirName(name: string): boolean {
  const n = name.trim().toLowerCase().replace(/\.zip$/i, '')
  return n === VDATAHUB_STATIC_ROOT || n.startsWith(`${VDATAHUB_STATIC_ROOT}-`) || n.startsWith(`${VDATAHUB_STATIC_ROOT}_`)
}

export function isVdatahubStaticPath(path: string): boolean {
  const parts = path.replaceAll('\\', '/').split('/').filter(Boolean)
  const last = parts.at(-1) ?? ''
  return isVdatahubStaticDirName(last)
}

export function vdatahubZipFileName(): string {
  return `${VDATAHUB_STATIC_ROOT}.zip`
}

/** Pick the best deployable war/jar, or an sce-vdatahub static directory. */
export function pickTargetArtifact(entries: ProjectWorkspaceEntry[]): ProjectWorkspaceEntry | null {
  const files = entries.filter((e) => !e.is_dir && ARTIFACT_EXT.test(e.name) && !EXCLUDED_JAR.test(e.name))
  if (files.length) {
    const war = files.find((e) => e.name.toLowerCase().endsWith('.war'))
    if (war) return war
    const byNameLen = [...files].sort((a, b) => a.name.length - b.name.length)
    return byNameLen[0] ?? null
  }
  const dirs = entries.filter((e) => e.is_dir && isVdatahubStaticDirName(e.name))
  if (!dirs.length) return null
  const exact = dirs.find((e) => e.name.toLowerCase() === VDATAHUB_STATIC_ROOT)
  if (exact) return exact
  const byName = [...dirs].sort((a, b) => a.name.localeCompare(b.name))
  return byName[0] ?? null
}
