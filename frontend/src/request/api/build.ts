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

export function pushProjectWorkspaceFtp(
  projectRecId: string,
  data: WorkspaceFtpPushPayload,
): Promise<WorkspaceFtpPushResult> {
  return http.post(`/projects/${projectRecId}/workspace/ftp-push`, data)
}
