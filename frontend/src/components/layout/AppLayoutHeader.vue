<script setup lang="ts">
import { computed, onMounted, ref, watchEffect } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowDown, Monitor, Moon, Sunny, SwitchButton } from '@element-plus/icons-vue'
import type { AppLocale } from '@/i18n'
import { useSessionStore, useSettingsStore } from '@/stores'
import { fetchAppHealth } from '@/request/api/app'
import { getToken } from '@/request/utils'
import type { ThemeMode } from '@/types'
import AppBrandLogo from './AppBrandLogo.vue'
import { APP_NAV_ITEMS } from './nav-items'

const { t } = useI18n()
const route = useRoute()
const vueRouter = useRouter()
const sessionStore = useSessionStore()
const settingsStore = useSettingsStore()

const isDev = import.meta.env.DEV

const navItems = computed(() =>
  APP_NAV_ITEMS.filter((item) => !item.devOnly || isDev),
)

const themeOptions: { value: ThemeMode; icon: typeof Sunny; labelKey: string }[] = [
  { value: 'light', icon: Sunny, labelKey: 'themeLight' },
  { value: 'dark', icon: Moon, labelKey: 'themeDark' },
  { value: 'system', icon: Monitor, labelKey: 'themeSystem' },
]

const langOptions = [
  { value: 'zh-CN', label: '中文' },
  { value: 'en', label: 'English' },
] as const

const userInitial = computed(() => {
  const name = sessionStore.currentUser?.login_name?.trim()
  return name ? name.charAt(0).toUpperCase() : '?'
})

const prefersDark = ref(false)
const appVersion = ref('')

watchEffect((onCleanup) => {
  if (typeof window === 'undefined' || !window.matchMedia) return
  const mq = window.matchMedia('(prefers-color-scheme: dark)')
  const sync = (): void => {
    prefersDark.value = mq.matches
  }
  sync()
  mq.addEventListener('change', sync)
  onCleanup(() => mq.removeEventListener('change', sync))
})

const accountPanelDark = computed(() => {
  const mode = settingsStore.themeMode
  if (mode === 'dark') return true
  if (mode === 'light') return false
  return prefersDark.value
})

/** Opaque popper — no EP overlay / blur bleed-through */
const accountPopperStyle = computed(() => {
  if (accountPanelDark.value) {
    return {
      padding: '0',
      background: '#1e293b',
      border: '1px solid #334155',
      borderRadius: '12px',
      boxShadow: '0 12px 32px rgba(0, 0, 0, 0.45)',
      backdropFilter: 'none',
    } as const
  }
  return {
    padding: '0',
    background: '#ffffff',
    border: '1px solid #e2e8f0',
    borderRadius: '12px',
    boxShadow: '0 12px 32px rgba(15, 23, 42, 0.16)',
    backdropFilter: 'none',
  } as const
})

function isNavActive(item: (typeof APP_NAV_ITEMS)[number]): boolean {
  return route.name === item.routeName || route.path === item.path || route.path.startsWith(`${item.path}/`)
}

function switchLang(next: AppLocale): void {
  settingsStore.setLocale(next)
}

function onLogout(): void {
  sessionStore.logout()
  vueRouter.replace({ name: 'home' })
}

onMounted(() => {
  void fetchAppHealth()
    .then((res) => {
      appVersion.value = res.version?.trim() ?? ''
    })
    .catch(() => {
      appVersion.value = ''
    })
  if (sessionStore.currentUser || !getToken()) return
  void sessionStore.loadSession()
})
</script>

<template>
  <header class="app-shell-header">
    <div class="app-shell-header__inner">
      <RouterLink class="shell-brand shell-brand--link" :to="{ name: 'projects' }">
        <AppBrandLogo />
        <span class="shell-brand__text">{{ t('appTitle') }}</span>
        <span v-if="appVersion" class="shell-brand__version">v{{ appVersion }}</span>
      </RouterLink>

      <nav class="app-shell-nav" :aria-label="t('appNavAria')">
        <RouterLink
          v-for="item in navItems"
          :key="item.routeName"
          :to="item.path"
          class="app-shell-nav__item"
          :class="{ 'is-active': isNavActive(item) }"
        >
          {{ t(item.labelKey) }}
        </RouterLink>
      </nav>

      <div class="app-shell-header__actions">
        <el-popover
          teleported
          effect="plain"
          trigger="click"
          placement="bottom-end"
          :width="280"
          :show-arrow="false"
          :offset="8"
          :popper-style="accountPopperStyle"
          popper-class="app-shell-popover"
        >
          <template #reference>
            <button type="button" class="app-shell-user" :aria-label="t('accountMenuAria')">
              <span class="app-shell-user__avatar">{{ userInitial }}</span>
              <span class="app-shell-user__name">
                {{
                  sessionStore.loading ? t('userLoading') : sessionStore.currentUser?.login_name ?? t('guestUser')
                }}
              </span>
              <el-icon class="app-shell-user__chevron" :size="12">
                <ArrowDown />
              </el-icon>
            </button>
          </template>

          <div class="app-shell-panel">
              <div class="app-shell-panel__profile">
                <div class="app-shell-panel__avatar" aria-hidden="true">{{ userInitial }}</div>
                <div class="app-shell-panel__meta">
                  <div class="app-shell-panel__name">
                    {{ sessionStore.currentUser?.login_name ?? '—' }}
                  </div>
                  <div class="app-shell-panel__role">{{ t('loggedInUser') }}</div>
                </div>
              </div>

              <div class="app-shell-panel__section">
                <div class="app-shell-panel__label">{{ t('switchTheme') }}</div>
                <div class="app-shell-panel__options">
                  <button
                    v-for="opt in themeOptions"
                    :key="opt.value"
                    type="button"
                    class="app-shell-panel__option"
                    :class="{ 'is-active': settingsStore.themeMode === opt.value }"
                    @click="settingsStore.setTheme(opt.value)"
                  >
                    <el-icon :size="14">
                      <component :is="opt.icon" />
                    </el-icon>
                    <span>{{ t(opt.labelKey) }}</span>
                  </button>
                </div>
              </div>

              <div class="app-shell-panel__section">
                <div class="app-shell-panel__label">{{ t('switchLanguage') }}</div>
                <div class="app-shell-panel__options">
                  <button
                    v-for="opt in langOptions"
                    :key="opt.value"
                    type="button"
                    class="app-shell-panel__option"
                    :class="{ 'is-active': settingsStore.locale === opt.value }"
                    @click="switchLang(opt.value)"
                  >
                    {{ opt.label }}
                  </button>
                </div>
              </div>

              <div class="app-shell-panel__divider" />

              <button type="button" class="app-shell-panel__logout" @click="onLogout">
                <el-icon :size="16">
                  <SwitchButton />
                </el-icon>
                {{ t('logout') }}
              </button>
          </div>
        </el-popover>
      </div>
    </div>
  </header>
</template>
