import { apiPath } from '@/api/prefix'
import { downloadBlob, request } from '@/api/http'
import type { Project, ProjectWrite } from './types'

export function getProject(id: string): Promise<Project> {
  return request<Project>({ method: 'GET', url: apiPath('projects', id) })
}

export function listProjects(workspaceId: string): Promise<Project[]> {
  return request<Project[]>({
    method: 'GET',
    url: apiPath('projects'),
    params: { workspace_rec_id: workspaceId },
  }).then((rows) => rows ?? [])
}

export function saveProject(body: ProjectWrite): Promise<Project> {
  return request<Project>({ method: 'POST', url: apiPath('projects'), data: body })
}

export function cloneProject(id: string): Promise<Project> {
  return request<Project>({ method: 'POST', url: apiPath('projects', id, 'clone') })
}

export function deleteProject(id: string): Promise<void> {
  return request({ method: 'DELETE', url: apiPath('projects', id) })
}

export type GitBranchUpdate = {
  updated: number
  skipped: number
  branch: string
}

export type PostActionWrite = {
  family?: 'maven' | 'node' | ''
  kind: 'email' | 'shell'
  enabled?: boolean
  env_rec_id?: string
  command?: string
  when?: string
  to?: string
  subject?: string
  attach_log?: boolean
}

export type PostActionUpdate = {
  updated: number
  kind: string
  family?: string
}

export function updateWorkspacePostActions(workspaceId: string, action: PostActionWrite): Promise<PostActionUpdate> {
  const { family, ...body } = action
  return request<PostActionUpdate>({
    method: 'POST',
    url: apiPath('projects', 'post-actions'),
    data: { workspace_rec_id: workspaceId, family: family || undefined, action: body },
  })
}

export type CommandPatchWrite = {
  family?: 'maven' | 'node' | ''
  goal?: string
  find?: string
  replace?: string
  set_command?: string
  workdir?: string
  output_dir?: string
  artifact_pattern?: string
  log_retention_days?: number
}

export type CommandUpdate = {
  updated: number
  skipped: number
  family?: string
}

export function updateWorkspaceCommands(workspaceId: string, patch: CommandPatchWrite): Promise<CommandUpdate> {
  return request<CommandUpdate>({
    method: 'POST',
    url: apiPath('projects', 'commands'),
    data: { workspace_rec_id: workspaceId, patch },
  })
}

export function updateWorkspaceGitBranches(workspaceId: string, branch: string): Promise<GitBranchUpdate> {
  return request<GitBranchUpdate>({
    method: 'POST',
    url: apiPath('projects', 'git-branches'),
    data: { workspace_rec_id: workspaceId, repo_branch: branch },
  })
}

export type CronPreview = {
  expr: string
  ok: boolean
  error?: string
  times?: string[]
  tz?: string
}

export function previewCron(expr: string): Promise<CronPreview> {
  return request<CronPreview>({
    method: 'POST',
    url: apiPath('projects', 'cron-preview'),
    data: { expr },
  })
}

export type ProjectFileEntry = {
  name: string
  path: string
  is_dir: boolean
  size?: number
  mod_time?: string
}

export type ProjectTree = {
  root: string
  path: string
  exists: boolean
  entries: ProjectFileEntry[]
}

export type ProjectFileBlob = {
  path: string
  content: string
  truncated?: boolean
}

export function listProjectFiles(id: string, path = ''): Promise<ProjectTree> {
  return request<ProjectTree>({
    method: 'GET',
    url: apiPath('projects', id, 'files'),
    params: path ? { path } : undefined,
  })
}

export function readProjectFile(id: string, path: string): Promise<ProjectFileBlob> {
  return request<ProjectFileBlob>({
    method: 'GET',
    url: apiPath('projects', id, 'file'),
    params: { path },
  })
}

export type RemoteNoteView = {
  exists: boolean
  content: string
  info_path: string
}

export function fetchRemoteNote(
  projectId: string,
  body: { env_rec_id: string; push_path: string; file_name?: string },
): Promise<RemoteNoteView> {
  return request<RemoteNoteView>({
    method: 'POST',
    url: apiPath('projects', projectId, 'remote-note'),
    data: body,
    timeout: 20000,
  }).then((row) => ({
    exists: Boolean(row?.exists),
    content: row?.content ?? '',
    info_path: row?.info_path ?? '',
  }))
}

export function clearProjectFiles(id: string): Promise<void> {
  return request({ method: 'POST', url: apiPath('projects', id, 'files', 'clear') })
}

export function deleteProjectFile(id: string, path: string): Promise<void> {
  return request({ method: 'DELETE', url: apiPath('projects', id, 'file'), params: { path } })
}

export function downloadProjectFile(id: string, path: string, filename: string): Promise<void> {
  return downloadBlob(
    {
      method: 'GET',
      url: apiPath('projects', id, 'file', 'download'),
      params: { path },
      timeout: 180000,
    },
    filename,
  )
}

export type ProjectDepRow = {
  name: string
  version?: string
  extra?: string
  source?: string
  depth?: number
  note?: string
  key?: string
}

export type ProjectDeps = {
  exists: boolean
  cached?: boolean
  sources?: string[]
  direct?: number
  rows: ProjectDepRow[]
}

export function listProjectDeps(id: string, refresh = false): Promise<ProjectDeps> {
  return request<ProjectDeps>({
    method: 'GET',
    url: apiPath('projects', id, 'deps'),
    params: refresh ? { refresh: 1 } : undefined,
    timeout: 180000,
  }).then((body) => ({
    exists: Boolean(body?.exists),
    cached: Boolean(body?.cached),
    sources: body?.sources ?? [],
    direct: body?.direct ?? 0,
    rows: (body?.rows ?? []).map((row, i) => ({ ...row, key: `${i}:${row.source || ''}:${row.name}` })),
  }))
}
