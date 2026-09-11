import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

export type JobRow = {
  rec_id: string
  workspace_rec_id?: string
  project_rec_id?: string
  job_type: string
  job_status: string
  payload_json?: string
  error_text?: string
  created_at?: string
  started_at?: string
  finished_at?: string
}

export type JobStep = {
  rec_id: string
  step_key: string
  step_name: string
  step_status: string
  error_text?: string
  started_at?: string
  finished_at?: string
}

export function isBuildJob(row: Pick<JobRow, 'job_type'>): boolean {
  return row.job_type === 'build'
}

export function jobOrigin(row: Pick<JobRow, 'payload_json'>): string {
  const raw = row.payload_json?.trim()
  if (!raw) return ''
  try {
    const body = JSON.parse(raw) as { origin?: string }
    return String(body.origin || '').trim()
  } catch {
    return ''
  }
}

export type JobStats = {
  running: number
  queued: number
  succeeded: number
  failed: number
  canceled?: number
}

export type JobListPage = {
  items: JobRow[]
  live: JobRow[]
  stats: JobStats
  total: number
  page: number
  page_size: number
}

export type JobListQuery = {
  workspaceId?: string
  projectId?: string
  jobType?: string
  page?: number
  pageSize?: number
}

export function listJobsPage(q: JobListQuery): Promise<JobListPage> {
  return request<JobListPage>({
    method: 'GET',
    url: apiPath('jobs'),
    params: {
      workspace_rec_id: q.workspaceId || undefined,
      project_rec_id: q.projectId || undefined,
      job_type: q.jobType || undefined,
      page: q.page ?? 1,
      page_size: q.pageSize ?? 20,
    },
  }).then((res) => ({
    items: res?.items ?? [],
    live: res?.live ?? [],
    stats: res?.stats ?? { running: 0, queued: 0, succeeded: 0, failed: 0 },
    total: res?.total ?? 0,
    page: res?.page ?? 1,
    page_size: res?.page_size ?? q.pageSize ?? 20,
  }))
}

export function listJobs(workspaceId: string, opts?: { projectId?: string; jobType?: string; pageSize?: number }): Promise<JobRow[]> {
  return listJobsPage({
    workspaceId,
    projectId: opts?.projectId,
    jobType: opts?.jobType,
    page: 1,
    pageSize: opts?.pageSize ?? 100,
  }).then((page) => page.items)
}

export type JobPayload = {
  tag?: string
  branch?: string
  from_branch?: string
  force?: boolean
  notes?: string
  pom_version?: string
  update_pom?: boolean
  command?: string
  env_rec_id?: string
  remote_path?: string
  backup_path?: string
  pusher?: string
  task_ref?: string
  detail?: string
  artifact?: string
  artifacts?: string[]
  merge_sources?: { project_rec_id: string; path: string }[]
}

export type BatchSubmitResult = {
  queued: number
  skipped: number
  job_ids?: string[]
}

export function submitWorkspaceJobs(
  workspaceId: string,
  jobType: string,
  payload: JobPayload,
): Promise<BatchSubmitResult> {
  return request<BatchSubmitResult>({
    method: 'POST',
    url: apiPath('jobs', 'batch'),
    data: {
      workspace_rec_id: workspaceId,
      job_type: jobType,
      payload,
    },
  })
}

export function submitBuild(workspaceId: string, projectId: string): Promise<JobRow> {
  return request<JobRow>({
    method: 'POST',
    url: apiPath('jobs'),
    data: {
      workspace_rec_id: workspaceId,
      project_rec_id: projectId,
      job_type: 'build',
      payload: {},
    },
  })
}

export function submitJob(workspaceId: string, projectId: string, jobType: string, payload: JobPayload = {}): Promise<JobRow> {
  return request<JobRow>({
    method: 'POST',
    url: apiPath('jobs'),
    data: {
      workspace_rec_id: workspaceId,
      project_rec_id: projectId,
      job_type: jobType,
      payload,
    },
  })
}

const terminal = new Set(['succeeded', 'failed', 'canceled', 'cancelled'])

export type JobView = { job: JobRow; steps?: JobStep[] }

export async function waitForJob(
  id: string,
  intervalMs = 2000,
  onTick?: (view: JobView) => void,
): Promise<JobRow> {
  for (;;) {
    const view = await getJob(id)
    onTick?.(view)
    const status = view.job.job_status
    if (terminal.has(status)) {
      return view.job
    }
    await new Promise((resolve) => setTimeout(resolve, intervalMs))
  }
}

export function getJob(id: string): Promise<JobView> {
  return request<JobView>({ method: 'GET', url: apiPath('jobs', id) })
}

export type JobLogView = {
  log: string
  size: number
  offset: number
}

export function getJobLog(id: string, offset = 0): Promise<JobLogView> {
  return request<JobLogView>({
    method: 'GET',
    url: apiPath('jobs', id, 'log'),
    params: offset > 0 ? { offset } : undefined,
  }).then((res) => ({
    log: res.log || '',
    size: Number(res.size) || 0,
    offset: Number(res.offset) || 0,
  }))
}

export function applyJobLog(current: string, view: JobLogView): { text: string; size: number } {
  if (view.offset === 0) {
    return { text: view.log, size: view.size }
  }
  return { text: current + view.log, size: view.size }
}

export function cancelJob(id: string): Promise<void> {
  return request({ method: 'POST', url: apiPath('jobs', id, 'cancel') })
}
