import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import type { EnvRow } from '@/pages/envs/kinds'

export function listWorkspaceEnvs(workspaceId: string): Promise<EnvRow[]> {
  return request<EnvRow[]>({
    method: 'GET',
    url: apiPath('envs'),
    params: { workspace_rec_id: workspaceId },
  }).then((rows) => rows ?? [])
}

export function getWorkspaceEnv(id: string): Promise<EnvRow> {
  return request<EnvRow>({ method: 'GET', url: apiPath('envs', id) })
}

export type EnvMailPayload = {
  recipients: string
  subject: string
  body: string
  html?: string
  from_name?: string
}

export function sendEnvMail(id: string, payload: EnvMailPayload): Promise<void> {
  return request({
    method: 'POST',
    url: apiPath('envs', id, 'mail'),
    data: payload,
  })
}
