import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import type { Workspace } from '@/stores/workspace'
import type { WorkspaceWrite } from './types'

export function createWorkspace(body: Pick<WorkspaceWrite, 'workspace_code' | 'workspace_name'>): Promise<Workspace> {
  return request<Workspace>({ method: 'POST', url: apiPath('workspaces'), data: body })
}

export function updateWorkspace(id: string, body: Pick<WorkspaceWrite, 'workspace_name' | 'remark'>): Promise<Workspace> {
  return request<Workspace>({ method: 'PUT', url: apiPath('workspaces', id), data: body })
}

export function cloneWorkspace(body: {
  source_rec_id: string
  workspace_code: string
  workspace_name: string
}): Promise<Workspace> {
  return request<Workspace>({ method: 'POST', url: apiPath('workspaces', 'clone'), data: body })
}

export function deleteWorkspace(id: string): Promise<void> {
  return request({ method: 'DELETE', url: apiPath('workspaces', id) })
}

export type WorkspaceBundle = {
  format_version: number
  exported_at?: string
  source_workspace_rec_id?: string
  workspace: {
    workspace_code: string
    workspace_name: string
    remark?: string
  }
  projects: unknown[]
}

export type WorkspaceImportResult = Workspace & {
  projects_imported: number
  mode: string
  format_version: number
}

export function exportWorkspace(id: string): Promise<WorkspaceBundle> {
  return request<WorkspaceBundle>({ method: 'GET', url: apiPath('workspaces', id, 'export') })
}

export function importWorkspace(body: {
  mode: 'new' | 'replace'
  target_workspace_rec_id?: string
  workspace_code?: string
  workspace_name?: string
  bundle: WorkspaceBundle
}): Promise<WorkspaceImportResult> {
  return request<WorkspaceImportResult>({ method: 'POST', url: apiPath('workspaces', 'import'), data: body })
}

export function downloadWorkspaceBundle(bundle: WorkspaceBundle): void {
  const code = (bundle.workspace?.workspace_code || 'workspace').replace(/[^A-Za-z0-9._-]+/g, '-')
  const stamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d+Z$/, 'Z').slice(0, 15)
  const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${code}-autopack-${stamp}.json`
  link.click()
  URL.revokeObjectURL(url)
}
