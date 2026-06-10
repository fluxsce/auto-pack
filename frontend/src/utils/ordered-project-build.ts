import type {
  MavenCyclicGroup,
  MavenInternalEdge,
  MavenWorkspaceModule,
} from '@/request/api/project'
import { fetchProjectBuild, triggerProjectBuild } from '@/request/api/build'
import type { PackProject } from '@/types'

const POLL_MS = 2000

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** Collect target projects and all workspace-internal Maven upstream providers. */
export function collectUpstreamRecIds(
  targetRecIds: string[],
  edges: MavenInternalEdge[] | undefined,
): Set<string> {
  const needed = new Set(targetRecIds.map((id) => id.trim()).filter(Boolean))
  if (!edges?.length || needed.size === 0) {
    return needed
  }
  const byConsumer = new Map<string, MavenInternalEdge[]>()
  for (const e of edges) {
    const list = byConsumer.get(e.consumer_rec_id) ?? []
    list.push(e)
    byConsumer.set(e.consumer_rec_id, list)
  }
  const stack = [...needed]
  while (stack.length > 0) {
    const id = stack.pop()!
    for (const e of byConsumer.get(id) ?? []) {
      if (!needed.has(e.provider_rec_id)) {
        needed.add(e.provider_rec_id)
        stack.push(e.provider_rec_id)
      }
    }
  }
  return needed
}

export type OrderProjectsOptions = {
  /** Empty = all modules in full_build_order (workspace-wide auto build). */
  targetRecIds?: string[]
  edges?: MavenInternalEdge[]
  /** Append non-Maven projects after Maven modules (workspace-wide). */
  includeNonMaven?: boolean
}

/** Topological build order: upstream modules first, then targets. */
export function orderProjectsForBuild(
  projects: PackProject[],
  fullBuildOrder: MavenWorkspaceModule[],
  opts: OrderProjectsOptions = {},
): PackProject[] {
  const targetRecIds = (opts.targetRecIds ?? []).map((id) => id.trim()).filter(Boolean)
  const byRecId = new Map(projects.map((p) => [p.rec_id, p]))
  const needed =
    targetRecIds.length === 0
      ? new Set(fullBuildOrder.map((m) => m.project_rec_id))
      : collectUpstreamRecIds(targetRecIds, opts.edges)

  const ordered: PackProject[] = []
  const seen = new Set<string>()

  for (const m of fullBuildOrder) {
    if (!needed.has(m.project_rec_id)) continue
    const p = byRecId.get(m.project_rec_id)
    if (p && !seen.has(p.rec_id)) {
      ordered.push(p)
      seen.add(p.rec_id)
    }
  }

  for (const id of targetRecIds) {
    if (seen.has(id)) continue
    const p = byRecId.get(id)
    if (p) {
      ordered.push(p)
      seen.add(id)
    }
  }

  if (opts.includeNonMaven && targetRecIds.length === 0) {
    for (const p of projects) {
      if (seen.has(p.rec_id)) continue
      if (String(p.project_type).toLowerCase() !== 'maven') {
        ordered.push(p)
        seen.add(p.rec_id)
      }
    }
  }

  return ordered
}

export type OrderedBuildOutcome = 'success' | 'failed' | 'skipped'

export type OrderedBuildCallbacks = {
  onProjectStart: (project: PackProject, index: number, total: number) => void
  onProjectDone: (
    project: PackProject,
    outcome: OrderedBuildOutcome,
    detail: { buildNum?: number; message?: string },
  ) => void
  shouldAbort?: () => boolean
}

export async function waitForBuildComplete(
  projectRecId: string,
  buildRecId: string,
): Promise<'success' | 'failed'> {
  for (;;) {
    const build = await fetchProjectBuild(projectRecId, buildRecId)
    if (build.status === 'success') return 'success'
    if (build.status === 'failed' || build.status === 'cancelled') return 'failed'
    await sleep(POLL_MS)
  }
}

export type OrderedBuildStepState = {
  buildRecId?: string
  outcome?: 'success' | 'failed'
  buildNum?: number
  message?: string
}

export type RunOrderedBuildsOptions = {
  startIndex?: number
  /** Per project rec_id — used to resume after page refresh. */
  stepStates?: Record<string, OrderedBuildStepState>
  onStepPersist?: (index: number, step: OrderedBuildStepState) => void
  /** When false, skip callbacks for steps already finished in stepStates (resume). */
  replayCompleted?: boolean
  cyclicGroups?: MavenCyclicGroup[]
  fullBuildOrder?: MavenWorkspaceModule[]
  onCyclicRetryStart?: (pass: number, maxPasses: number) => void
}

function cyclicRecIdSet(groups: MavenCyclicGroup[] | undefined): Set<string> {
  const out = new Set<string>()
  for (const g of groups ?? []) {
    for (const id of g.project_rec_ids ?? []) {
      const trimmed = id.trim()
      if (trimmed) out.add(trimmed)
    }
  }
  return out
}

function orderByFullBuildOrder(
  projects: PackProject[],
  fullBuildOrder: MavenWorkspaceModule[] | undefined,
): PackProject[] {
  if (!fullBuildOrder?.length) return projects
  const byRecId = new Map(projects.map((p) => [p.rec_id, p]))
  const out: PackProject[] = []
  const seen = new Set<string>()
  for (const m of fullBuildOrder) {
    const p = byRecId.get(m.project_rec_id)
    if (p && !seen.has(p.rec_id)) {
      out.push(p)
      seen.add(p.rec_id)
    }
  }
  for (const p of projects) {
    if (!seen.has(p.rec_id)) out.push(p)
  }
  return out
}

async function buildOneProject(
  project: PackProject,
  callbacks: OrderedBuildCallbacks,
  opts: RunOrderedBuildsOptions,
  index: number,
  total: number,
  prev?: OrderedBuildStepState,
): Promise<'success' | 'failed'> {
  callbacks.onProjectStart(project, index, total)

  try {
    let buildRecId = prev?.buildRecId
    let buildNum = prev?.buildNum

    if (!buildRecId) {
      const build = await triggerProjectBuild(project.rec_id)
      buildRecId = build.rec_id
      buildNum = build.build_num
      opts.onStepPersist?.(index, { buildRecId, buildNum })
    }

    const outcome = await waitForBuildComplete(project.rec_id, buildRecId)
    if (outcome === 'success') {
      const step: OrderedBuildStepState = { buildRecId, buildNum, outcome: 'success' }
      opts.onStepPersist?.(index, step)
      callbacks.onProjectDone(project, 'success', { buildNum })
      return 'success'
    }

    const latest = await fetchProjectBuild(project.rec_id, buildRecId)
    const step: OrderedBuildStepState = {
      buildRecId,
      buildNum,
      outcome: 'failed',
      message: latest.message?.trim(),
    }
    opts.onStepPersist?.(index, step)
    callbacks.onProjectDone(project, 'failed', { buildNum, message: step.message })
    return 'failed'
  } catch (e) {
    const step: OrderedBuildStepState = {
      outcome: 'failed',
      message: (e as Error).message,
    }
    opts.onStepPersist?.(index, step)
    callbacks.onProjectDone(project, 'failed', { message: step.message })
    return 'failed'
  }
}

/** Trigger builds sequentially in the given order (deps first). */
export async function runOrderedProjectBuilds(
  projects: PackProject[],
  callbacks: OrderedBuildCallbacks,
  opts: RunOrderedBuildsOptions = {},
): Promise<{ succeeded: number; failed: number }> {
  let succeeded = 0
  let failed = 0
  const total = projects.length
  const start = Math.max(0, opts.startIndex ?? 0)
  const replay = opts.replayCompleted ?? false

  for (let i = start; i < projects.length; i++) {
    if (callbacks.shouldAbort?.()) break
    const project = projects[i]!
    const prev = opts.stepStates?.[project.rec_id]

    if (prev?.outcome === 'success') {
      succeeded++
      if (replay) {
        callbacks.onProjectDone(project, 'success', { buildNum: prev.buildNum })
      }
      continue
    }
    if (prev?.outcome === 'failed') {
      failed++
      if (replay) {
        callbacks.onProjectDone(project, 'failed', {
          buildNum: prev.buildNum,
          message: prev.message,
        })
      }
      continue
    }

    const outcome = await buildOneProject(project, callbacks, opts, i, total, prev)
    if (outcome === 'success') succeeded++
    else failed++
  }

  const cyclicIDs = cyclicRecIdSet(opts.cyclicGroups)
  if (cyclicIDs.size > 0 && failed > 0) {
    const failedProjects = projects.filter((p) => {
      const st = opts.stepStates?.[p.rec_id]
      return st?.outcome === 'failed' && cyclicIDs.has(p.rec_id)
    })
    if (failedProjects.length > 0) {
      const maxPasses = Math.min(4, Math.max(2, failedProjects.length))
      const retryOrder = orderByFullBuildOrder(failedProjects, opts.fullBuildOrder)
      for (let pass = 2; pass <= maxPasses && failed > 0; pass++) {
        opts.onCyclicRetryStart?.(pass, maxPasses)
        let progress = false
        for (const project of retryOrder) {
          const st = opts.stepStates?.[project.rec_id]
          if (st?.outcome !== 'failed') continue
          const index = projects.findIndex((p) => p.rec_id === project.rec_id)
          if (index < 0) continue
          if (opts.stepStates) delete opts.stepStates[project.rec_id]
          const outcome = await buildOneProject(project, callbacks, opts, index, total)
          if (outcome === 'success') {
            succeeded++
            failed--
            progress = true
          }
        }
        if (!progress) break
      }
    }
  }

  return { succeeded, failed }
}
