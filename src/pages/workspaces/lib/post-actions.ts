export const postActionKinds = ['ftp', 'sftp', 'ssh', 'email', 'shell'] as const

export type PostActionKind = (typeof postActionKinds)[number]

export const postWhenKinds = ['success', 'failure', 'always'] as const

export type PostActionWhen = (typeof postWhenKinds)[number]

export type PostAction = {
  id: string
  kind: PostActionKind
  enabled?: boolean
  env_rec_id?: string
  remote_path?: string
  command?: string
  when?: PostActionWhen
  to?: string
  subject?: string
  attach_log?: boolean
}

export function newPostAction(kind: PostActionKind): PostAction {
  const id = globalThis.crypto?.randomUUID?.() ?? `act-${Date.now().toString(16)}`
  return {
    id,
    kind,
    enabled: true,
    env_rec_id: '',
    remote_path: '',
    command: '',
    when: 'success',
    to: '',
    subject: '',
    attach_log: false,
  }
}

export function envKindForAction(kind: PostActionKind): string {
  if (kind === 'email') return 'smtp'
  if (kind === 'shell') return ''
  return kind
}

export function isPostActionKind(value: string): value is PostActionKind {
  return (postActionKinds as readonly string[]).includes(value)
}

export function isPostWhen(value: string): value is PostActionWhen {
  return (postWhenKinds as readonly string[]).includes(value)
}

export function normalizeWhen(value?: string): PostActionWhen {
  if (value === 'failure' || value === 'failed' || value === 'fail') return 'failure'
  if (value === 'always' || value === 'all') return 'always'
  return 'success'
}
