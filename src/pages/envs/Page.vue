<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import {
  RsButton,
  RsCard,
  RsDialog,
  RsInput,
  RsTable,
  RsTabs,
  rsConfirm,
  useRsToast,
  type RsTabItem,
  type RsTableColumn,
} from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import { useWorkspaceStore } from '@/stores/workspace'
import EnvEditor from './EnvEditor.vue'
import {
  FALLBACK_KINDS,
  canonicalKind,
  defaultsOf,
  findKind,
  kindTagClass,
  type EnvKindSpec,
  type EnvRow,
} from './kinds'
import './env-panel.css'

const { t } = useI18n()
const route = useRoute()
const toast = useRsToast()
const ws = useWorkspaceStore()
const rows = ref<EnvRow[]>([])
const kinds = ref<EnvKindSpec[]>(FALLBACK_KINDS)
const activeKind = ref('all')
const editing = ref(false)
const saving = ref(false)
const probing = ref(false)
const verifyingId = ref('')
const probeResult = ref<{ ok: boolean; message: string } | null>(null)
const listTestOpen = ref(false)
const listTestTo = ref('')
const listTestRow = ref<EnvRow | null>(null)
const recId = ref('')
const envKind = ref('ftp')
const envName = ref('')
const isEnabled = ref(1)
const config = ref<Record<string, unknown>>({})
const secretsSet = ref<string[]>([])

const tabItems = computed<RsTabItem[]>(() => [
  { value: 'all', label: t('envs.tabAll') },
  ...kinds.value.map((item) => ({
    value: item.kind,
    label: kindLabel(item.kind),
  })),
])

const filtered = computed(() => {
  if (activeKind.value === 'all') return rows.value
  const k = canonicalKind(activeKind.value)
  return rows.value.filter((row) => canonicalKind(row.env_kind) === k)
})

const columns = computed<RsTableColumn<EnvRow>[]>(() => [
  { key: 'env_name', title: t('envs.name'), minWidth: 160 },
  { key: 'env_kind', title: t('envs.kind'), width: 120 },
  { key: 'config_hint', title: t('envs.hintCol'), minWidth: 220 },
  { key: 'is_enabled', title: t('envs.enabled'), width: 96, align: 'center' },
  { key: 'actions', title: t('common.actions'), width: 220, align: 'right' },
])

const emptyTitle = computed(() => (activeKind.value === 'all' ? t('envs.empty') : t('envs.emptyKind')))
const emptyHint = computed(() => (activeKind.value === 'all' ? t('envs.emptyHint') : ''))

function kindLabel(code: string): string {
  const key = `envs.kinds.${canonicalKind(code)}`
  const label = t(key)
  return label === key ? code : label
}

function blankConfig(kind: string): Record<string, unknown> {
  const spec = findKind(kinds.value, kind)
  return spec ? defaultsOf(spec) : {}
}

function openCreate(): void {
  const kind = activeKind.value === 'all' ? 'ftp' : canonicalKind(activeKind.value)
  recId.value = ''
  envKind.value = kind
  envName.value = ''
  isEnabled.value = 1
  config.value = blankConfig(kind)
  secretsSet.value = []
  probeResult.value = null
  editing.value = true
}

async function openEdit(row: EnvRow): Promise<void> {
  try {
    const detail = await request<EnvRow>({ method: 'GET', url: apiPath('envs', row.rec_id) })
    recId.value = detail.rec_id
    envKind.value = canonicalKind(detail.env_kind)
    envName.value = detail.env_name
    isEnabled.value = detail.is_enabled === 0 ? 0 : 1
    config.value = { ...blankConfig(detail.env_kind), ...detail.config }
    secretsSet.value = detail.secrets_set ?? []
    probeResult.value = null
    editing.value = true
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}

function closeEditor(): void {
  editing.value = false
  recId.value = ''
  probeResult.value = null
}

async function loadKinds(): Promise<void> {
  try {
    const list = await request<EnvKindSpec[]>({ method: 'GET', url: apiPath('envs', 'kinds') })
    if (list?.length) kinds.value = list
  } catch {
    kinds.value = FALLBACK_KINDS
  }
}

async function load(): Promise<void> {
  if (!ws.currentId) return
  try {
    rows.value =
      (await request<EnvRow[]>({
        method: 'GET',
        url: apiPath('envs'),
        params: { workspace_rec_id: ws.currentId },
      })) ?? []
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}

function applyQuery(): void {
  const kind = String(route.query.kind ?? '')
  if (kind && kind !== 'all') {
    activeKind.value = canonicalKind(kind)
  }
  if (String(route.query.create ?? '') === '1') {
    openCreate()
  }
}

onMounted(async () => {
  await loadKinds()
  await load()
  applyQuery()
})
watch(
  () => ws.currentId,
  () => {
    closeEditor()
    void load()
  },
)
watch(envKind, () => {
  if (editing.value) probeResult.value = null
})

function formatProbeMessage(res: { ok: boolean; message: string; details?: Record<string, string> }): string {
  const lines = [res.message]
  if (res.details) {
    for (const [key, value] of Object.entries(res.details)) {
      if (value) lines.push(`${key}: ${value}`)
    }
  }
  return lines.filter(Boolean).join('\n')
}

async function probeDraft(testTo = ''): Promise<void> {
  if (!ws.currentId) return
  if (envKind.value === 'smtp' && !testTo.trim()) {
    toast.error(t('envs.testRecipientRequired'))
    return
  }
  probing.value = true
  probeResult.value = null
  try {
    const res = await request<{ ok: boolean; message: string; details?: Record<string, string> }>({
      method: 'POST',
      url: apiPath('envs', 'probe'),
      data: {
        workspace_rec_id: ws.currentId,
        rec_id: recId.value,
        env_kind: envKind.value,
        config: config.value,
        test_to: testTo.trim(),
      },
    })
    probeResult.value = { ok: res.ok, message: formatProbeMessage(res) }
    if (res.ok) toast.success(t('envs.verifyOk'))
    else toast.error(t('envs.verifyFail'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    probing.value = false
  }
}

function startProbeRow(row: EnvRow): void {
  if (canonicalKind(row.env_kind) === 'smtp') {
    listTestRow.value = row
    listTestTo.value = ''
    listTestOpen.value = true
    return
  }
  void probeRow(row)
}

async function confirmListTest(): Promise<void> {
  const row = listTestRow.value
  const to = listTestTo.value.trim()
  if (!row) return
  if (!to) {
    toast.error(t('envs.testRecipientRequired'))
    return
  }
  listTestOpen.value = false
  await probeRow(row, to)
}

async function probeRow(row: EnvRow, testTo = ''): Promise<void> {
  if (!ws.currentId) return
  verifyingId.value = row.rec_id
  try {
    const res = await request<{ ok: boolean; message: string; details?: Record<string, string> }>({
      method: 'POST',
      url: apiPath('envs', 'probe'),
      data: {
        workspace_rec_id: ws.currentId,
        rec_id: row.rec_id,
        env_kind: row.env_kind,
        test_to: testTo,
      },
    })
    const msg = formatProbeMessage(res)
    if (res.ok) toast.success(msg || t('envs.verifyOk'))
    else toast.error(msg || t('envs.verifyFail'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    verifyingId.value = ''
  }
}

async function save(): Promise<void> {
  if (!ws.currentId) return
  saving.value = true
  try {
    await request({
      method: 'POST',
      url: apiPath('envs'),
      data: {
        rec_id: recId.value,
        workspace_rec_id: ws.currentId,
        env_kind: envKind.value,
        env_name: envName.value,
        is_enabled: isEnabled.value,
        config: config.value,
      },
    })
    toast.success(t('common.saved'))
    closeEditor()
    await load()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

async function remove(row: EnvRow): Promise<void> {
  const ok = await rsConfirm.danger({
    title: t('common.delete'),
    description: t('envs.deleteConfirm', { name: row.env_name }),
    confirmText: t('common.delete'),
  })
  if (!ok) return
  try {
    await request({ method: 'DELETE', url: apiPath('envs', row.rec_id) })
    toast.success(t('common.deleted'))
    if (recId.value === row.rec_id) closeEditor()
    await load()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
}
</script>

<template>
  <div class="env-embed">
    <div class="env-embed__bar">
      <div>
        <h2 class="env-embed__title">{{ t('envs.title') }}</h2>
        <p class="env-embed__desc">{{ t('envs.desc') }}</p>
      </div>
      <div class="env-embed__actions">
        <RsButton v-if="editing" variant="ghost" size="sm" @click="closeEditor">{{ t('common.back') }}</RsButton>
        <RsButton v-else size="sm" @click="openCreate">{{ t('common.add') }}</RsButton>
        <RsButton v-if="!editing" variant="ghost" size="sm" @click="load">{{ t('common.refresh') }}</RsButton>
      </div>
    </div>
    <div class="env-embed__body">
      <EnvEditor
        v-if="editing"
        :kinds="kinds"
        :rec-id="recId"
        :env-kind="envKind"
        :env-name="envName"
        :is-enabled="isEnabled"
        :config="config"
        :secrets-set="secretsSet"
        :saving="saving"
        :probing="probing"
        :kind-locked="!!recId"
        :probe-result="probeResult"
        @update:env-kind="envKind = $event"
        @update:env-name="envName = $event"
        @update:is-enabled="isEnabled = $event"
        @update:config="config = $event"
        @save="save"
        @cancel="closeEditor"
        @probe="probeDraft"
        @clear-probe="probeResult = null"
      />

      <RsCard v-else variant="outlined" elevated radius="lg" :padding="false" class="env-list">
        <div class="env-list__tabs">
          <RsTabs v-model="activeKind" :items="tabItems" panelless variant="line" />
        </div>

        <RsTable v-if="filtered.length" :columns="columns" :data="filtered" row-key="rec_id" :bordered="false">
          <template #env_name="{ row }">
            <span class="env-list__name">{{ row.env_name }}</span>
          </template>
          <template #env_kind="{ row }">
            <span :class="kindTagClass(row.env_kind, kinds)">{{ kindLabel(row.env_kind) }}</span>
          </template>
          <template #config_hint="{ row }">
            <span class="env-list__hint">{{ row.config_hint || t('common.dash') }}</span>
          </template>
          <template #is_enabled="{ row }">
            <span class="env-list__state" :class="{ 'is-on': row.is_enabled !== 0 }">
              {{ row.is_enabled === 0 ? t('envs.disabled') : t('envs.enabled') }}
            </span>
          </template>
          <template #actions="{ row }">
            <div class="env-list__actions">
              <RsButton size="sm" variant="text" tone="primary" @click="openEdit(row)">{{ t('common.edit') }}</RsButton>
              <RsButton size="sm" variant="text" tone="success" :disabled="verifyingId === row.rec_id" @click="startProbeRow(row)">
                {{ t('envs.verify') }}
              </RsButton>
              <RsButton size="sm" variant="text" tone="danger" @click="remove(row)">{{ t('common.delete') }}</RsButton>
            </div>
          </template>
        </RsTable>

        <div v-else class="env-list__empty">
          <p class="env-list__empty-title">{{ emptyTitle }}</p>
          <p v-if="emptyHint" class="env-list__empty-hint">{{ emptyHint }}</p>
        </div>
      </RsCard>
    </div>

    <RsDialog v-model:open="listTestOpen" layout="form" width="sm" :title="t('envs.testConnection')">
      <template #body>
        <p class="env-embed__desc">{{ t('envs.testRecipientHint') }}</p>
        <RsInput v-model="listTestTo" :placeholder="t('envs.testRecipient')" />
      </template>
      <template #footer>
        <RsButton variant="ghost" @click="listTestOpen = false">{{ t('common.cancel') }}</RsButton>
        <RsButton @click="confirmListTest">{{ t('envs.verify') }}</RsButton>
      </template>
    </RsDialog>
  </div>
</template>

<style scoped>
.env-embed {
  display: flex;
  flex-direction: column;
}

.env-embed__bar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-shrink: 0;
  padding: 0 0 1rem;
}

.env-embed__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 650;
  color: var(--shell-title);
}

.env-embed__desc {
  margin: 0.3rem 0 0;
  max-width: 40rem;
  font-size: 0.8rem;
  line-height: 1.55;
  color: var(--shell-muted);
}

.env-embed__actions {
  display: inline-flex;
  gap: 0.4rem;
  flex-shrink: 0;
}

.env-embed__body {
  min-width: 0;
}

.env-list {
  overflow: hidden;
}

.env-list :deep(.rs-table__row:last-child .rs-table__td) {
  border-bottom: none;
}

.env-list__tabs {
  padding: 0 1.15rem;
  border-bottom: 1px solid var(--shell-border);
}

.env-list__tabs :deep(.rs-tabs--line.rs-tabs--panelless .rs-tabs__nav) {
  background: transparent;
  border-bottom: none;
}

.env-list__tabs :deep(.rs-tabs--line .rs-tabs__trigger) {
  height: 2.75rem;
  font-size: 0.875rem;
}

.env-list__tabs :deep(.rs-tabs--line .rs-tabs__trigger[data-state='active']::after) {
  inset-inline: 0.35rem;
}

.env-list__name {
  font-weight: 550;
  color: var(--shell-title);
}

.env-list__hint {
  color: var(--shell-muted);
}

.env-list__state {
  font-size: 0.8rem;
  color: var(--shell-muted);
}

.env-list__state.is-on {
  color: #059669;
  font-weight: 550;
}

.env-list__actions {
  display: inline-flex;
  justify-content: flex-end;
  align-items: center;
  gap: 0.15rem;
}

.env-list__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 2.4rem 1.25rem 2.75rem;
  text-align: center;
}

.env-list__empty-title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 550;
  color: var(--shell-title);
}

.env-list__empty-hint {
  margin: 0.45rem 0 0;
  max-width: 22rem;
  font-size: 0.82rem;
  line-height: 1.6;
  color: var(--shell-muted);
}
</style>
