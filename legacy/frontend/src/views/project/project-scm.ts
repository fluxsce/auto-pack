import type { PackProject, PackWorkspace, ProjectBuildConfig, ProjectType } from '@/types'
import { mergeBuildConfig } from './project-defaults'
import { derivedProjectWorkDir, resolvedWorkPath } from './project-paths'
import { effectiveWorkspaceRoot } from './workspace/workspace-paths'

export type ScmMode = 'local' | 'git'

export function effectiveScmMode(
  project: Pick<PackProject, 'repo_url' | 'git_env_rec_id'> | null | undefined,
  buildConfig: Pick<ProjectBuildConfig, 'scm_mode'> | null | undefined,
): ScmMode {
  const mode = String(buildConfig?.scm_mode ?? '').toLowerCase()
  if (mode === 'git' || mode === 'local') return mode
  if (project?.repo_url?.trim() || project?.git_env_rec_id?.trim()) return 'git'
  return 'local'
}

export function usesGitSource(buildConfig: Pick<ProjectBuildConfig, 'scm_mode'>): boolean {
  return effectiveScmMode(null, buildConfig) === 'git'
}

/** Display path used for builds (local override or workspace project dir). */
export function resolveProjectSourceDisplay(
  project: Pick<PackProject, 'project_name' | 'project_type' | 'config_json'> | null | undefined,
  ws: Pick<PackWorkspace, 'root_dir' | 'effective_root_dir' | 'workspace_code'> | null | undefined,
  buildConfig?: Pick<ProjectBuildConfig, 'local_source_path'> | null,
): string {
  if (!project) return ''
  const cfg =
    buildConfig ?? mergeBuildConfig(project.project_type as ProjectType, project.config_json)
  const override = cfg.local_source_path?.trim()
  const workPath = resolvedWorkPath(ws, derivedProjectWorkDir(project.project_name))
  if (!override) return workPath
  if (/^([a-zA-Z]:[\\/]|\/)/.test(override)) return override.replace(/\\/g, '/')
  if (!ws) return override
  const root = effectiveWorkspaceRoot(ws)
  return `${root.replace(/\\/g, '/')}/${override.replace(/^\/+/, '')}`
}
