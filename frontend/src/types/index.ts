export type ThemeMode = 'light' | 'dark' | 'system'

/** 与后端 auth.UserView JSON 字段一致 */
export interface UserView {
  rec_id: string
  login_name: string
}

export interface LoginResult {
  token: string
  expires_at: string
  user: UserView
}

export interface ChangePasswordPayload {
  old_password: string
  new_password: string
}

export interface ChangeUsernamePayload {
  password: string
  new_username: string
}

/** Admin user row from AP_USER_AUTH */
export interface UserAccount {
  rec_id: string
  login_name: string
  is_enabled: number
  remark: string
  created_at: string
  updated_at: string
}

export interface UserAccountPayload {
  login_name: string
  password?: string
  is_enabled: number
  remark: string
}

export interface UserResetPasswordPayload {
  new_password: string
}

export interface OperationLogEntry {
  rec_id: string
  user_rec_id: string
  login_name: string
  action: string
  module: string
  target_type: string
  target_id: string
  target_name: string
  summary: string
  detail: string
  request_method: string
  request_path: string
  client_ip: string
  success: number
  error_message: string
  created_at: string
}

export type EnvType = 'ssh' | 'sftp' | 'ftp' | 'git' | 'maven' | 'nodejs' | 'jdk' | 'smtp'

export interface EnvSetting {
  rec_id: string
  env_name: string
  env_type: EnvType
  config_json: string
  is_default: number
  is_enabled: number
  sort_num: number
  remark: string
  created_at: string
  updated_at: string
}

export interface EnvProbeResult {
  ok: boolean
  message: string
  details?: Record<string, string>
}

export interface EnvSettingPayload {
  env_name: string
  env_type: EnvType
  config_json: string
  is_default: number
  is_enabled: number
  sort_num: number
  remark: string
}

export type ProjectType = 'maven' | 'npm' | 'vue' | 'go' | 'gradle' | 'custom'

export type {
  PostBuildAction,
  PostBuildActionType,
  PostBuildTransferSet,
  PostBuildWhen,
} from './post-build'

export type { BuildSchedule, BuildTriggers } from './build-triggers'

export interface ProjectBuildConfig {
  scm_mode?: 'local' | 'git'
  /** Optional override; empty uses workspace work_dir */
  local_source_path?: string
  build_cmd: string
  output_dir: string
  artifact_pattern: string
  /** Days to keep build history and log files; default 3. */
  log_retention_days: number
  post_build_actions: import('./post-build').PostBuildAction[]
  build_triggers: import('./build-triggers').BuildTriggers
}

export interface PackWorkspace {
  rec_id: string
  workspace_code: string
  workspace_name: string
  root_dir: string
  /** Resolved root when root_dir is empty (application data/). */
  effective_root_dir?: string
  is_default: number
  is_enabled: number
  sort_num: number
  remark: string
  created_at: string
  updated_at: string
}

export interface PackWorkspacePayload {
  workspace_code: string
  workspace_name: string
  root_dir: string
  is_default: number
  is_enabled: number
  sort_num: number
  remark: string
}

export interface PackWorkspaceClonePayload extends PackWorkspacePayload {
  source_rec_id: string
}

export interface PackWorkspaceCloneResult extends PackWorkspace {
  projects_cloned: number
}

export interface WorkspaceBundleProject {
  source_rec_id?: string
  project_name: string
  project_version: string
  project_type: ProjectType
  repo_url: string
  repo_branch: string
  work_dir: string
  config_json: string
  git_env_rec_id: string
  jdk_env_rec_id: string
  maven_env_rec_id: string
  nodejs_env_rec_id: string
  deploy_env_rec_id: string
  is_enabled: number
  sort_num: number
  remark: string
}

export interface WorkspaceBundle {
  format_version: number
  exported_at: string
  source_workspace_rec_id?: string
  workspace: {
    workspace_code: string
    workspace_name: string
    root_dir: string
    is_enabled: number
    sort_num: number
    remark: string
  }
  projects: WorkspaceBundleProject[]
}

export type WorkspaceImportMode = 'new' | 'replace'

export interface WorkspaceImportPayload {
  mode: WorkspaceImportMode
  target_workspace_rec_id?: string
  workspace_code?: string
  workspace_name?: string
  root_dir?: string
  bundle: WorkspaceBundle
}

export interface WorkspaceImportResult extends PackWorkspace {
  projects_imported: number
  mode: WorkspaceImportMode
}

export interface PackProject {
  rec_id: string
  workspace_rec_id: string
  project_name: string
  project_version: string
  project_type: ProjectType
  repo_url: string
  repo_branch: string
  /** Relative path under workspace root; derived from project name + version */
  work_dir: string
  config_json: string
  git_env_rec_id: string
  jdk_env_rec_id: string
  maven_env_rec_id: string
  nodejs_env_rec_id: string
  deploy_env_rec_id: string
  is_enabled: number
  sort_num: number
  remark: string
  created_at: string
  updated_at: string
  /** Latest build in this workspace (project list API only) */
  last_build_status?: PackBuildStatus | null
  last_build_num?: number | null
  last_build_at?: string | null
}

export interface PackProjectPayload {
  workspace_rec_id: string
  project_name: string
  project_version: string
  project_type: ProjectType
  repo_url: string
  repo_branch: string
  /** Omit or empty; server derives path from project name + version */
  work_dir?: string
  config_json: string
  git_env_rec_id: string
  jdk_env_rec_id: string
  maven_env_rec_id: string
  nodejs_env_rec_id: string
  deploy_env_rec_id: string
  is_enabled: number
  sort_num: number
  remark: string
}

export type PackBuildStatus = 'pending' | 'running' | 'success' | 'failed' | 'cancelled'

export interface PackBuild {
  rec_id: string
  project_rec_id: string
  build_num: number
  status: PackBuildStatus
  message: string
  log_path: string
  started_at?: string
  finished_at?: string
  created_at: string
}

export interface PackBuildLog {
  content: string
  offset: number
  size: number
  total: number
  complete: boolean
  status: PackBuildStatus
  build: PackBuild
}

export interface ProjectWorkspaceEntry {
  name: string
  path: string
  is_dir: boolean
  size: number
}

export interface ProjectWorkspaceListing {
  path: string
  abs_path: string
  exists: boolean
  entries: ProjectWorkspaceEntry[]
}

export interface ProjectWorkspaceFile {
  path: string
  abs_path: string
  size: number
  content: string
  truncated: boolean
  readable: boolean
}
