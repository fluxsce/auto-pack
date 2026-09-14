/** 工程作业上下文变量分组（与 procenv jobContextEnv 对齐）。 */
export type ProjectEnvVarGroup = 'job' | 'workspace' | 'project' | 'scm' | 'release'

export type ProjectEnvVarDef = {
  key: string
  group: ProjectEnvVarGroup
}

/** 构建/后置脚本与 local_source_path 可用的 AP_* 变量清单。 */
export const PROJECT_ENV_VAR_GROUPS: ProjectEnvVarGroup[] = [
  'job',
  'workspace',
  'project',
  'scm',
  'release',
]

export const PROJECT_ENV_VARS: ProjectEnvVarDef[] = [
  { key: 'AP_JOB_ID', group: 'job' },
  { key: 'AP_JOB_TYPE', group: 'job' },
  { key: 'AP_WORKSPACE_ID', group: 'workspace' },
  { key: 'AP_WORKSPACE_CODE', group: 'workspace' },
  { key: 'AP_WORKSPACE_NAME', group: 'workspace' },
  { key: 'AP_WORKSPACE_PATH', group: 'workspace' },
  { key: 'AP_PROJECT_ID', group: 'project' },
  { key: 'AP_PROJECT_NAME', group: 'project' },
  { key: 'AP_PROJECT_TYPE', group: 'project' },
  { key: 'AP_PROJECT_PATH', group: 'project' },
  { key: 'AP_PROJECT_WORKDIR', group: 'project' },
  { key: 'AP_SCM_MODE', group: 'scm' },
  { key: 'AP_LOCAL_SOURCE_PATH', group: 'scm' },
  { key: 'AP_REPO_BRANCH', group: 'scm' },
  { key: 'AP_REPO_URL', group: 'scm' },
  { key: 'AP_TAG', group: 'release' },
]

export const PROJECT_ENV_VAR_EXAMPLES = [
  'dest="/home/appinstall/${AP_WORKSPACE_CODE}/${AP_PROJECT_NAME}"',
  'local_source_path: /data/src/${AP_WORKSPACE_CODE}/${AP_PROJECT_NAME}',
  'mvn -B package -Dapp.name=${AP_PROJECT_NAME}',
] as const

export function projectEnvVarsByGroup(group: ProjectEnvVarGroup): ProjectEnvVarDef[] {
  return PROJECT_ENV_VARS.filter((item) => item.group === group)
}
