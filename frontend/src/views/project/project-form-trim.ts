import type { PackProject, PackProjectPayload } from '@/types'

/** Trim leading/trailing whitespace on project identity and Git fields. */
export function normalizeProjectFormFields(form: {
  workspace_rec_id: string
  project_name: string
  project_version: string
  repo_url: string
  repo_branch: string
  remark: string
  git_env_rec_id?: string
  jdk_env_rec_id?: string
  maven_env_rec_id?: string
  nodejs_env_rec_id?: string
  deploy_env_rec_id?: string
}): void {
  form.workspace_rec_id = form.workspace_rec_id.trim()
  form.project_name = form.project_name.trim()
  form.project_version = form.project_version.trim() || '1.0.0'
  form.repo_url = form.repo_url.trim()
  form.repo_branch = form.repo_branch.trim() || 'main'
  form.remark = form.remark.trim()
  if (form.git_env_rec_id != null) form.git_env_rec_id = String(form.git_env_rec_id).trim()
  if (form.jdk_env_rec_id != null) form.jdk_env_rec_id = String(form.jdk_env_rec_id).trim()
  if (form.maven_env_rec_id != null) form.maven_env_rec_id = String(form.maven_env_rec_id).trim()
  if (form.nodejs_env_rec_id != null) form.nodejs_env_rec_id = String(form.nodejs_env_rec_id).trim()
  if (form.deploy_env_rec_id != null) form.deploy_env_rec_id = String(form.deploy_env_rec_id).trim()
}

export function trimProjectFromApi(row: PackProject): PackProject {
  return {
    ...row,
    project_name: row.project_name.trim(),
    project_version: row.project_version.trim(),
    repo_url: row.repo_url.trim(),
    repo_branch: row.repo_branch.trim() || 'main',
    remark: row.remark.trim(),
    git_env_rec_id: (row.git_env_rec_id ?? '').trim(),
    jdk_env_rec_id: (row.jdk_env_rec_id ?? '').trim(),
    maven_env_rec_id: (row.maven_env_rec_id ?? '').trim(),
    nodejs_env_rec_id: (row.nodejs_env_rec_id ?? '').trim(),
    deploy_env_rec_id: (row.deploy_env_rec_id ?? '').trim(),
  }
}

