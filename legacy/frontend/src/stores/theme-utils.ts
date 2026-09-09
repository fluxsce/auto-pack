import { LOCALE_KEY, THEME_KEY } from '@/constants'
import type { AppLocale } from '@/i18n'
import type { ThemeMode } from '@/types'

export function readStoredTheme(): ThemeMode {
  const stored = localStorage.getItem(THEME_KEY)
  if (stored === 'light' || stored === 'dark' || stored === 'system') {
    return stored
  }
  return 'system'
}

export function readStoredLocale(): AppLocale {
  const stored = localStorage.getItem(LOCALE_KEY)
  if (stored === 'zh' || stored === 'zh-CN') return 'zh-CN'
  if (stored === 'en') return stored
  return 'zh-CN'
}

function systemPrefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function resolveDark(mode: ThemeMode): boolean {
  if (mode === 'dark') return true
  if (mode === 'light') return false
  return systemPrefersDark()
}

export function applyThemeToDom(mode: ThemeMode): void {
  const root = document.documentElement
  const dark = resolveDark(mode)
  root.classList.toggle('dark', dark)
  root.dataset.theme = dark ? 'dark' : 'light'
}

export function bindSystemThemeListener(onSystemChange: () => void): void {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', onSystemChange)
}
