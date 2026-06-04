import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { LOCALE_KEY, THEME_KEY } from '@/constants'
import { i18n, type AppLocale } from '@/i18n'
import type { ThemeMode } from '@/types'
import {
  applyThemeToDom,
  bindSystemThemeListener,
  readStoredLocale,
  readStoredTheme,
  resolveDark,
} from './theme-utils'

export const useSettingsStore = defineStore('settings', () => {
  const themeMode = ref<ThemeMode>(readStoredTheme())
  const locale = ref<AppLocale>(readStoredLocale())

  const isDark = computed(() => resolveDark(themeMode.value))

  let initialized = false

  function init(): void {
    if (initialized) return
    initialized = true

    applyThemeToDom(themeMode.value)
    i18n.global.locale.value = locale.value

    watch(themeMode, (mode) => applyThemeToDom(mode))

    bindSystemThemeListener(() => {
      if (themeMode.value === 'system') {
        applyThemeToDom('system')
      }
    })
  }

  function setTheme(mode: ThemeMode): void {
    themeMode.value = mode
    localStorage.setItem(THEME_KEY, mode)
    applyThemeToDom(mode)
  }

  function toggleTheme(): void {
    setTheme(isDark.value ? 'light' : 'dark')
  }

  function setLocale(next: AppLocale): void {
    locale.value = next
    localStorage.setItem(LOCALE_KEY, next)
    i18n.global.locale.value = next
  }

  return {
    themeMode,
    locale,
    isDark,
    init,
    setTheme,
    toggleTheme,
    setLocale,
  }
})
