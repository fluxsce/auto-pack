<script setup lang="ts">
import { RsIcon } from '@/ui'
import { useI18n } from 'vue-i18n'
import { locale, localeOptions, setLocale, setTheme, themeMode, themeOptions } from './appearance'

const { t } = useI18n()

function themeKey(mode: 'light' | 'dark' | 'system'): string {
  if (mode === 'light') return 'prefs.themeLight'
  if (mode === 'dark') return 'prefs.themeDark'
  return 'prefs.themeSystem'
}
</script>

<template>
  <div class="prefs" role="toolbar" :aria-label="t('prefs.aria')">
    <div class="prefs__group" role="group" :aria-label="t('prefs.theme')">
      <button
        v-for="opt in themeOptions"
        :key="opt.value"
        type="button"
        class="prefs__btn"
        :class="{ 'is-active': themeMode === opt.value }"
        :title="t(themeKey(opt.value))"
        :aria-label="t(themeKey(opt.value))"
        :aria-pressed="themeMode === opt.value"
        @click="setTheme(opt.value)"
      >
        <RsIcon :name="opt.icon" />
      </button>
    </div>
    <span class="prefs__divider" aria-hidden="true" />
    <div class="prefs__group" role="group" :aria-label="t('prefs.locale')">
      <button
        v-for="opt in localeOptions"
        :key="opt.value"
        type="button"
        class="prefs__btn prefs__btn--text"
        :class="{ 'is-active': locale === opt.value }"
        :aria-pressed="locale === opt.value"
        @click="setLocale(opt.value)"
      >
        {{ opt.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.prefs {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px;
  border-radius: 999px;
  background: var(--prefs-bg, rgba(255, 255, 255, 0.12));
  border: 1px solid var(--prefs-border, rgba(255, 255, 255, 0.2));
  backdrop-filter: blur(12px);
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);
}

.prefs__group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

.prefs__divider {
  width: 1px;
  height: 20px;
  margin: 0 4px;
  background: var(--prefs-border, rgba(255, 255, 255, 0.2));
  flex-shrink: 0;
}

.prefs__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 36px;
  height: 32px;
  padding: 0 10px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--prefs-text, rgba(248, 250, 252, 0.9));
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.prefs__btn:hover:not(.is-active) {
  background: rgba(255, 255, 255, 0.1);
}

.prefs__btn.is-active {
  background: var(--prefs-active-bg, rgba(255, 255, 255, 0.95));
  color: var(--prefs-active-fg, #0f172a);
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.12);
}

.prefs__btn--text {
  min-width: 40px;
}
</style>
