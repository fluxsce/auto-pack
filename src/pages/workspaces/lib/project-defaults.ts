import { isPostActionKind, newPostAction, normalizeWhen, type PostAction } from './post-actions'

export const projectTypes = [
  { value: 'maven', label: 'Maven' },
  { value: 'npm', label: 'npm' },
  { value: 'vue', label: 'Vue' },
  { value: 'go', label: 'Go' },
  { value: 'custom', label: 'Custom' },
] as const

export type ProjectRecipe = {
  command: string
  workdir?: string
  artifacts?: string[]
  auto_sync?: boolean
  scm_mode?: 'local' | 'git'
  local_source_path?: string
  output_dir?: string
  artifact_pattern?: string
  remote_path?: string
  log_retention_days?: number
  git_env_rec_id?: string
  jdk_env_rec_id?: string
  maven_env_rec_id?: string
  node_env_rec_id?: string
  ftp_env_rec_id?: string
  depends_on?: string[]
  post_actions?: PostAction[]
}

const buildDefaults: Record<string, Pick<ProjectRecipe, 'command' | 'output_dir' | 'artifact_pattern'>> = {
  maven: { command: 'mvn -B -DskipTests package', output_dir: 'target', artifact_pattern: '*.jar' },
  npm: { command: 'npm run build', output_dir: 'dist', artifact_pattern: '*' },
  vue: { command: 'npm run build', output_dir: 'dist', artifact_pattern: '*' },
  go: { command: 'go build -o dist/ ./...', output_dir: 'dist', artifact_pattern: '*' },
  custom: { command: '', output_dir: '', artifact_pattern: '' },
}

export function defaultBuild(type: string): Pick<ProjectRecipe, 'command' | 'output_dir' | 'artifact_pattern'> {
  return { ...(buildDefaults[type] ?? buildDefaults.custom) }
}

export function defaultCommand(type: string): string {
  return defaultBuild(type).command ?? ''
}

export function envKindsForType(type: string): string[] {
  if (type === 'maven') return ['jdk', 'maven']
  if (type === 'npm' || type === 'vue') return ['node']
  return ['jdk', 'maven', 'node']
}

export function emptyRecipe(type = 'maven'): ProjectRecipe {
  return {
    scm_mode: 'git',
    auto_sync: true,
    local_source_path: '',
    remote_path: '',
    workdir: '',
    log_retention_days: 3,
    git_env_rec_id: '',
    jdk_env_rec_id: '',
    maven_env_rec_id: '',
    node_env_rec_id: '',
    ftp_env_rec_id: '',
    depends_on: [],
    post_actions: [],
    ...defaultBuild(type),
  }
}

/** 本机绝对目录（POSIX /... 或 Windows X:/...）。 */
export function isAbsLocalPath(value: string): boolean {
  const path = value.trim().replaceAll('\\', '/')
  if (!path || path.includes('..')) return false
  if (path.startsWith('/')) return true
  return /^[A-Za-z]:\//.test(path)
}

/** 与后端 WorkRel 对齐：本地相对子路径才叠到工程名下；绝对路径不拼进空间根。 */
export function workRel(name: string, recipe: ProjectRecipe): string {
  const raw = (recipe.local_source_path || '').trim().replaceAll('\\', '/')
  if (recipe.scm_mode === 'local' && raw && !isAbsLocalPath(raw)) {
    const extra = raw.split('/').filter(Boolean).join('/')
    return name ? `${name}/${extra}` : extra
  }
  return name
}

/** 界面/作业看到的源码目录：本地绝对路径原样用，否则空间根 + workRel。 */
export function checkoutDisplay(root: string, name: string, recipe: ProjectRecipe, storedRel = ''): string {
  const raw = (recipe.local_source_path || '').trim()
  if (recipe.scm_mode === 'local' && isAbsLocalPath(raw)) {
    return raw.replaceAll('\\', '/')
  }
  const rel = storedRel.trim() || workRel(name, recipe)
  const base = root.replace(/[/\\]+$/, '')
  return base && rel ? `${base}/${rel}` : rel
}

function migrateActions(stored: Partial<ProjectRecipe>): PostAction[] {
  if (stored.post_actions?.length) {
    return stored.post_actions.filter((item) => isPostActionKind(item.kind)).map((item) => ({
      ...newPostAction(item.kind),
      ...item,
      id: item.id || newPostAction(item.kind).id,
      when: normalizeWhen(item.when),
      attach_log: item.attach_log === true,
    }))
  }
  if (stored.ftp_env_rec_id) {
    return [
      {
        ...newPostAction('ftp'),
        env_rec_id: stored.ftp_env_rec_id,
        remote_path: stored.remote_path || '',
      },
    ]
  }
  return []
}

export function parseRecipe(raw: string, type = 'maven'): ProjectRecipe {
  const fallback = emptyRecipe(type)
  try {
    const stored = JSON.parse(raw || '{}') as Partial<ProjectRecipe> & { build_cmd?: string }
    const command = (stored.command || stored.build_cmd || '').trim() || fallback.command
    return {
      ...fallback,
      ...stored,
      command,
      scm_mode: stored.scm_mode === 'local' ? 'local' : 'git',
      auto_sync: stored.auto_sync !== false,
      artifacts: stored.artifacts,
      depends_on: stored.depends_on ?? [],
      post_actions: migrateActions(stored),
      log_retention_days: Number(stored.log_retention_days ?? fallback.log_retention_days) || 0,
    }
  } catch {
    return fallback
  }
}

export function serializeRecipe(recipe: ProjectRecipe): string {
  const actions = recipe.post_actions ?? []
  const firstUpload = actions.find((item) => ['ftp', 'sftp', 'ssh'].includes(item.kind) && item.env_rec_id)
  const artifacts = recipe.artifact_pattern?.trim() ? [recipe.artifact_pattern.trim()] : recipe.artifacts
  return JSON.stringify({
    command: recipe.command,
    workdir: recipe.workdir || undefined,
    artifacts,
    auto_sync: recipe.auto_sync !== false,
    scm_mode: recipe.scm_mode,
    local_source_path: recipe.local_source_path || undefined,
    output_dir: recipe.output_dir || undefined,
    artifact_pattern: recipe.artifact_pattern || undefined,
    remote_path: firstUpload?.remote_path || recipe.remote_path || undefined,
    log_retention_days: recipe.log_retention_days || undefined,
    git_env_rec_id: recipe.git_env_rec_id || undefined,
    jdk_env_rec_id: recipe.jdk_env_rec_id || undefined,
    maven_env_rec_id: recipe.maven_env_rec_id || undefined,
    node_env_rec_id: recipe.node_env_rec_id || undefined,
    ftp_env_rec_id: firstUpload?.env_rec_id || recipe.ftp_env_rec_id || undefined,
    depends_on: (recipe.depends_on ?? []).filter(Boolean),
    post_actions: actions,
  })
}
