import type { ComposerTranslation } from 'vue-i18n'

export type GitAuthType = 'none' | 'https' | 'ssh'

export function parseGitAuthType(configJson: string | undefined): GitAuthType {
  try {
    const raw = JSON.parse(configJson || '{}') as { auth_type?: string }
    const t = String(raw.auth_type || 'none').toLowerCase()
    if (t === 'https' || t === 'ssh') return t
    return 'none'
  } catch {
    return 'none'
  }
}

export function gitAuthTypeLabel(t: ComposerTranslation, authType: GitAuthType): string {
  return t(`envGitAuthType_${authType}`)
}

export function gitEnvOptionLabel(t: ComposerTranslation, envName: string, configJson: string): string {
  const mode = parseGitAuthType(configJson)
  if (mode === 'none') return envName
  return `${envName} (${gitAuthTypeLabel(t, mode)})`
}
