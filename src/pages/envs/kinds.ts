export type EnvFieldType = 'text' | 'password' | 'number' | 'bool' | 'select'

export type EnvFieldSpec = {
  key: string
  type: EnvFieldType
  required?: boolean
  secret?: boolean
  default?: unknown
  options?: string[]
  group?: string
  when?: Record<string, string>
}

export type EnvKindSpec = {
  kind: string
  category: string
  fields: EnvFieldSpec[]
}

export type EnvRow = {
  rec_id: string
  workspace_rec_id?: string
  env_kind: string
  env_name: string
  config_hint?: string
  is_enabled?: number
  config?: Record<string, unknown>
  secrets_set?: string[]
}

/** 与后端 Catalog 对齐；GET /envs/kinds 失败时使用。 */
export const FALLBACK_KINDS: EnvKindSpec[] = [
  {
    kind: 'ssh',
    category: 'remote',
    fields: [
      { key: 'host', type: 'text', required: true },
      { key: 'port', type: 'number', default: 22 },
      { key: 'username', type: 'text' },
      { key: 'password', type: 'password', secret: true },
      { key: 'private_key_path', type: 'text' },
      { key: 'timeout_sec', type: 'number', default: 60 },
    ],
  },
  {
    kind: 'sftp',
    category: 'remote',
    fields: [
      { key: 'host', type: 'text', required: true },
      { key: 'port', type: 'number', default: 22 },
      { key: 'username', type: 'text' },
      { key: 'password', type: 'password', secret: true },
      { key: 'private_key_path', type: 'text' },
      { key: 'remote_path', type: 'text', default: '/' },
      { key: 'timeout_sec', type: 'number', default: 60 },
    ],
  },
  {
    kind: 'ftp',
    category: 'remote',
    fields: [
      { key: 'host', type: 'text', required: true },
      { key: 'port', type: 'number', default: 21 },
      { key: 'username', type: 'text' },
      { key: 'password', type: 'password', secret: true },
      { key: 'remote_dir', type: 'text', default: '/' },
      { key: 'passive', type: 'bool', default: true },
      { key: 'timeout_sec', type: 'number', default: 60 },
    ],
  },
  {
    kind: 'git',
    category: 'credential',
    fields: [
      { key: 'auth_type', type: 'select', default: 'https', options: ['none', 'https', 'ssh'], group: 'auth' },
      { key: 'auth_token', type: 'password', secret: true, group: 'auth', when: { auth_type: 'https' } },
      { key: 'auth_username', type: 'text', group: 'auth', when: { auth_type: 'https' } },
      { key: 'auth_password', type: 'password', secret: true, group: 'auth', when: { auth_type: 'https' } },
      { key: 'private_key_path', type: 'text', group: 'auth', when: { auth_type: 'ssh' } },
      { key: 'private_key_passphrase', type: 'password', secret: true, group: 'auth', when: { auth_type: 'ssh' } },
      { key: 'git_path', type: 'text', group: 'runtime' },
      { key: 'username', type: 'text', group: 'commit' },
      { key: 'email', type: 'text', group: 'commit' },
    ],
  },
  {
    kind: 'maven',
    category: 'toolchain',
    fields: [
      { key: 'maven_home', type: 'text' },
      { key: 'mvn_path', type: 'text' },
      { key: 'settings_xml', type: 'text' },
      { key: 'java_home', type: 'text' },
    ],
  },
  {
    kind: 'node',
    category: 'toolchain',
    fields: [
      { key: 'node_path', type: 'text' },
      { key: 'npm_path', type: 'text' },
      { key: 'npx_path', type: 'text' },
      { key: 'npmrc_path', type: 'text' },
      { key: 'registry', type: 'text' },
      { key: 'auth_token', type: 'password', secret: true },
      { key: 'auth_username', type: 'text' },
      { key: 'auth_password', type: 'password', secret: true },
      { key: 'pnpm_allow_all_builds', type: 'bool', default: true },
    ],
  },
  {
    kind: 'jdk',
    category: 'toolchain',
    fields: [
      { key: 'java_home', type: 'text' },
      { key: 'java_path', type: 'text' },
    ],
  },
  {
    kind: 'smtp',
    category: 'notification',
    fields: [
      { key: 'host', type: 'text', required: true },
      { key: 'port', type: 'number', default: 587 },
      { key: 'from', type: 'text' },
      { key: 'from_name', type: 'text', default: 'Auto Pack' },
      { key: 'to', type: 'text' },
      { key: 'username', type: 'text' },
      { key: 'password', type: 'password', secret: true },
      { key: 'tls', type: 'bool', default: false },
      { key: 'starttls', type: 'bool', default: true },
      { key: 'timeout_sec', type: 'number', default: 60 },
    ],
  },
]

export function canonicalKind(kind: string): string {
  const k = kind.trim().toLowerCase()
  if (k === 'nodejs' || k === 'node.js') return 'node'
  return k
}

export function findKind(kinds: EnvKindSpec[], kind: string): EnvKindSpec | undefined {
  const k = canonicalKind(kind)
  return kinds.find((item) => item.kind === k)
}

export function defaultsOf(spec: EnvKindSpec): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const field of spec.fields) {
    if (field.default !== undefined) out[field.key] = field.default
    else if (field.type === 'bool') out[field.key] = false
    else out[field.key] = ''
  }
  return out
}

export function fieldVisible(field: EnvFieldSpec, config: Record<string, unknown>): boolean {
  if (!field.when) return true
  return Object.entries(field.when).every(([key, value]) => String(config[key] ?? '') === value)
}

export function kindTagClass(kind: string, kinds: EnvKindSpec[]): string {
  const spec = findKind(kinds, kind)
  const cat = spec?.category || 'unknown'
  return `env-kind-tag env-kind-tag--${cat}`
}
