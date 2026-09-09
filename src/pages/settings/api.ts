import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

export type SystemProps = {
  job_concurrency: number
  log_retention_days: number
  web_timeout_sec: number
}

export type GlobalVar = {
  rec_id: string
  key: string
  value: string
  is_enabled: number
  remark?: string
}

export function getProps(workspaceId: string): Promise<SystemProps> {
  return request<SystemProps>({
    method: 'GET',
    url: apiPath('settings', 'props'),
    params: { workspace_rec_id: workspaceId },
  })
}

export function saveProps(workspaceId: string, body: SystemProps): Promise<SystemProps> {
  return request<SystemProps>({
    method: 'PUT',
    url: apiPath('settings', 'props'),
    params: { workspace_rec_id: workspaceId },
    data: body,
  })
}

export function listVars(workspaceId: string): Promise<GlobalVar[]> {
  return request<GlobalVar[]>({
    method: 'GET',
    url: apiPath('settings', 'vars'),
    params: { workspace_rec_id: workspaceId },
  }).then((rows) => rows ?? [])
}

export function saveVar(workspaceId: string, body: Partial<GlobalVar> & { key: string }): Promise<GlobalVar> {
  return request<GlobalVar>({
    method: 'POST',
    url: apiPath('settings', 'vars'),
    params: { workspace_rec_id: workspaceId },
    data: body,
  })
}

export function deleteVar(workspaceId: string, key: string): Promise<void> {
  return request({
    method: 'DELETE',
    url: apiPath('settings', 'vars', key),
    params: { workspace_rec_id: workspaceId },
  })
}
