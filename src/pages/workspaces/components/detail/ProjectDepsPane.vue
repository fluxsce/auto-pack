<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsEmpty, RsTable, RsTag, useRsToast, type RsTableColumn } from '@/ui'
import { errText, listProjectDeps, type ProjectDepRow } from '../../api'

const props = defineProps<{
  projectId: string
}>()

const { t } = useI18n()
const toast = useRsToast()
const rows = ref<ProjectDepRow[]>([])
const sources = ref<string[]>([])
const exists = ref(true)
const cached = ref(false)
const direct = ref(0)
const loading = ref(false)
const directOnly = ref(true)

const visibleRows = computed(() => {
  if (!directOnly.value) return rows.value
  return rows.value.filter((row) => (row.depth ?? 0) <= 1)
})

const columns = computed<RsTableColumn<ProjectDepRow>[]>(() => {
  const cols: RsTableColumn<ProjectDepRow>[] = [
    { key: 'name', title: t('workspaces.name'), minWidth: 280 },
    { key: 'version', title: t('workspaces.detail.version'), width: 140 },
    { key: 'extra', title: t('workspaces.detail.scope'), width: 110 },
    { key: 'note', title: t('workspaces.detail.note'), minWidth: 180 },
  ]
  if (sources.value.length > 1) {
    cols.push({ key: 'source', title: t('workspaces.detail.source'), minWidth: 160 })
  }
  return cols
})

function kindLabel(extra?: string): string {
  const key = extra?.trim()
  if (!key) return ''
  const path = `workspaces.detail.depKind.${key}`
  const label = t(path)
  return label === path ? key : label
}

async function load(refresh = false): Promise<void> {
  if (!props.projectId) return
  loading.value = true
  if (refresh) {
    rows.value = []
    sources.value = []
  }
  exists.value = true
  cached.value = false
  try {
    const body = await listProjectDeps(props.projectId, refresh)
    exists.value = body.exists
    cached.value = Boolean(body.cached)
    sources.value = body.sources ?? []
    direct.value = body.direct ?? 0
    rows.value = body.rows ?? []
  } catch (e) {
    toast.error(errText(e))
  } finally {
    loading.value = false
  }
}

watch(() => props.projectId, () => {
  void load()
}, { immediate: true })
</script>

<template>
  <section class="pane">
    <header class="pane-head">
      <h3>{{ t('workspaces.detail.nav.deps') }}</h3>
      <span v-if="loading" class="pane-meta">{{ t('workspaces.detail.depsRunning') }}</span>
      <span v-else-if="cached" class="pane-meta">{{ t('workspaces.detail.depsCached') }}</span>
      <span v-else-if="sources.length" class="pane-meta mono">{{ sources[0] }}</span>
      <span v-if="direct" class="pane-meta">{{ t('workspaces.detail.depsDirect', { n: direct }) }}</span>
      <span v-if="rows.length" class="pane-meta">{{ rows.length }}</span>
      <RsButton size="sm" variant="ghost" :disabled="directOnly" @click="directOnly = true">
        {{ t('workspaces.detail.depsDirectOnly') }}
      </RsButton>
      <RsButton size="sm" variant="ghost" :disabled="!directOnly" @click="directOnly = false">
        {{ t('workspaces.detail.depsAll') }}
      </RsButton>
      <RsButton size="sm" variant="ghost" :loading="loading" @click="load(true)">{{ t('common.refresh') }}</RsButton>
      <p class="pane-desc">{{ t('workspaces.detail.depsHint') }}</p>
    </header>
    <div v-if="visibleRows.length" class="block grow table-wrap">
      <RsTable fill :columns="columns" :data="visibleRows" row-key="key" :bordered="false">
        <template #name="{ row }">
          <span class="dep-name mono" :style="{ paddingLeft: `${(row.depth || 0) * 1.05}rem` }">{{ row.name }}</span>
        </template>
        <template #extra="{ row }">
          <RsTag v-if="row.extra" round>{{ kindLabel(row.extra) }}</RsTag>
        </template>
        <template #note="{ row }">
          <span v-if="row.note" class="pane-meta">{{ row.note }}</span>
        </template>
      </RsTable>
    </div>
    <RsEmpty
      v-else-if="!loading"
      :description="exists ? t('workspaces.detail.emptyDeps') : t('workspaces.detail.noCheckout')"
    />
  </section>
</template>

<style scoped src="./pane.css"></style>
<style scoped>
.table-wrap {
  padding: 0.35rem 0.4rem 0.5rem;
}

.dep-name {
  display: inline-block;
  font-size: 0.8rem;
}
</style>
