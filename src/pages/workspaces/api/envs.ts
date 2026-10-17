import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import type { EnvRow } from '@/pages/envs/kinds'

export function listWorkspaceEnvs(_workspaceId?: string): Promise<EnvRow[]> {
  return request<EnvRow[]>({
    method: 'GET',
    url: apiPath('envs'),
  }).then((rows) => rows ?? [])
}

export type OSSSignResult = {
  ok?: boolean
  message?: string
  url?: string
  expires_at?: string
}

export function signOssObject(envId: string, key: string): Promise<{ url: string; expiresAt: string }> {
  return request<OSSSignResult>({
    method: 'POST',
    url: apiPath('envs', envId, 'oss-sign'),
    data: { key, mode: 'link' },
  }).then((out) => {
    if (!out?.ok || !out.url) throw new Error(out?.message || 'oss sign failed')
    return { url: out.url, expiresAt: out.expires_at || '' }
  })
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
