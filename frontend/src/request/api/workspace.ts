import type { PackWorkspace, PackWorkspaceClonePayload, PackWorkspaceCloneResult, PackWorkspacePayload } from '@/types'
import type { MavenInternalEdge, MavenWorkspaceAnalysis, MavenWorkspaceModule } from './project'
import http from '../utils/http'

export type { MavenInternalEdge, MavenWorkspaceAnalysis }

export function fetchWorkspaces(): Promise<{ items: PackWorkspace[] }> {
  return http.get('/workspaces')
}

export function createWorkspace(data: PackWorkspacePayload): Promise<PackWorkspace> {
  return http.post('/workspaces', data)
}

export function updateWorkspace(recId: string, data: PackWorkspacePayload): Promise<PackWorkspace> {
  return http.put(`/workspaces/${recId}`, data)
}

export function deleteWorkspace(recId: string): Promise<{ message: string }> {
  return http.delete(`/workspaces/${recId}`)
}

/** Delete workspace and all projects/build records inside it. Irreversible. */
export function purgeWorkspace(recId: string): Promise<{ deleted_projects: number }> {
  return http.post(`/workspaces/${recId}/purge`)
}

/** Clone workspace metadata and all projects (configs, triggers, post-build) into a new workspace. */
export function cloneWorkspace(data: PackWorkspaceClonePayload): Promise<PackWorkspaceCloneResult> {
  return http.post('/workspaces/clone', data)
}

export type WorkspaceSourceSyncStatus = {
  workspace_rec_id?: string
  workspace_name?: string
  status: 'idle' | 'running' | 'completed' | string
  current_project?: string
  done_count?: number
  total_count?: number
  synced?: number
  skipped?: number
  failed?: number
  failed_items?: string[]
  foreign?: boolean
  active_workspace_name?: string
}

/** Start async git pre-sync for all projects in a workspace. Poll status separately. */
export function startWorkspaceSourceSync(workspaceRecId: string): Promise<{ status: string }> {
  return http.post(`/workspaces/${workspaceRecId}/sync-sources`)
}

/** Poll git pre-sync progress (use after startWorkspaceSourceSync). */
export function fetchWorkspaceSourceSyncStatus(
  workspaceRecId: string,
): Promise<WorkspaceSourceSyncStatus> {
  return http.get(`/workspaces/${workspaceRecId}/sync-sources/status`)
}

/** Analyze Maven module order and internal deps for all projects in a workspace. */
export function fetchWorkspaceMavenDependencyAnalysis(
  workspaceRecId: string,
  params?: { settings_xml?: string },
): Promise<{
  analysis: MavenWorkspaceAnalysis
  modules: MavenWorkspaceModule[]
  install_command: string
  workspace_rec_id: string
}> {
  return http.get(`/workspaces/${workspaceRecId}/maven-dependency-analysis`, { params })
}
