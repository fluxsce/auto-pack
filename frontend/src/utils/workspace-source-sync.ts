import {
  fetchWorkspaceSourceSyncStatus,
  startWorkspaceSourceSync,
  type WorkspaceSourceSyncStatus,
} from '@/request/api/workspace'

const POLL_MS = 1500

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** Start async git pre-sync and poll until completed (no long HTTP timeout). */
export async function waitWorkspaceSourceSync(
  workspaceRecId: string,
  onProgress?: (status: WorkspaceSourceSyncStatus) => void,
): Promise<WorkspaceSourceSyncStatus> {
  await startWorkspaceSourceSync(workspaceRecId)
  for (;;) {
    const st = await fetchWorkspaceSourceSyncStatus(workspaceRecId)
    onProgress?.(st)
    if (st.foreign) {
      const name = st.active_workspace_name?.trim() || ''
      const err = new Error(name)
      err.name = 'WorkspaceSourceSyncForeign'
      throw err
    }
    if (st.status === 'completed') {
      return st
    }
    if (st.status === 'idle') {
      return st
    }
    await sleep(POLL_MS)
  }
}
