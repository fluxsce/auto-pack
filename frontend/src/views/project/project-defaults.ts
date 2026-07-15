import type { ProjectBuildConfig, ProjectType } from '@/types'
import { DEFAULT_BUILD_TRIGGERS } from '@/types/build-triggers'
import { parseBuildTriggers } from './build-triggers/build-triggers-defaults'
import { parsePostBuildActions } from './post-build/post-build-defaults'

export const DEFAULT_LOG_RETENTION_DAYS = 3

const emptyPostBuild = { post_build_actions: [] as ProjectBuildConfig['post_build_actions'] }
const emptyTriggers = { build_triggers: structuredClone(DEFAULT_BUILD_TRIGGERS) }
const defaultLogRetention = { log_retention_days: DEFAULT_LOG_RETENTION_DAYS }

export const DEFAULT_BUILD_CONFIG: Record<ProjectType, ProjectBuildConfig> = {
  maven: {
    build_cmd: 'mvn clean package -DskipTests',
    output_dir: 'target',
    artifact_pattern: '*.jar',
    ...defaultLogRetention,
    ...emptyPostBuild,
    ...emptyTriggers,
  },
  npm: {
    build_cmd: 'npm run build',
    output_dir: 'dist',
    artifact_pattern: '*',
    ...defaultLogRetention,
    ...emptyPostBuild,
    ...emptyTriggers,
  },
  vue: {
    build_cmd: 'npm run build',
    output_dir: 'dist',
    artifact_pattern: '*',
    ...defaultLogRetention,
    ...emptyPostBuild,
    ...emptyTriggers,
  },
  go: {
    build_cmd: 'go build -o bin/app ./...',
    output_dir: 'bin',
    artifact_pattern: '*',
    ...defaultLogRetention,
    ...emptyPostBuild,
    ...emptyTriggers,
  },
  gradle: {
    build_cmd: './gradlew build',
    output_dir: 'build/libs',
    artifact_pattern: '*.jar',
    ...defaultLogRetention,
    ...emptyPostBuild,
    ...emptyTriggers,
  },
  custom: {
    build_cmd: '',
    output_dir: '',
    artifact_pattern: '',
    ...defaultLogRetention,
    ...emptyPostBuild,
    ...emptyTriggers,
  },
}

/** Merge stored config_json with type defaults; empty stored fields keep defaults. */
export function mergeBuildConfig(type: ProjectType, raw: string): ProjectBuildConfig {
  const defaults = DEFAULT_BUILD_CONFIG[type]
  try {
    const stored = JSON.parse(raw || '{}') as Partial<ProjectBuildConfig>
    const scmMode = stored.scm_mode === 'git' ? 'git' : stored.scm_mode === 'local' ? 'local' : undefined
    const localSourcePath =
      typeof stored.local_source_path === 'string' ? stored.local_source_path.trim() : ''
    const rawDays = (stored as Record<string, unknown>).log_retention_days
    const logRetentionDays =
      typeof rawDays === 'number' && rawDays > 0 ? Math.min(365, Math.floor(rawDays)) : defaults.log_retention_days
    return {
      scm_mode: scmMode,
      local_source_path: localSourcePath,
      build_cmd: stored.build_cmd?.trim() ? stored.build_cmd : defaults.build_cmd,
      output_dir: stored.output_dir?.trim() ? stored.output_dir : defaults.output_dir,
      artifact_pattern: stored.artifact_pattern?.trim()
        ? stored.artifact_pattern
        : defaults.artifact_pattern,
      log_retention_days: logRetentionDays,
      post_build_actions: parsePostBuildActions(
        (stored as Record<string, unknown>).post_build_actions,
      ),
      build_triggers: parseBuildTriggers(
        (stored as Record<string, unknown>).build_triggers,
      ),
    }
  } catch {
    return { ...defaults, post_build_actions: [], build_triggers: structuredClone(DEFAULT_BUILD_TRIGGERS) }
  }
}
