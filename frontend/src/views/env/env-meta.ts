import type { ComposerTranslation } from 'vue-i18n'
import type { EnvType } from '@/types'
import { FALLBACK_ENV_TYPES } from './env-defaults'

export type EnvCategoryKey = 'remote' | 'credential' | 'toolchain' | 'notification'

export interface EnvCategory {
  key: EnvCategoryKey
  labelKey: string
  types: EnvType[]
}

/** Grouped for UI sections (remote access vs local toolchain). */
export const ENV_CATEGORIES: EnvCategory[] = [
  {
    key: 'remote',
    labelKey: 'envCategoryRemote',
    types: ['ssh', 'sftp', 'ftp'],
  },
  {
    key: 'notification',
    labelKey: 'envCategoryNotification',
    types: ['smtp'],
  },
  {
    key: 'credential',
    labelKey: 'envCategoryCredential',
    types: ['git'],
  },
  {
    key: 'toolchain',
    labelKey: 'envCategoryToolchain',
    types: ['maven', 'nodejs', 'jdk'],
  },
]

/** All types support server-side probe today. */
export const VERIFIABLE_ENV_TYPES: EnvType[] = [
  'ssh',
  'sftp',
  'ftp',
  'git',
  'maven',
  'nodejs',
  'jdk',
  'smtp',
]

export function supportsEnvVerify(type: EnvType): boolean {
  return VERIFIABLE_ENV_TYPES.includes(type)
}

export function isRemoteEnvType(type: EnvType): boolean {
  return type === 'ssh' || type === 'sftp' || type === 'ftp'
}

export function envCategoryOf(type: EnvType): EnvCategoryKey {
  if (isRemoteEnvType(type)) return 'remote'
  if (type === 'git') return 'credential'
  if (type === 'smtp') return 'notification'
  return 'toolchain'
}

export function isEnvType(value: unknown): value is EnvType {
  return typeof value === 'string' && VERIFIABLE_ENV_TYPES.includes(value as EnvType)
}

export function normalizeEnvTypes(types: unknown): EnvType[] {
  if (!Array.isArray(types)) return [...FALLBACK_ENV_TYPES]
  const out = types.filter(isEnvType)
  return out.length > 0 ? out : [...FALLBACK_ENV_TYPES]
}

export function envTypeLabel(t: ComposerTranslation, type: unknown): string {
  if (!isEnvType(type)) return t('envType_unknown')
  return t(`envType_${type}`)
}

/** CSS modifier for env type pills (remote vs toolchain). */
export function envTypeTagClass(type: unknown): string {
  if (!isEnvType(type)) return 'env-type-tag env-type-tag--unknown'
  return `env-type-tag env-type-tag--${envCategoryOf(type)}`
}
