import type {
  PostBuildAction,
  PostBuildActionType,
  PostBuildTransferSet,
  PostBuildWhen,
} from '@/types/post-build'
import { randomId } from '@/utils/random-id'
import { isRegisteredPostBuildType } from './post-build-registry'

export function newPostBuildActionId(): string {
  return randomId()
}

export function defaultTransferSet(): PostBuildTransferSet {
  return {
    source_files: '**/*',
    remove_prefix: '',
    remote_dir: '/',
    exec_command: '',
  }
}

export function defaultPostBuildAction(type: PostBuildActionType = 'ssh_transfer'): PostBuildAction {
  const base: PostBuildAction = {
    id: newPostBuildActionId(),
    name: '',
    type,
    enabled: true,
    when: 'success',
    env_rec_id: '',
  }
  switch (type) {
    case 'shell_exec':
    case 'ssh_exec':
      return { ...base, command: '' }
    case 'ssh_transfer':
      return { ...base, transfer_sets: [defaultTransferSet()] }
    case 'email_notify':
      return {
        ...base,
        when: 'failure',
        recipients: '',
        subject: '',
        body: '',
      }
    default:
      return base
  }
}

/** Reset type-specific fields when the user changes action type in the editor. */
export function applyPostBuildTypeChange(action: PostBuildAction, type: PostBuildActionType): void {
  const fresh = defaultPostBuildAction(type)
  action.type = type
  action.command = fresh.command
  action.transfer_sets = fresh.transfer_sets
  action.recipients = fresh.recipients
  action.subject = fresh.subject
  action.body = fresh.body
  action.when = fresh.when
  if (type === 'ssh_exec' && !action.env_rec_id) {
    action.env_rec_id = ''
  }
}

function isWhen(v: unknown): v is PostBuildWhen {
  return v === 'success' || v === 'failure' || v === 'always'
}

function normalizeTransferSet(raw: unknown): PostBuildTransferSet {
  const o = raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {}
  return {
    source_files: String(o.source_files ?? '**/*'),
    remove_prefix: String(o.remove_prefix ?? ''),
    remote_dir: String(o.remote_dir ?? '/'),
    exec_command: String(o.exec_command ?? ''),
  }
}

function normalizeAction(raw: unknown): PostBuildAction | null {
  if (!raw || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const type = o.type
  if (!isRegisteredPostBuildType(type)) return null
  const id = String(o.id ?? '').trim() || newPostBuildActionId()
  const action: PostBuildAction = {
    id,
    name: String(o.name ?? ''),
    type,
    enabled: o.enabled !== false,
    when: isWhen(o.when) ? o.when : 'success',
    env_rec_id: String(o.env_rec_id ?? ''),
  }
  if (type === 'shell_exec' || type === 'ssh_exec') {
    action.command = String(o.command ?? '')
  }
  if (type === 'ssh_transfer') {
    const sets = Array.isArray(o.transfer_sets) ? o.transfer_sets : []
    action.transfer_sets =
      sets.length > 0 ? sets.map(normalizeTransferSet) : [defaultTransferSet()]
  }
  if (type === 'email_notify') {
    action.recipients = String(o.recipients ?? '')
    action.subject = String(o.subject ?? '')
    action.body = String(o.body ?? '')
  }
  return action
}

/** Parse post_build_actions from config_json object. */
export function parsePostBuildActions(raw: unknown): PostBuildAction[] {
  if (!Array.isArray(raw)) return []
  return raw.map(normalizeAction).filter((a): a is PostBuildAction => a != null)
}

/** Legacy single deploy_env_rec_id → one transfer action. */
export function migrateDeployEnvToActions(deployEnvRecId: string): PostBuildAction[] {
  const id = deployEnvRecId.trim()
  if (!id) return []
  return [
    {
      ...defaultPostBuildAction('ssh_transfer'),
      name: '',
      env_rec_id: id,
      transfer_sets: [
        {
          source_files: '**/*',
          remove_prefix: 'target',
          remote_dir: '/',
          exec_command: '',
        },
      ],
    },
  ]
}

/** First enabled transfer action env for backward-compatible deploy_env_rec_id. */
export function primaryDeployEnvFromActions(actions: PostBuildAction[]): string {
  const hit = actions.find((a) => a.enabled && a.type === 'ssh_transfer' && a.env_rec_id.trim())
  return hit?.env_rec_id.trim() ?? ''
}
