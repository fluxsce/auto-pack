import { fetchAppHealth } from '@/request/api/app'
import { fetchProjectBuild } from '@/request/api/build'
import { randomId } from '@/utils/random-id'

export type AutoBuildScope = 'workspace' | 'selected'

export type AutoBuildStepState = {
  buildRecId?: string
  outcome?: 'success' | 'failed'
  buildNum?: number
  message?: string
}

export type AutoBuildSession = {
  version: 1
  workspaceRecId: string
  jobId: string
  status: 'running' | 'completed'
  scope: AutoBuildScope
  selectedRecIds: string[]
  includeNonMaven: boolean
  planRecIds: string[]
  currentIndex: number
  steps: Record<string, AutoBuildStepState>
  logText: string
  succeeded: number
  failed: number
  /** Backend process boot time when the session started. */
  serverBootAt: string
  updatedAt: string
}

const STORAGE_PREFIX = 'autopack:auto-build:'

let cachedServerBootAt: string | null = null

function storageKey(workspaceRecId: string): string {
  return STORAGE_PREFIX + workspaceRecId.trim()
}

export function isInterruptedBuildMessage(message: string | undefined | null): boolean {
  const m = (message ?? '').toLowerCase()
  return m.includes('interrupted') || m.includes('server restarted') || m.includes('build runner not active')
}

export async function fetchServerBootAt(): Promise<string> {
  if (cachedServerBootAt) return cachedServerBootAt
  const health = await fetchAppHealth()
  const boot = health.started_at?.trim() ?? ''
  if (boot) cachedServerBootAt = boot
  return boot
}

export function loadAutoBuildSession(workspaceRecId: string): AutoBuildSession | null {
  const id = workspaceRecId.trim()
  if (!id) return null
  try {
    const raw = sessionStorage.getItem(storageKey(id))
    if (!raw) return null
    const parsed = JSON.parse(raw) as AutoBuildSession
    if (parsed.version !== 1 || parsed.workspaceRecId !== id) return null
    return parsed
  } catch {
    return null
  }
}

export function saveAutoBuildSession(session: AutoBuildSession): void {
  const id = session.workspaceRecId.trim()
  if (!id) return
  session.updatedAt = new Date().toISOString()
  try {
    sessionStorage.setItem(storageKey(id), JSON.stringify(session))
  } catch {
    /* quota or private mode */
  }
}

export function clearAutoBuildSession(workspaceRecId: string): void {
  const id = workspaceRecId.trim()
  if (!id) return
  try {
    sessionStorage.removeItem(storageKey(id))
  } catch {
    /* ignore */
  }
}

/** Returns true when the session must not resume (e.g. backend restarted). */
export async function isAutoBuildSessionStale(session: AutoBuildSession): Promise<boolean> {
  const bootAt = await fetchServerBootAt()
  if (!session.serverBootAt?.trim()) {
    return true
  }
  if (bootAt && session.serverBootAt !== bootAt) {
    return true
  }
  for (const [projectRecId, step] of Object.entries(session.steps)) {
    if (!step.buildRecId || step.outcome === 'success') continue
    try {
      const build = await fetchProjectBuild(projectRecId, step.buildRecId)
      if (build.status === 'failed' && isInterruptedBuildMessage(build.message)) {
        return true
      }
    } catch {
      /* ignore fetch errors */
    }
  }
  return false
}

export async function hasRunningAutoBuildSession(workspaceRecId: string): Promise<boolean> {
  const session = loadAutoBuildSession(workspaceRecId)
  if (session?.status !== 'running') return false
  if (await isAutoBuildSessionStale(session)) {
    clearAutoBuildSession(workspaceRecId)
    return false
  }
  return true
}

export function createAutoBuildSession(
  workspaceRecId: string,
  serverBootAt: string,
  partial: Pick<
    AutoBuildSession,
    'scope' | 'selectedRecIds' | 'includeNonMaven' | 'planRecIds' | 'logText'
  >,
): AutoBuildSession {
  return {
    version: 1,
    workspaceRecId: workspaceRecId.trim(),
    jobId: randomId(),
    status: 'running',
    scope: partial.scope,
    selectedRecIds: [...partial.selectedRecIds],
    includeNonMaven: partial.includeNonMaven,
    planRecIds: [...partial.planRecIds],
    currentIndex: 0,
    steps: {},
    logText: partial.logText,
    succeeded: 0,
    failed: 0,
    serverBootAt: serverBootAt.trim(),
    updatedAt: new Date().toISOString(),
  }
}
