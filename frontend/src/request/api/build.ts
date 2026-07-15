import type {
  PackBuild,
  PackBuildLog,
  ProjectWorkspaceFile,
  ProjectWorkspaceListing,
} from '@/types'
import http from '../utils/http'

export function fetchProjectBuilds(
  projectRecId: string,
  limit = 30,
): Promise<{ items: PackBuild[] }> {
  return http.get(`/projects/${projectRecId}/builds`, { params: { limit } })
}

export function triggerProjectBuild(projectRecId: string): Promise<PackBuild> {
  return http.post(`/projects/${projectRecId}/builds`)
}

export function fetchProjectBuild(projectRecId: string, buildRecId: string): Promise<PackBuild> {
  return http.get(`/projects/${projectRecId}/builds/${buildRecId}`)
}

export function fetchProjectBuildLog(
  projectRecId: string,
  buildRecId: string,
  offset = 0,
): Promise<PackBuildLog> {
  return http.get(`/projects/${projectRecId}/builds/${buildRecId}/log`, { params: { offset } })
}

export function fetchProjectWorkspace(
  projectRecId: string,
  path = '.',
): Promise<ProjectWorkspaceListing> {
  return http.get(`/projects/${projectRecId}/workspace`, { params: { path } })
}

export function fetchProjectWorkspaceFile(
  projectRecId: string,
  path: string,
): Promise<ProjectWorkspaceFile> {
  return http.get(`/projects/${projectRecId}/workspace/file`, { params: { path } })
}

export function deleteProjectWorkspace(
  projectRecId: string,
  path = '.',
): Promise<{ message: string }> {
  return http.delete(`/projects/${projectRecId}/workspace`, { params: { path } })
}

export interface WorkspaceFtpPushPayload {
  path: string
  ftp_env_rec_id: string
  push_path: string
  backup_path?: string
  pusher: string
  task_ref: string
  detail?: string
}

export interface WorkspaceFtpPushResult {
  backup_path: string
  update_path: string
  info_file_path: string
  info_content: string
}

export interface WorkspaceFtpPushStartResponse {
  task_id: string
  status: 'running'
}

export interface WorkspaceFtpPushStatusResponse {
  task_id: string
  project_rec_id?: string
  status: 'idle' | 'running' | 'success' | 'failed'
  message?: string
  path?: string
  push_path?: string
  started_at?: string
  finished_at?: string
  log_content?: string
  result?: WorkspaceFtpPushResult
}

export function pushProjectWorkspaceFtp(
  projectRecId: string,
  data: WorkspaceFtpPushPayload,
): Promise<WorkspaceFtpPushStartResponse> {
  return http.post(`/projects/${projectRecId}/workspace/ftp-push`, data)
}

export function fetchProjectWorkspaceFtpPushStatus(
  projectRecId: string,
  taskId: string,
): Promise<WorkspaceFtpPushStatusResponse> {
  return http.get(`/projects/${projectRecId}/workspace/ftp-push/status`, { params: { task_id: taskId } })
}

export interface WorkspaceFtpRemoteNote {
  exists: boolean
  content: string
  info_path: string
}

export function fetchProjectWorkspaceFtpRemoteNote(
  projectRecId: string,
  params: { ftp_env_rec_id: string; push_path: string; file_name?: string },
): Promise<WorkspaceFtpRemoteNote> {
  return http.get(`/projects/${projectRecId}/workspace/ftp-remote-note`, { params })
}
