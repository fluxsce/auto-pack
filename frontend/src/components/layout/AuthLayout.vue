<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Monitor, Moon, Sunny } from '@element-plus/icons-vue'
import type { AppLocale } from '@/i18n'
import AppBrandLogo from '@/components/layout/AppBrandLogo.vue'
import TechBackground from '@/components/layout/TechBackground.vue'
import { useSettingsStore } from '@/stores'
import type { ThemeMode } from '@/types'

const { t } = useI18n()
const settingsStore = useSettingsStore()

const themeOptions: { value: ThemeMode; icon: typeof Sunny; labelKey: string }[] = [
  { value: 'light', icon: Sunny, labelKey: 'themeLight' },
  { value: 'dark', icon: Moon, labelKey: 'themeDark' },
  { value: 'system', icon: Monitor, labelKey: 'themeSystem' },
]

const langOptions = [
  { value: 'zh-CN' as AppLocale, label: '中文' },
  { value: 'en' as AppLocale, label: 'EN' },
] as const
</script>

<template>
  <div class="auth-layout tech-shell">
    <TechBackground />

    <header class="auth-layout__toolbar">
      <div class="shell-brand">
        <AppBrandLogo />
        <span class="shell-brand__text">{{ t('appTitle') }}</span>
      </div>
      <div class="shell-prefs" role="toolbar" :aria-label="t('loginPrefsAria')">
        <div class="shell-prefs__group" role="group" :aria-label="t('switchTheme')">
          <button
            v-for="opt in themeOptions"
            :key="opt.value"
            type="button"
            class="shell-prefs__btn"
            :class="{ 'is-active': settingsStore.themeMode === opt.value }"
            :title="t(opt.labelKey)"
            :aria-label="t(opt.labelKey)"
            :aria-pressed="settingsStore.themeMode === opt.value"
            @click="settingsStore.setTheme(opt.value)"
          >
            <el-icon :size="16">
              <component :is="opt.icon" />
            </el-icon>
          </button>
        </div>
        <span class="shell-prefs__divider" aria-hidden="true" />
        <div class="shell-prefs__group" role="group" :aria-label="t('switchLanguage')">
          <button
            v-for="opt in langOptions"
            :key="opt.value"
            type="button"
            class="shell-prefs__btn shell-prefs__btn--text"
            :class="{ 'is-active': settingsStore.locale === opt.value }"
            :aria-pressed="settingsStore.locale === opt.value"
            @click="settingsStore.setLocale(opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>
    </header>

    <main class="auth-layout__main">
      <router-view />
    </main>

    <footer class="auth-layout__footer">
      <span>{{ t('loginFooter') }}</span>
    </footer>
  </div>
</template>

<style scoped lang="scss">
.auth-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  overflow: hidden;

  &__toolbar {
    position: relative;
    z-index: 2;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 20px clamp(20px, 4vw, 48px);
  }

  &__main {
    position: relative;
    z-index: 1;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 0;
    padding: 0 clamp(20px, 4vw, 48px);
    overflow-y: auto;
  }

  &__footer {
    position: relative;
    z-index: 2;
    flex-shrink: 0;
    padding: 16px clamp(20px, 4vw, 48px) 28px;
    text-align: center;
    font-size: 12px;
    color: var(--shell-text-muted);
  }
}
</style>
