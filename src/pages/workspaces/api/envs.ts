import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import type { EnvRow } from '@/pages/envs/kinds'

export function listWorkspaceEnvs(_workspaceId?: string): Promise<EnvRow[]> {
  return request<EnvRow[]>({
    method: 'GET',
    url: apiPath('envs'),
  }).then((rows) => rows ?? [])
}

export function getWorkspaceEnv(id: string): Promise<EnvRow> {
  return request<EnvRow>({ method: 'GET', url: apiPath('envs', id) })
}

export type EnvMailPayload = {
  recipients: string
  subject?: string
  body?: string
  html?: string
  from_name?: string
  /** Built-in template: ftp_update | project_update */
  template?: string
  fields?: Record<string, string>
}

export function sendEnvMail(id: string, payload: EnvMailPayload): Promise<void> {
  return request({
    method: 'POST',
    url: apiPath('envs', id, 'mail'),
    data: payload,
  })
}
