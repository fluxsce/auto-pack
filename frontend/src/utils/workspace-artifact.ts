import type { ProjectWorkspaceEntry } from '@/types'

const ARTIFACT_EXT = /\.(war|jar)$/i
const EXCLUDED_JAR = /-(sources|javadoc|original|tests)\.jar$/i

/** Pick the best deployable war/jar from a workspace directory listing. */
export function pickTargetArtifact(entries: ProjectWorkspaceEntry[]): ProjectWorkspaceEntry | null {
  const files = entries.filter((e) => !e.is_dir && ARTIFACT_EXT.test(e.name) && !EXCLUDED_JAR.test(e.name))
  if (!files.length) return null
  const war = files.find((e) => e.name.toLowerCase().endsWith('.war'))
  if (war) return war
  return files.sort((a, b) => a.name.length - b.name.length)[0] ?? null
}
