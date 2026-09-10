import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

export type AutoBuildStatus = {
  status: 'idle' | 'running' | 'completed' | string
  build_rec_id?: string
  workspace_rec_id?: string
  workspace_name?: string
  scope?: string
  succeeded?: number
  skipped?: number
  failed?: number
  current_project?: string
  done_count?: number
  total_count?: number
  phase?: string
}

export type AutoBuildStart = {
  status: string
  build_rec_id: string
  workspace_rec_id: string
  workspace_name?: string
  total_count?: number
}

export type WorkspaceBuildRecord = {
  rec_id: string
  workspace_rec_id: string
  build_num: number
  status: string
  scope?: string
  succeeded?: number
  skipped?: number
  failed?: number
  message?: string
  started_at?: string
  finished_at?: string
}

export function startAutoBuild(body: {
  workspace_rec_id: string
  scope: 'workspace' | 'selected'
  project_rec_ids?: string[]
  include_non_maven?: boolean
}): Promise<AutoBuildStart> {
  return request<AutoBuildStart>({ method: 'POST', url: apiPath('build'), data: body })
}

export function autoBuildStatus(workspaceId: string): Promise<AutoBuildStatus> {
  return request<AutoBuildStatus>({
    method: 'GET',
    url: apiPath('build', 'status'),
    params: { workspace_rec_id: workspaceId },
  })
}

export function autoBuildHistory(workspaceId: string): Promise<WorkspaceBuildRecord[]> {
  return request<{ items?: WorkspaceBuildRecord[] }>({
    method: 'GET',
    url: apiPath('build', 'history'),
    params: { workspace_rec_id: workspaceId },
  }).then((body) => body.items ?? [])
}

export function autoBuildLog(workspaceId: string, buildId: string, offset = 0): Promise<{ content: string; complete: boolean; status: string; build: WorkspaceBuildRecord }> {
  return request({
    method: 'GET',
    url: apiPath('build', 'history', buildId, 'log'),
    params: { workspace_rec_id: workspaceId, offset },
  })
}
