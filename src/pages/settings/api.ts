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

export function getProps(): Promise<SystemProps> {
  return request<SystemProps>({
    method: 'GET',
    url: apiPath('settings', 'props'),
  })
}

export function saveProps(body: SystemProps): Promise<SystemProps> {
  return request<SystemProps>({
    method: 'PUT',
    url: apiPath('settings', 'props'),
    data: body,
  })
}

export function listVars(): Promise<GlobalVar[]> {
  return request<GlobalVar[]>({
    method: 'GET',
    url: apiPath('settings', 'vars'),
  }).then((rows) => rows ?? [])
}

export function saveVar(body: Partial<GlobalVar> & { key: string }): Promise<GlobalVar> {
  return request<GlobalVar>({
    method: 'POST',
    url: apiPath('settings', 'vars'),
    data: body,
  })
}

export function deleteVar(key: string): Promise<void> {
  return request({
    method: 'DELETE',
    url: apiPath('settings', 'vars', key),
  })
}

export type UpgradeStatus = {
  version: string
  goos: string
  goarch: string
  upgradable: boolean
  message?: string
  install_root?: string
}

export function getUpgradeStatus(): Promise<UpgradeStatus> {
  return request<UpgradeStatus>({
    method: 'GET',
    url: apiPath('settings', 'upgrade'),
  })
}

/** 上传发行包并安排重启；成功后服务会退出，前端不必再刷新状态。 */
export function upgradeServer(file: File, sha256: string): Promise<{ message: string }> {
  const body = new FormData()
  body.append('file', file)
  body.append('sha256', sha256)
  return request<{ message: string }>({
    method: 'POST',
    url: apiPath('settings', 'upgrade'),
    data: body,
    timeout: 30 * 60 * 1000,
  })
}
