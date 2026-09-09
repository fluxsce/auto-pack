const prefix = 'autopack:auto-build:'

export type AutoBuildScope = 'workspace' | 'selected'

export type OrderedBuildStepState = {
  buildRecId?: string
  outcome?: 'success' | 'failed'
  buildNum?: number
  message?: string
}

export type AutoBuildSession = {
  version: 1
  workspaceRecId: string
  status: 'running' | 'completed'
  scope: AutoBuildScope
  selectedRecIds: string[]
  includeNonMaven: boolean
  planRecIds: string[]
  currentIndex: number
  steps: Record<string, OrderedBuildStepState>
  logText: string
  succeeded: number
  failed: number
  serverBootAt: string
  updatedAt: number
}

export function sessionKey(workspaceId: string): string {
  return prefix + workspaceId
}

export function loadAutoBuildSession(workspaceId: string): AutoBuildSession | null {
  try {
    const raw = sessionStorage.getItem(sessionKey(workspaceId))
    if (!raw) return null
    const parsed = JSON.parse(raw) as AutoBuildSession
    if (parsed.version !== 1 || parsed.workspaceRecId !== workspaceId) return null
    return parsed
  } catch {
    return null
  }
}

export function saveAutoBuildSession(session: AutoBuildSession): void {
  session.updatedAt = Date.now()
  sessionStorage.setItem(sessionKey(session.workspaceRecId), JSON.stringify(session))
}

export function clearAutoBuildSession(workspaceId: string): void {
  sessionStorage.removeItem(sessionKey(workspaceId))
}

export function createAutoBuildSession(
  workspaceId: string,
  serverBootAt: string,
  init: Pick<AutoBuildSession, 'scope' | 'selectedRecIds' | 'includeNonMaven' | 'planRecIds' | 'logText'>,
): AutoBuildSession {
  return {
    version: 1,
    workspaceRecId: workspaceId,
    status: 'running',
    currentIndex: 0,
    steps: {},
    succeeded: 0,
    failed: 0,
    serverBootAt,
    updatedAt: Date.now(),
    ...init,
  }
}

export async function isAutoBuildSessionStale(session: AutoBuildSession, bootAt: string): Promise<boolean> {
  if (!session.serverBootAt || !bootAt || session.serverBootAt !== bootAt) return true
  return false
}

export function hasRunningAutoBuildSession(workspaceId: string): boolean {
  return loadAutoBuildSession(workspaceId)?.status === 'running'
}
