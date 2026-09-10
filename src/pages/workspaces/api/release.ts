import { apiPath } from '@/api/prefix'
import { notifyUnauthorized, request } from '@/api/http'

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
  phase?: string
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

export type SyncEvent = {
  event: 'start' | 'begin' | 'item' | 'done' | 'error' | string
  index?: number
  total?: number
  item?: SyncSourceItem
  synced?: number
  skipped?: number
  failed?: number
  message?: string
}

export async function syncWorkspaceSources(
  workspaceId: string,
  onEvent?: (ev: SyncEvent) => void,
): Promise<SyncSourcesResult> {
  const token = localStorage.getItem('ap_token')
  const res = await fetch(apiPath('release', 'sync-sources'), {
    method: 'POST',
    headers: {
      Accept: 'application/x-ndjson',
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ workspace_rec_id: workspaceId }),
  })
  const type = String(res.headers.get('content-type') || '')
  if (!res.ok) {
    if (res.status === 401) notifyUnauthorized()
    throw new Error(await readError(res))
  }
  if (type.includes('application/json') && !type.includes('ndjson')) {
    const body = (await res.json()) as SyncSourcesResult
    replaySyncResult(body, onEvent)
    return body
  }
  return readSyncStream(res, onEvent)
}

function replaySyncResult(body: SyncSourcesResult, onEvent?: (ev: SyncEvent) => void): void {
  if (!onEvent) return
  const items = body.items ?? []
  onEvent({ event: 'start', total: items.length })
  items.forEach((item, i) => {
    onEvent({ event: 'item', index: i + 1, total: items.length, item })
  })
  onEvent({ event: 'done', synced: body.synced, skipped: body.skipped, failed: body.failed, total: items.length })
}

async function readSyncStream(res: Response, onEvent?: (ev: SyncEvent) => void): Promise<SyncSourcesResult> {
  const out: SyncSourcesResult = { synced: 0, skipped: 0, failed: 0, items: [] }
  if (!res.body) {
    throw new Error('empty sync stream')
  }
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  for (;;) {
    const chunk = await reader.read()
    buf += decoder.decode(chunk.value || new Uint8Array(), { stream: !chunk.done })
    const lines = buf.split('\n')
    buf = chunk.done ? '' : (lines.pop() ?? '')
    for (const raw of lines) {
      const line = raw.trim()
      if (!line) continue
      const ev = JSON.parse(line) as SyncEvent
      if (ev.event === 'error') {
        throw new Error(ev.message || 'sync failed')
      }
      if (ev.event === 'item' && ev.item) {
        out.items = [...(out.items ?? []), ev.item]
        if (ev.item.outcome === 'failed') {
          out.failed_items = [...(out.failed_items ?? []), `${ev.item.project_name}: ${ev.item.message || ''}`]
        }
      }
      if (ev.event === 'done') {
        out.synced = ev.synced ?? out.synced
        out.skipped = ev.skipped ?? out.skipped
        out.failed = ev.failed ?? out.failed
      }
      onEvent?.(ev)
    }
    if (chunk.done) break
  }
  return out
}

async function readError(res: Response): Promise<string> {
  const text = await res.text()
  try {
    const parsed = JSON.parse(text) as { error?: string; message?: string }
    return parsed.error || parsed.message || text || res.statusText
  } catch {
    return text || res.statusText || 'request failed'
  }
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
