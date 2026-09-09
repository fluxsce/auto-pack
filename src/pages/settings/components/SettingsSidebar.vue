<script setup lang="ts">
import { RsIcon } from '@/ui'
import { useI18n } from 'vue-i18n'
import { SETTINGS_SECTIONS, type SettingsSection } from '../sections'

defineProps<{
  selected: SettingsSection
}>()

const emit = defineEmits<{
  select: [id: SettingsSection]
}>()

const { t } = useI18n()
</script>

<template>
  <aside class="sidebar" :aria-label="t('settings.title')">
    <div class="sidebar__title">{{ t('settings.nav') }}</div>
    <nav class="sidebar__nav">
      <button
        v-for="item in SETTINGS_SECTIONS"
        :key="item.id"
        type="button"
        class="sidebar__item"
        :class="{ 'sidebar__item--on': selected === item.id }"
        :aria-current="selected === item.id ? 'page' : undefined"
        @click="emit('select', item.id)"
      >
        <span class="sidebar__icon" aria-hidden="true">
          <RsIcon :name="item.icon" :size="16" />
        </span>
        <span class="sidebar__text">
          <span class="sidebar__name">{{ t(item.labelKey) }}</span>
          <span class="sidebar__meta">{{ t(item.hintKey) }}</span>
        </span>
      </button>
    </nav>
  </aside>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  width: 16.5rem;
  flex-shrink: 0;
  align-self: stretch;
  border-right: 1px solid var(--shell-border);
  background: color-mix(in srgb, var(--shell-title) 2%, var(--shell-header-bg, transparent));
}

.sidebar__title {
  padding: 0.85rem 1rem 0.45rem;
  font-size: 0.7rem;
  font-weight: 650;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--shell-muted);
}

.sidebar__nav {
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding: 0.2rem 0 0.8rem;
}

.sidebar__item {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  width: 100%;
  padding: 0.65rem 0.9rem;
  border: none;
  border-left: 2px solid transparent;
  background: transparent;
  color: var(--shell-title);
  text-align: left;
  cursor: pointer;
}

.sidebar__item:hover {
  background: var(--shell-nav-hover);
}

.sidebar__item--on {
  background: rgba(14, 165, 233, 0.12);
  border-left-color: #0ea5e9;
}

.sidebar__icon {
  display: inline-flex;
  color: var(--shell-muted);
  flex-shrink: 0;
}

.sidebar__item--on .sidebar__icon {
  color: #0ea5e9;
}

.sidebar__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.sidebar__name,
.sidebar__meta {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar__name {
  font-size: 0.875rem;
  font-weight: 600;
}

.sidebar__meta {
  font-size: 0.72rem;
  color: var(--shell-muted);
}

@media (max-width: 860px) {
  .sidebar {
    width: 100%;
    border-right: none;
    border-bottom: 1px solid var(--shell-border);
  }
}
</style>
