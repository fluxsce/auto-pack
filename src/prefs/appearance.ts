import { ref } from 'vue'
import { applyTheme as applyRsTheme, type RsLocale, type RsThemeMode } from '@niuma/ui'

export type ThemeMode = 'light' | 'dark' | 'system'
export type AppLocale = RsLocale

const THEME_KEY = 'ap_theme'
const LOCALE_KEY = 'ap_locale'

function storageGet(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function storageSet(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    /* ignore */
  }
}

function systemPrefersDark(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches
}

export function resolveTheme(mode: ThemeMode): RsThemeMode {
  if (mode === 'light' || mode === 'dark') {
    return mode
  }
  return systemPrefersDark() ? 'dark' : 'light'
}

function readThemeMode(): ThemeMode {
  const saved = storageGet(THEME_KEY)
  if (saved === 'light' || saved === 'dark' || saved === 'system') {
    return saved
  }
  return 'system'
}

function readLocale(): AppLocale {
  const saved = storageGet(LOCALE_KEY)
  if (saved === 'zh-CN' || saved === 'en-US') {
    return saved
  }
  if (saved === 'en' || saved === 'zh') {
    return saved === 'en' ? 'en-US' : 'zh-CN'
  }
  if (typeof navigator !== 'undefined' && navigator.language.toLowerCase().startsWith('en')) {
    return 'en-US'
  }
  return 'zh-CN'
}

function applyDocumentLocale(next: AppLocale): void {
  if (typeof document === 'undefined') {
    return
  }
  document.documentElement.lang = next
}

/** 立刻写到 html，登录页 / 工作台 / ensis-ui 共用同一套 data-rs-theme。 */
export function applyThemeToDom(mode: ThemeMode = themeMode.value): RsThemeMode {
  const resolved = resolveTheme(mode)
  if (typeof document === 'undefined') {
    return resolved
  }
  const root = document.documentElement
  root.setAttribute('data-rs-theme', resolved)
  root.classList.toggle('dark', resolved === 'dark')
  root.style.colorScheme = resolved
  applyRsTheme(resolved, root)
  return resolved
}

export const themeMode = ref<ThemeMode>(readThemeMode())
export const theme = ref<RsThemeMode>(resolveTheme(themeMode.value))
export const locale = ref<AppLocale>(readLocale())

applyDocumentLocale(locale.value)
applyThemeToDom(themeMode.value)

export function setTheme(mode: ThemeMode): void {
  themeMode.value = mode
  theme.value = applyThemeToDom(mode)
  storageSet(THEME_KEY, mode)
}

export function setLocale(next: AppLocale): void {
  locale.value = next
  storageSet(LOCALE_KEY, next)
  applyDocumentLocale(next)
}

if (typeof window !== 'undefined') {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (themeMode.value === 'system') {
      theme.value = applyThemeToDom('system')
    }
  })
}

export const localeOptions = [
  { value: 'zh-CN', label: '中文' },
  { value: 'en-US', label: 'EN' },
] as const

export const themeOptions = [
  { value: 'light', icon: 'sun' },
  { value: 'dark', icon: 'moon' },
  { value: 'system', icon: 'monitor' },
] as const
