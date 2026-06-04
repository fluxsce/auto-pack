import type { PackWorkspace, PackWorkspaceClonePayload, PackWorkspaceCloneResult, PackWorkspacePayload } from '@/types'
import http from '../utils/http'

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

/** Clone workspace metadata and all projects (configs, triggers, post-build) into a new workspace. */
export function cloneWorkspace(data: PackWorkspaceClonePayload): Promise<PackWorkspaceCloneResult> {
  return http.post('/workspaces/clone', data)
}
