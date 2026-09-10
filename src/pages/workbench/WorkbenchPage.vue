<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { RsSelect, type RsSelectModelValue } from '@/ui'
import { apiPath } from '@/api/prefix'
import { request, setRequestTimeout } from '@/api/http'
import { getProps } from '@/pages/settings/api'
import { useWorkspaceStore } from '@/stores/workspace'
import TechBackdrop from '@/components/TechBackdrop.vue'
import AccountPopover from './AccountPopover.vue'
import brandMark from '@/assets/brand.png'

const { t } = useI18n()
const route = useRoute()
const ws = useWorkspaceStore()
const version = ref('')

const modules = computed(() => [
  { id: 'workspaces', label: t('nav.workspaces') },
  { id: 'jobs', label: t('nav.jobs') },
  { id: 'settings', label: t('nav.settings') },
  { id: 'users', label: t('nav.users') },
  { id: 'audit', label: t('nav.audit') },
])

const workspaceOptions = computed(() =>
  ws.list.map((item) => ({ value: item.rec_id, label: item.workspace_name })),
)

onMounted(() => {
  void ws.load()
  void getProps()
    .then((p) => setRequestTimeout(p.web_timeout_sec))
    .catch(() => undefined)
  void request<{ version?: string }>({ method: 'GET', url: apiPath('health') })
    .then((res) => {
      version.value = res.version?.trim() ?? ''
    })
    .catch(() => {
      version.value = ''
    })
})

function isNavActive(id: string): boolean {
  if (id === 'workspaces') return route.path === '/workspaces' || route.path.startsWith('/workspaces/')
  if (id === 'settings') return route.name === 'settings' || route.name === 'envs'
  return route.name === id
}

function onWorkspace(value: RsSelectModelValue): void {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw == null || raw === '') return
  const id = typeof raw === 'object' ? raw.value : raw
  ws.select(String(id))
}

</script>

<template>
  <div class="wb tech-shell">
    <TechBackdrop />
    <header class="wb__header">
      <RouterLink class="wb__brand" :to="{ name: 'dashboard' }">
        <img class="wb__logo" :src="brandMark" alt="" aria-hidden="true" />
        <span class="wb__brand-text">
          <span>{{ t('brand.appTitle') }}</span>
          <span v-if="version" class="wb__ver">v{{ version }}</span>
        </span>
      </RouterLink>

      <nav class="wb__nav" :aria-label="t('nav.aria')">
        <RouterLink
          v-for="item in modules"
          :key="item.id"
          :to="{ name: item.id }"
          class="wb__nav-item"
          :class="{ 'is-active': isNavActive(item.id) }"
        >
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="wb__actions">
        <RsSelect
          v-if="ws.list.length"
          class="wb__ws"
          :model-value="ws.currentId"
          :options="workspaceOptions"
          :aria-label="t('nav.workspace')"
          @update:model-value="onWorkspace"
        />
        <AccountPopover />
      </div>
    </header>

    <main class="wb__stage">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.wb {
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 100%;
  overflow: hidden;
  color: var(--shell-title);
  background: transparent;
}

.wb__header,
.wb__stage {
  position: relative;
  z-index: 1;
}

.wb__header {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 1rem;
  min-height: 64px;
  padding: 0 clamp(16px, 3vw, 32px);
  border-bottom: 1px solid var(--shell-border);
  background: var(--shell-header-bg);
  backdrop-filter: blur(16px);
  box-shadow: var(--shell-header-shadow);
}

.wb__brand {
  display: inline-flex;
  align-items: center;
  gap: 0.7rem;
  min-width: 0;
  color: inherit;
  text-decoration: none;
}

.wb__logo {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
  box-shadow: 0 0 0 2px rgba(14, 165, 233, 0.28);
}

.wb__brand-text {
  display: inline-flex;
  align-items: baseline;
  gap: 0.45rem;
  font-size: 0.95rem;
  font-weight: 750;
  letter-spacing: 0.02em;
}

.wb__ver {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--shell-muted);
}

.wb__nav {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  min-width: 0;
  gap: 0.2rem;
}

.wb__nav-item {
  display: inline-flex;
  align-items: center;
  height: 40px;
  padding: 0 0.95rem;
  border-radius: 8px;
  color: var(--shell-muted);
  font-size: 0.9rem;
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
  transition:
    color 0.18s ease,
    background 0.18s ease,
    box-shadow 0.18s ease;
}

.wb__nav-item:hover {
  color: var(--shell-title);
  background: var(--shell-nav-hover);
}

.wb__nav-item.is-active {
  color: var(--shell-title);
  background: rgba(14, 165, 233, 0.16);
  box-shadow: inset 0 -2px 0 #0ea5e9;
}

.wb__actions {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  flex-shrink: 0;
  margin-left: auto;
}

.wb__ws {
  min-width: 11.5rem;
}

.wb__stage {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 28px clamp(16px, 3.5vw, 40px) 40px;
}

@media (max-width: 980px) {
  .wb__header {
    flex-wrap: wrap;
    padding-top: 0.7rem;
    padding-bottom: 0.7rem;
  }

  .wb__nav {
    order: 3;
    flex: 1 1 100%;
    justify-content: flex-start;
    overflow: auto;
  }
}
</style>
