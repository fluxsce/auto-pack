import {
  fetchWorkspaceSourceSyncStatus,
  startWorkspaceSourceSync,
  type WorkspaceSourceSyncStatus,
} from '@/request/api/workspace'

const POLL_MS = 1500

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/**
 * Start async git pre-sync and poll until completed (no long HTTP timeout).
 *
 * Different workspaces may sync in parallel (server git.sync_max_workspaces).
 * Capacity exhaustion is reported as HTTP 409 from startWorkspaceSourceSync
 * (message already localized by the API). Status foreign is legacy and ignored.
 */
export async function waitWorkspaceSourceSync(
  workspaceRecId: string,
  onProgress?: (status: WorkspaceSourceSyncStatus) => void,
): Promise<WorkspaceSourceSyncStatus> {
  await startWorkspaceSourceSync(workspaceRecId)
  for (;;) {
    const st = await fetchWorkspaceSourceSyncStatus(workspaceRecId)
    onProgress?.(st)
    if (st.status === 'completed') {
      return st
    }
    if (st.status === 'idle') {
      return st
    }
    await sleep(POLL_MS)
  }
}
