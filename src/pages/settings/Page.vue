<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { SETTINGS_SECTIONS, type SettingsSection } from './sections'
import SettingsSidebar from './components/SettingsSidebar.vue'
import PropsPane from './components/PropsPane.vue'
import VarsPane from './components/VarsPane.vue'
import EnvsPage from '@/pages/envs/Page.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

function parseSection(raw: unknown): SettingsSection {
  const id = String(raw ?? '')
  if (SETTINGS_SECTIONS.some((item) => item.id === id)) return id as SettingsSection
  return 'sdk'
}

const section = computed({
  get: () => parseSection(route.query.section),
  set: (id: SettingsSection) => {
    void router.replace({ name: 'settings', query: { ...route.query, section: id } })
  },
})

watch(
  () => route.query.section,
  (raw) => {
    const next = parseSection(raw)
    if (String(raw ?? '') !== next) {
      void router.replace({ name: 'settings', query: { ...route.query, section: next } })
    }
  },
  { immediate: true },
)
</script>

<template>
  <section class="settings">
    <header class="settings__head">
      <div>
        <h1 class="settings__title">{{ t('settings.title') }}</h1>
        <p class="settings__desc">{{ t('settings.desc') }}</p>
      </div>
    </header>
    <div class="settings__body">
      <SettingsSidebar :selected="section" @select="section = $event" />
      <div class="settings__main">
        <EnvsPage v-if="section === 'sdk'" />
        <PropsPane v-else-if="section === 'props'" />
        <VarsPane v-else-if="section === 'vars'" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
}

.settings__head {
  flex-shrink: 0;
  padding: 0 0 1rem;
}

.settings__title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--shell-title);
}

.settings__desc {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
  line-height: 1.55;
  color: var(--shell-muted);
}

.settings__body {
  display: flex;
  align-items: stretch;
  border: 1px solid var(--shell-border);
  border-radius: 12px;
  background: var(--shell-header-bg, transparent);
}

.settings__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: 1rem 1.15rem 1.15rem;
}

@media (max-width: 860px) {
  .settings__body {
    flex-direction: column;
  }
}
</style>
