import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

export type ReleasePostOptions = {
  auto_build_after_release: boolean
  auto_ftp_push_after_build: boolean
  ftp_push_project_ids: string[]
  ftp_env_rec_id: string
  push_path: string
  backup_path: string
  backup_manual: boolean
  pusher: string
  task_ref: string
  detail: string
  release_date?: string
  update_pom_version?: boolean
  create_target_branch?: boolean
  target_branch?: string
}

export const RELEASE_OK = 'ok'

export type ReleaseResultItem = {
  project_name: string
  status: 'ok' | 'skipped' | 'failed' | string
  message?: string
  steps?: string[]
}

export type AutoReleaseStatus = {
  status: 'idle' | 'running' | 'completed' | string
  release_rec_id?: string
  workspace_rec_id?: string
  workspace_name?: string
  tag?: string
  release_branch?: string
  released?: number
  skipped?: number
  failed?: number
  results?: ReleaseResultItem[]
  current_project?: string
  done_count?: number
  total_count?: number
  foreign?: boolean
  active_workspace_name?: string
  post_options?: ReleasePostOptions
}

export type AutoReleaseStart = {
  status: string
  release_rec_id: string
  workspace_rec_id: string
  workspace_name?: string
  release_branch?: string
  tag?: string
}

export type AutoReleaseDefaults = {
  release_branch: string
  tag_branch: string
  tag_source?: string
}

export type WorkspaceReleaseRecord = {
  rec_id: string
  workspace_rec_id: string
  release_num: number
  status: string
  release_branch?: string
  tag_branch?: string
  target_branch?: string
  released?: number
  skipped?: number
  failed?: number
  message?: string
  started_at?: string
  finished_at?: string
}

export type MavenModule = {
  project_rec_id: string
  project_name: string
  coordinates?: string
}

export type MavenEdge = {
  consumer_rec_id: string
  consumer_name: string
  provider_rec_id: string
  provider_name: string
}

export type MavenCycle = {
  project_rec_ids: string[]
  project_names?: string[]
}

export type MavenAnalysis = {
  full_build_order: MavenModule[]
  internal_edges: MavenEdge[]
  cyclic_groups?: MavenCycle[]
  warnings?: string[]
}

export type SyncSourceItem = {
  project_name: string
  outcome: 'synced' | 'skipped' | 'failed' | string
  action?: string
  branch?: string
  message?: string
  log?: string
}

export type SyncSourcesResult = {
  synced: number
  skipped: number
  failed: number
  items?: SyncSourceItem[]
  failed_items?: string[]
}

export function startAutoRelease(body: {
  workspace_rec_id: string
  release_branch: string
  tag_branch: string
  release_note?: string
  release_date?: string
  allow_repeat?: boolean
  update_pom_version?: boolean
  create_target_branch?: boolean
  target_branch?: string
  post_options?: ReleasePostOptions
}): Promise<AutoReleaseStart> {
  return request<AutoReleaseStart>({ method: 'POST', url: apiPath('release'), data: body })
}

export function autoReleaseStatus(workspaceId: string): Promise<AutoReleaseStatus> {
  return request<AutoReleaseStatus>({
    method: 'GET',
    url: apiPath('release', 'status'),
    params: { workspace_rec_id: workspaceId },
  })
}

export function autoReleaseDefaults(workspaceId: string): Promise<AutoReleaseDefaults> {
  return request<AutoReleaseDefaults>({
    method: 'GET',
    url: apiPath('release', 'defaults'),
    params: { workspace_rec_id: workspaceId },
  })
}

export function saveAutoReleasePostOptions(workspaceId: string, post_options: ReleasePostOptions): Promise<{ release_rec_id: string }> {
  return request({
    method: 'PUT',
    url: apiPath('release', 'post-options'),
    data: { workspace_rec_id: workspaceId, post_options },
  })
}

export function autoReleaseHistory(workspaceId: string): Promise<WorkspaceReleaseRecord[]> {
  return request<{ items?: WorkspaceReleaseRecord[] }>({
    method: 'GET',
    url: apiPath('release', 'history'),
    params: { workspace_rec_id: workspaceId },
  }).then((body) => body.items ?? [])
}

export function autoReleaseLog(workspaceId: string, releaseId: string, offset = 0): Promise<{ content: string; complete: boolean; release: WorkspaceReleaseRecord }> {
  return request({
    method: 'GET',
    url: apiPath('release', 'history', releaseId, 'log'),
    params: { workspace_rec_id: workspaceId, offset },
  })
}

export function syncWorkspaceSources(workspaceId: string): Promise<SyncSourcesResult> {
  return request<SyncSourcesResult>({
    method: 'POST',
    url: apiPath('release', 'sync-sources'),
    data: { workspace_rec_id: workspaceId },
    timeout: 180000,
  })
}

export function fetchMavenOrder(workspaceId: string): Promise<MavenAnalysis> {
  return request<{ analysis: MavenAnalysis }>({
    method: 'GET',
    url: apiPath('release', 'maven-order'),
    params: { workspace_rec_id: workspaceId },
  }).then((body) => body.analysis)
}

export function fetchServerBootAt(): Promise<string> {
  return request<{ started_at?: string }>({ method: 'GET', url: apiPath('health') }).then((body) => body.started_at || '')
}
