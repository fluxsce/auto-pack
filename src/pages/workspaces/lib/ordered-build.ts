import type { Project } from '../api'
import type { MavenAnalysis, MavenEdge, MavenModule } from '../api/release'

export function isOrderedBuildType(type: string): boolean {
  const code = type.trim().toLowerCase()
  return code === 'maven' || code === 'npm' || code === 'node' || code === 'nodejs' || code === 'vue' || code === 'frontend'
}

export function orderProjectsForBuild(
  projects: Project[],
  fullBuildOrder: MavenModule[],
  opts: { targetRecIds?: string[]; edges?: MavenEdge[]; includeNonMaven?: boolean },
): Project[] {
  const byId = new Map(projects.map((p) => [p.rec_id, p]))
  const targets = new Set(opts.targetRecIds ?? [])
  const needed = new Set<string>()
  if (targets.size === 0) {
    for (const m of fullBuildOrder) needed.add(m.project_rec_id)
    for (const p of projects) {
      if (isOrderedBuildType(p.project_type)) needed.add(p.rec_id)
    }
  } else {
    for (const id of targets) needed.add(id)
    collectUpstream(opts.edges ?? [], needed)
  }
  const out: Project[] = []
  const seen = new Set<string>()
  for (const m of fullBuildOrder) {
    if (!needed.has(m.project_rec_id) || seen.has(m.project_rec_id)) continue
    const p = byId.get(m.project_rec_id)
    if (!p) continue
    out.push(p)
    seen.add(p.rec_id)
  }
  const leftover = [...needed]
    .filter((id) => !seen.has(id))
    .map((id) => byId.get(id))
    .filter((p): p is Project => !!p)
    .sort((a, b) => a.project_name.localeCompare(b.project_name))
  for (const p of leftover) {
    out.push(p)
    seen.add(p.rec_id)
  }
  if (targets.size === 0 && opts.includeNonMaven) {
    const extras = projects
      .filter((p) => !seen.has(p.rec_id) && !isOrderedBuildType(p.project_type))
      .sort((a, b) => a.project_name.localeCompare(b.project_name))
    for (const p of extras) {
      out.push(p)
      seen.add(p.rec_id)
    }
  }
  return out
}

function collectUpstream(edges: MavenEdge[], needed: Set<string>): void {
  let grew = true
  while (grew) {
    grew = false
    for (const e of edges) {
      if (!needed.has(e.consumer_rec_id) || needed.has(e.provider_rec_id)) continue
      needed.add(e.provider_rec_id)
      grew = true
    }
  }
}

export function cyclicRecIds(analysis: MavenAnalysis): Set<string> {
  const ids = new Set<string>()
  for (const g of analysis.cyclic_groups ?? []) {
    for (const id of g.project_rec_ids ?? []) ids.add(id)
  }
  return ids
}
