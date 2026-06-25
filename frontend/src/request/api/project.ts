import type { PackProject, PackProjectPayload, ProjectType } from '@/types'
import http from '../utils/http'

export function fetchProjectTypes(): Promise<{ types: ProjectType[] }> {
  return http.get('/project/types')
}

export interface FetchProjectsParams {
  workspace_rec_id?: string
  project_type?: ProjectType
}

export function fetchProjects(params?: FetchProjectsParams): Promise<{ items: PackProject[] }> {
  return http.get('/projects', { params })
}

export function fetchProject(recId: string): Promise<PackProject> {
  return http.get(`/projects/${recId}`)
}

export interface MavenModuleCoords {
  group_id: string
  artifact_id: string
  version?: string
}

export interface MavenWorkspaceModule {
  project_rec_id: string
  project_name: string
  project_version: string
  pom_path: string
  coordinates: MavenModuleCoords
  dependencies?: MavenModuleCoords[]
}

export interface MavenDependencyAnalysis {
  target_rec_id: string
  build_order: MavenWorkspaceModule[]
  skipped_installed?: MavenWorkspaceModule[]
  missing_pom?: string[]
  warnings?: string[]
}

export interface MavenInternalEdge {
  consumer_rec_id: string
  consumer_name: string
  provider_rec_id: string
  provider_name: string
  dependency: MavenModuleCoords
}

export interface MavenCyclicGroup {
  project_rec_ids: string[]
  project_names?: string[]
}

export interface MavenWorkspaceAnalysis {
  module_count: number
  full_build_order: MavenWorkspaceModule[]
  build_order: MavenWorkspaceModule[]
  skipped_installed?: MavenWorkspaceModule[]
  internal_edges?: MavenInternalEdge[]
  cyclic_groups?: MavenCyclicGroup[]
  missing_pom?: string[]
  warnings?: string[]
}

export function fetchMavenDependencyAnalysis(
  projectRecId: string,
  params?: { settings_xml?: string },
): Promise<{
  analysis: MavenDependencyAnalysis
  modules: MavenWorkspaceModule[]
  install_command: string
  workspace_rec_id: string
}> {
  return http.get(`/projects/${projectRecId}/maven-dependency-analysis`, { params })
}

export function createProject(data: PackProjectPayload): Promise<PackProject> {
  return http.post('/projects', data)
}

export function updateProject(recId: string, data: PackProjectPayload): Promise<PackProject> {
  return http.put(`/projects/${recId}`, data)
}

export function deleteProject(recId: string): Promise<{ message: string }> {
  return http.delete(`/projects/${recId}`)
}

export interface CronPreviewResult {
  next_runs: string[]
  timezone: string
}

/** Preview next scheduled build times using server-side cron parser (robfig). */
export function previewProjectCron(data: { cron: string; count?: number }): Promise<CronPreviewResult> {
  return http.post('/projects/cron-preview', data)
}

export interface BatchRepoBranchPayload {
  workspace_rec_id: string
  repo_branch: string
}

export function batchUpdateRepoBranch(
  data: BatchRepoBranchPayload,
): Promise<{ updated: number }> {
  return http.post('/projects/batch-repo-branch', data)
}

export interface BatchProjectVersionPayload {
  workspace_rec_id: string
  project_version: string
}

export function batchUpdateProjectVersion(
  data: BatchProjectVersionPayload,
): Promise<{ updated: number }> {
  return http.post('/projects/batch-project-version', data)
}

export interface BatchPomVersionPayload {
  workspace_rec_id: string
  version_tag: string
  commit_message?: string
  allow_repeat?: boolean
}

export interface BatchPomVersionResultItem {
  project_name: string
  status: string
  message?: string
  pom_files?: number
  steps?: string[]
}

export interface BatchPomVersionStartResponse {
  status: 'running'
  batch_rec_id: string
  workspace_rec_id: string
  workspace_name?: string
  tag?: string
}

export type WorkspacePomVersionStatus = 'running' | 'success' | 'partial' | 'failed'

export interface WorkspacePomVersionRecord {
  rec_id: string
  workspace_rec_id: string
  batch_num: number
  status: WorkspacePomVersionStatus
  version_tag: string
  commit_message?: string
  updated: number
  skipped: number
  failed: number
  message?: string
  log_path?: string
  started_at?: string
  finished_at?: string
  created_at: string
}

export interface WorkspacePomVersionLogResponse {
  content: string
  offset: number
  size: number
  total: number
  complete: boolean
  status: WorkspacePomVersionStatus
  batch: WorkspacePomVersionRecord
}

export type BatchPomVersionJobStatus = 'idle' | 'running' | 'completed'

export interface BatchPomVersionStatusResponse {
  status: BatchPomVersionJobStatus
  batch_rec_id?: string
  workspace_rec_id?: string
  workspace_name?: string
  tag?: string
  commit_message?: string
  started_at?: string
  finished_at?: string
  updated?: number
  skipped?: number
  failed?: number
  results?: BatchPomVersionResultItem[]
  current_project?: string
  done_count?: number
  total_count?: number
  foreign?: boolean
  active_workspace_name?: string
}

/** Starts async pom version batch; poll status until completed. */
export function batchUpdatePomVersion(
  data: BatchPomVersionPayload,
): Promise<BatchPomVersionStartResponse> {
  return http.post('/projects/batch-pom-version', data)
}

export function batchPomVersionStatus(
  workspaceRecId: string,
): Promise<BatchPomVersionStatusResponse> {
  return http.get('/projects/batch-pom-version/status', {
    params: { workspace_rec_id: workspaceRecId },
  })
}

export function fetchWorkspacePomVersionHistory(
  workspaceRecId: string,
  limit = 30,
): Promise<{ items: WorkspacePomVersionRecord[] }> {
  return http.get('/projects/batch-pom-version/history', {
    params: { workspace_rec_id: workspaceRecId, limit },
  })
}

export function fetchWorkspacePomVersionLog(
  workspaceRecId: string,
  batchRecId: string,
  offset = 0,
): Promise<WorkspacePomVersionLogResponse> {
  return http.get(`/projects/batch-pom-version/history/${batchRecId}/log`, {
    params: { workspace_rec_id: workspaceRecId, offset },
  })
}

export interface BatchPostBuildEmailPayload {
  workspace_rec_id: string
  enabled: boolean
  when: string
  env_rec_id: string
  recipients: string
  subject: string
  body: string
  name?: string
}

export function batchUpdatePostBuildEmail(
  data: BatchPostBuildEmailPayload,
): Promise<{ updated: number }> {
  return http.post('/projects/batch-post-build-email', data)
}

export interface BatchBuildStepsPayload {
  workspace_rec_id: string
  project_type?: string
  update_build_cmd: boolean
  build_cmd?: string
  update_output_dir: boolean
  output_dir?: string
  update_artifact_pattern: boolean
  artifact_pattern?: string
  update_log_retention: boolean
  log_retention_days?: number
}

export function batchUpdateBuildSteps(
  data: BatchBuildStepsPayload,
): Promise<{ updated: number }> {
  return http.post('/projects/batch-build-steps', data)
}

export interface BatchReleasePostOptions {
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

export interface BatchAutoReleasePayload {
  workspace_rec_id: string
  release_branch: string
  tag_branch: string
  release_note?: string
  release_date?: string
  allow_repeat?: boolean
  update_pom_version?: boolean
  create_target_branch?: boolean
  target_branch?: string
  post_options?: BatchReleasePostOptions
}

export interface BatchAutoReleaseResultItem {
  project_name: string
  status: 'ok' | 'skipped' | 'failed'
  message?: string
  steps?: string[]
}

export interface BatchAutoReleaseResponse {
  released: number
  skipped: number
  failed: number
  results: BatchAutoReleaseResultItem[]
}

export interface BatchAutoReleaseStartResponse {
  status: 'running'
  release_rec_id: string
  workspace_rec_id: string
  workspace_name?: string
  release_branch?: string
  tag?: string
}

export type WorkspaceReleaseStatus = 'running' | 'success' | 'partial' | 'failed'

export interface WorkspaceReleaseRecord {
  rec_id: string
  workspace_rec_id: string
  release_num: number
  status: WorkspaceReleaseStatus
  release_branch: string
  tag_branch: string
  target_branch?: string
  released: number
  skipped: number
  failed: number
  message?: string
  log_path?: string
  started_at?: string
  finished_at?: string
  created_at: string
}

export interface WorkspaceReleaseLogResponse {
  content: string
  offset: number
  size: number
  total: number
  complete: boolean
  status: WorkspaceReleaseStatus
  release: WorkspaceReleaseRecord
}

export type BatchAutoReleaseJobStatus = 'idle' | 'running' | 'completed'

export interface BatchAutoReleaseStatusResponse {
  status: BatchAutoReleaseJobStatus
  release_rec_id?: string
  workspace_rec_id?: string
  workspace_name?: string
  tag?: string
  release_branch?: string
  started_at?: string
  finished_at?: string
  released?: number
  skipped?: number
  failed?: number
  results?: BatchAutoReleaseResultItem[]
  current_project?: string
  done_count?: number
  total_count?: number
  foreign?: boolean
  active_workspace_name?: string
  post_options?: BatchReleasePostOptions
}

/** Starts async auto-release; poll status until completed. No long HTTP timeout. */
export interface BatchAutoReleaseDefaultsResponse {
  release_branch: string
  tag_branch: string
  tag_source?: 'remote' | 'history' | 'workspace'
  post_options?: BatchReleasePostOptions
}

export function fetchBatchAutoReleaseDefaults(
  workspaceRecId: string,
): Promise<BatchAutoReleaseDefaultsResponse> {
  return http.get('/projects/batch-auto-release/defaults', {
    params: { workspace_rec_id: workspaceRecId },
  })
}

export function batchAutoRelease(
  data: BatchAutoReleasePayload,
): Promise<BatchAutoReleaseStartResponse> {
  return http.post('/projects/batch-auto-release', data)
}

export function batchAutoReleaseStatus(
  workspaceRecId: string,
): Promise<BatchAutoReleaseStatusResponse> {
  return http.get('/projects/batch-auto-release/status', {
    params: { workspace_rec_id: workspaceRecId },
  })
}

export function saveBatchAutoReleasePostOptions(
  workspaceRecId: string,
  postOptions: BatchReleasePostOptions,
): Promise<{ release_rec_id: string; status: string }> {
  return http.put('/projects/batch-auto-release/post-options', {
    workspace_rec_id: workspaceRecId,
    post_options: postOptions,
  })
}

export function fetchWorkspaceReleaseHistory(
  workspaceRecId: string,
  limit = 30,
): Promise<{ items: WorkspaceReleaseRecord[] }> {
  return http.get('/projects/batch-auto-release/history', {
    params: { workspace_rec_id: workspaceRecId, limit },
  })
}

export function fetchWorkspaceReleaseLog(
  workspaceRecId: string,
  releaseRecId: string,
  offset = 0,
): Promise<WorkspaceReleaseLogResponse> {
  return http.get(`/projects/batch-auto-release/history/${releaseRecId}/log`, {
    params: { workspace_rec_id: workspaceRecId, offset },
  })
}
