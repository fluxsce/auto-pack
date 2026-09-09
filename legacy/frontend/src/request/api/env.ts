import type { EnvProbeResult, EnvSetting, EnvSettingPayload, EnvType } from '@/types'
import http from '../utils/http'

export function fetchEnvTypes(): Promise<{ types: EnvType[] }> {
  return http.get('/env/types')
}

export function fetchEnvSettings(envType?: EnvType): Promise<{ items: EnvSetting[] }> {
  const params = envType ? { env_type: envType } : undefined
  return http.get('/env/settings', { params })
}

export function createEnvSetting(data: EnvSettingPayload): Promise<EnvSetting> {
  return http.post('/env/settings', data)
}

export function updateEnvSetting(recId: string, data: EnvSettingPayload): Promise<EnvSetting> {
  return http.put(`/env/settings/${recId}`, data)
}

export function deleteEnvSetting(recId: string): Promise<{ message: string }> {
  return http.delete(`/env/settings/${recId}`)
}

export function verifyEnvSetting(recId: string): Promise<EnvProbeResult> {
  return http.post(`/env/settings/${recId}/verify`)
}

export interface SendMailPayload {
  recipients: string
  subject: string
  /** Plain-text fallback body. */
  body: string
  /** Optional HTML body; when set, sent as multipart/alternative with body. */
  html?: string
  /** Optional display name override for the SMTP From header. */
  from_name?: string
}

/** Send an ad-hoc email via a configured SMTP environment. */
export function sendEnvMail(
  smtpEnvRecId: string,
  data: SendMailPayload,
): Promise<{ ok: boolean }> {
  return http.post(`/env/settings/${smtpEnvRecId}/send-mail`, data)
}

export interface EnvProbePayload {
  env_type: EnvType
  config_json: string
  merge_from_rec_id?: string
}

/** Test draft config before save; merges secrets when merge_from_rec_id is set. */
export function probeEnvSetting(payload: EnvProbePayload): Promise<EnvProbeResult> {
  return http.post('/env/settings/probe', payload)
}

export function fetchMavenSettingsPaths(mavenHome?: string): Promise<{ items: string[] }> {
  return http.get('/env/maven-settings-paths', {
    params: mavenHome?.trim() ? { maven_home: mavenHome.trim() } : undefined,
  })
}

export function fetchNpmrcPaths(): Promise<{ items: string[] }> {
  return http.get('/env/npmrc-paths')
}
