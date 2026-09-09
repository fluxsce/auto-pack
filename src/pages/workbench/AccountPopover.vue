<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { RsAvatar, RsButton, RsIcon, RsPopover } from '@/ui'
import { useAuthStore } from '@/stores/auth'
import {
  locale,
  localeOptions,
  setLocale,
  setTheme,
  themeMode,
  themeOptions,
} from '@/prefs/appearance'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const open = ref(false)

const displayName = computed(() => auth.me?.display_name || auth.me?.login_name || '')
const loginName = computed(() => auth.me?.login_name || '')
const showLogin = computed(() => Boolean(loginName.value && loginName.value !== displayName.value))

function themeKey(mode: 'light' | 'dark' | 'system'): string {
  if (mode === 'light') return 'prefs.themeLight'
  if (mode === 'dark') return 'prefs.themeDark'
  return 'prefs.themeSystem'
}

function logout(): void {
  open.value = false
  auth.expire()
  void router.replace({ name: 'login' })
}
</script>

<template>
  <RsPopover v-model:open="open" side="bottom" align="end" width="md">
    <button
      type="button"
      class="acct__who"
      :class="{ 'is-open': open }"
      :aria-label="t('nav.accountMenu')"
      :aria-expanded="open"
      :aria-haspopup="true"
    >
      <RsAvatar size="sm" :name="displayName" />
      <span class="acct__who-name">{{ displayName }}</span>
      <RsIcon name="chevron-down" class="acct__chevron" />
    </button>
    <template #content>
      <div class="acct">
        <div class="acct__profile">
          <RsAvatar size="md" :name="displayName" />
          <div class="acct__meta">
            <div class="acct__name">{{ displayName || '—' }}</div>
            <div class="acct__id">{{ showLogin ? loginName : t('nav.signedIn') }}</div>
          </div>
        </div>

        <div class="acct__section">
          <div class="acct__label">{{ t('prefs.theme') }}</div>
          <div class="acct__options">
            <button
              v-for="opt in themeOptions"
              :key="opt.value"
              type="button"
              class="acct__option"
              :class="{ 'is-active': themeMode === opt.value }"
              @click="setTheme(opt.value)"
            >
              <RsIcon :name="opt.icon" />
              <span>{{ t(themeKey(opt.value)) }}</span>
            </button>
          </div>
        </div>

        <div class="acct__section">
          <div class="acct__label">{{ t('prefs.locale') }}</div>
          <div class="acct__options">
            <button
              v-for="opt in localeOptions"
              :key="opt.value"
              type="button"
              class="acct__option"
              :class="{ 'is-active': locale === opt.value }"
              @click="setLocale(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>

        <RsButton class="acct__logout" variant="ghost" size="sm" @click="logout">
          {{ t('nav.logout') }}
        </RsButton>
      </div>
    </template>
  </RsPopover>
</template>

<style scoped>
.acct__who {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  max-width: 14rem;
  padding: 0.2rem 0.55rem 0.2rem 0.2rem;
  border-radius: 999px;
  border: 1px solid var(--shell-border);
  background: var(--shell-chip-bg);
  color: var(--shell-title);
  cursor: pointer;
}

.acct__who:hover,
.acct__who.is-open {
  background: var(--shell-nav-hover);
}

.acct__who-name {
  max-width: 8rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.82rem;
  font-weight: 600;
}

.acct__chevron {
  flex-shrink: 0;
  opacity: 0.7;
}

.acct {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
}

.acct__profile {
  display: flex;
  align-items: center;
  gap: 0.7rem;
}

.acct__name {
  font-size: 0.9rem;
  font-weight: 700;
  color: var(--rs-text);
}

.acct__id,
.acct__label {
  font-size: 0.75rem;
  color: var(--rs-muted);
}

.acct__label {
  margin-bottom: 0.35rem;
  font-weight: 600;
}

.acct__options {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.acct__option {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.3rem 0.55rem;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--rs-muted);
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
}

.acct__option:hover {
  background: var(--rs-item-hover);
  color: var(--rs-text);
}

.acct__option.is-active {
  background: color-mix(in srgb, var(--rs-primary) 14%, transparent);
  color: var(--rs-text);
}

.acct__logout {
  width: 100%;
  justify-content: flex-start;
  border-top: 1px solid var(--rs-border);
  border-radius: 0;
  padding-top: 0.65rem;
}

@media (max-width: 980px) {
  .acct__who-name {
    display: none;
  }
}
</style>
