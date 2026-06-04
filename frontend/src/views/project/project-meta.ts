import type { ComposerTranslation } from 'vue-i18n'
import type { EnvType, ProjectType } from '@/types'

export const FALLBACK_PROJECT_TYPES: ProjectType[] = [
  'maven',
  'npm',
  'vue',
  'go',
  'gradle',
  'custom',
]

export function normalizeProjectTypes(raw: unknown): ProjectType[] {
  if (!Array.isArray(raw)) return [...FALLBACK_PROJECT_TYPES]
  const out: ProjectType[] = []
  for (const v of raw) {
    if (typeof v === 'string' && FALLBACK_PROJECT_TYPES.includes(v as ProjectType)) {
      out.push(v as ProjectType)
    }
  }
  return out.length ? out : [...FALLBACK_PROJECT_TYPES]
}

export function projectTypeLabel(t: ComposerTranslation, type: unknown): string {
  const key = typeof type === 'string' ? type : ''
  if (!key) return t('projectType_unknown')
  const i18nKey = `projectType_${key}`
  const translated = t(i18nKey)
  return translated === i18nKey ? t('projectType_unknown') : translated
}

export function projectTypeTagClass(type: unknown): string {
  const tp = typeof type === 'string' ? type : 'custom'
  return `project-type-tag project-type-tag--${tp}`
}

/** Env types commonly linked per project type (for editor hints). */
export function suggestedEnvTypes(projectType: ProjectType): EnvType[] {
  switch (projectType) {
    case 'maven':
      return ['git', 'jdk', 'maven']
    case 'gradle':
      return ['git', 'jdk']
    case 'npm':
    case 'vue':
      return ['git', 'nodejs']
    case 'go':
      return ['git']
    default:
      return ['git']
  }
}

export const DEPLOY_ENV_TYPES: EnvType[] = ['ssh', 'sftp', 'ftp']
