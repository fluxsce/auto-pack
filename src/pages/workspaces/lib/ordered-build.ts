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

/** 从 pom.xml / package.json 图里取出目标工程的工作空间上游模块（不含自身）。 */
export function upstreamModules(projectId: string, analysis: MavenAnalysis | null | undefined): MavenModule[] {
  if (!analysis || !projectId) return []
  const needed = new Set<string>([projectId])
  collectUpstream(analysis.internal_edges ?? [], needed)
  needed.delete(projectId)
  const byId = new Map((analysis.full_build_order ?? []).map((m) => [m.project_rec_id, m]))
  const out: MavenModule[] = []
  for (const m of analysis.full_build_order ?? []) {
    if (!needed.has(m.project_rec_id)) continue
    out.push(m)
    needed.delete(m.project_rec_id)
  }
  for (const id of needed) {
    const m = byId.get(id)
    if (m) out.push(m)
  }
  return out
}

/** 目标工程 pom.xml 里对不上工作空间的 SNAPSHOT / 同组依赖。 */
export function unmatchedCoords(projectId: string, analysis: MavenAnalysis | null | undefined): string[] {
  if (!analysis || !projectId) return []
  const seen = new Set<string>()
  const out: string[] = []
  for (const row of analysis.unmatched_deps ?? []) {
    if (row.consumer_rec_id !== projectId) continue
    const coord = row.coordinates.trim()
    if (!coord || seen.has(coord)) continue
    seen.add(coord)
    out.push(coord)
  }
  return out
}

export function joinPreview(items: string[], max = 8): string {
  if (items.length <= max) return items.join('、')
  return `${items.slice(0, max).join('、')} 等 ${items.length} 个`
}

const liveJob = new Set(['queued', 'running', 'succeeded'])

/** 工作空间上游里尚未成功（也未在跑）的模块，立即构建时需要补齐。 */
export function missingUpstreamModules(
  projectId: string,
  analysis: MavenAnalysis | null | undefined,
): { missing: MavenModule[]; disabled: MavenModule[] } {
  const missing: MavenModule[] = []
  const disabled: MavenModule[] = []
  const seen = new Set<string>()
  for (const m of upstreamModules(projectId, analysis)) {
    if (seen.has(m.project_rec_id)) continue
    seen.add(m.project_rec_id)
    if (liveJob.has((m.last_job_status || '').trim())) continue
    if (m.is_enabled === 0) {
      disabled.push(m)
      continue
    }
    missing.push(m)
  }
  return { missing, disabled }
}
