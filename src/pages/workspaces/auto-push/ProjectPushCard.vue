<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsEmpty, RsFormItem, RsIcon, RsInput, useRsToast } from '@/ui'
import { errText, listProjectFiles, type ProjectFileEntry } from '../api'
import { keepDirsFirst } from '../lib/project-files'

const paths = defineModel<string[]>({ default: () => [] })
const props = defineProps<{
  projectId: string
  disabled?: boolean
}>()

const { t } = useI18n()
const toast = useRsToast()
const draft = ref('')
const dir = ref('')
const entries = ref<ProjectFileEntry[]>([])
const exists = ref(true)
const loading = ref(false)

const selected = computed(() => new Set(paths.value))
const rows = computed(() => [...entries.value].sort(keepDirsFirst))
const crumbs = computed(() => {
  const parts = dir.value.split('/').filter(Boolean)
  const items = [{ label: t('workspaces.detail.root'), path: '' }]
  let acc = ''
  for (const part of parts) {
    acc = acc ? `${acc}/${part}` : part
    items.push({ label: part, path: acc })
  }
  return items
})

function normalize(raw: string): string {
  return raw.trim().replaceAll('\\', '/')
}

function addPath(raw: string): void {
  const text = normalize(raw)
  if (!text || text === '.' || text === '*' || text === '**' || paths.value.includes(text)) return
  paths.value = [...paths.value, text]
}

function removePath(raw: string): void {
  paths.value = paths.value.filter((item) => item !== raw)
}

function togglePath(raw: string): void {
  if (selected.value.has(raw)) removePath(raw)
  else addPath(raw)
}

function addDraft(): void {
  for (const part of draft.value.split(/[\n,;]+/)) addPath(part)
  draft.value = ''
}

async function load(path = dir.value): Promise<void> {
  if (!props.projectId) return
  loading.value = true
  try {
    const tree = await listProjectFiles(props.projectId, path)
    dir.value = tree.path || path
    exists.value = tree.exists !== false
    entries.value = tree.entries ?? []
  } catch (e) {
    toast.error(errText(e))
  } finally {
    loading.value = false
  }
}

function openEntry(row: ProjectFileEntry): void {
  if (row.is_dir) {
    void load(row.path)
    return
  }
  togglePath(row.path)
}

watch(
  () => props.projectId,
  (id) => {
    dir.value = ''
    entries.value = []
    if (id) void load('')
  },
  { immediate: true },
)
</script>

<template>
  <div class="picker">
    <RsFormItem :label="t('workspaces.autoPushFiles')" :tooltip="t('workspaces.autoPushPathsTip')" required>
      <ul v-if="paths.length" class="file-list" :aria-label="t('workspaces.autoPushFiles')">
        <li v-for="item in paths" :key="item" class="row picked">
          <span class="picked__name" :title="item">{{ item }}</span>
          <RsButton size="sm" variant="text" tone="danger" :disabled="disabled" @click="removePath(item)">
            {{ t('workspaces.autoPushRemove') }}
          </RsButton>
        </li>
      </ul>
      <p v-else class="hint">{{ t('workspaces.autoPushNoFiles') }}</p>
      <div class="add">
        <RsInput
          v-model="draft"
          :disabled="disabled"
          :placeholder="t('workspaces.autoPushPathsPh')"
          @keydown.enter.prevent="addDraft"
        />
        <RsButton size="sm" :disabled="disabled || !draft.trim()" @click="addDraft">
          {{ t('workspaces.autoPushAddPath') }}
        </RsButton>
      </div>
    </RsFormItem>
    <nav class="crumbs" :aria-label="t('workspaces.detail.nav.files')">
      <button
        v-for="(item, index) in crumbs"
        :key="item.path || 'root'"
        type="button"
        class="crumbs__item"
        :disabled="disabled || loading"
        @click="load(item.path)"
      >
        <span v-if="index">/</span>
        {{ item.label }}
      </button>
    </nav>
    <ul v-if="rows.length || loading" class="browse" :aria-busy="loading">
      <li v-for="row in rows" :key="row.path" class="browse__row" :class="{ 'is-on': selected.has(row.path) }">
        <button type="button" class="browse__name" :disabled="disabled" @click="openEntry(row)">
          <RsIcon :name="row.is_dir ? 'folder' : 'file'" :size="16" />
          <span>{{ row.name }}{{ row.is_dir ? '/' : '' }}</span>
        </button>
        <RsButton size="sm" variant="text" tone="primary" :disabled="disabled" @click="togglePath(row.path)">
          {{ selected.has(row.path) ? t('workspaces.autoPushUnpick') : t('workspaces.autoPushPick') }}
        </RsButton>
      </li>
    </ul>
    <RsEmpty
      v-else
      :description="exists ? t('workspaces.detail.emptyDir') : t('workspaces.detail.noCheckout')"
    />
  </div>
</template>

<style scoped>
.picker {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  min-width: 0;
}

.file-list {
  margin: 0 0 0.45rem;
  padding: 0;
  list-style: none;
  border: 1px solid var(--shell-border);
  border-radius: 0.45rem;
  overflow: auto;
  max-height: 7.5rem;
}

.row {
  display: flex;
  align-items: center;
  min-height: 32px;
  padding: 0 0.35rem 0 var(--rs-space-md);
  border-bottom: 1px solid var(--rs-border-subtle);
  font-size: var(--rs-font-size-sm);
}

.row:last-child {
  border-bottom: 0;
}

.picked__name {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.add {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.hint {
  margin: 0 0 0.4rem;
  font-size: 0.78rem;
  color: var(--shell-muted);
}

.crumbs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.15rem 0.2rem;
  padding: 0.35rem 0.55rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.45rem;
  background: var(--shell-inset-bg);
}

.crumbs__item {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0;
  border: 0;
  background: transparent;
  color: #0ea5e9;
  font: inherit;
  font-size: 0.82rem;
  cursor: pointer;
}

.crumbs__item:disabled {
  cursor: default;
  opacity: 0.7;
}

.crumbs__item span {
  color: var(--shell-muted);
}

.browse {
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 10rem;
  overflow: auto;
  border: 1px solid var(--shell-border);
  border-radius: 0.45rem;
}

.browse__row {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 32px;
  padding: 0 0.35rem 0 0.55rem;
  border-bottom: 1px solid var(--rs-border-subtle);
}

.browse__row:last-child {
  border-bottom: 0;
}

.browse__row.is-on {
  background: color-mix(in srgb, var(--rs-primary) 10%, transparent);
}

.browse__name {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  min-width: 0;
  flex: 1;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 0.82rem;
  text-align: left;
  cursor: pointer;
}
</style>
