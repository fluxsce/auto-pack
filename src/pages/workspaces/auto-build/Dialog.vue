<script setup lang="ts">
import { computed, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsAlert, RsButton, RsCheckbox, RsDialog, RsFieldset, RsFormItem, RsLog, RsRadio, RsRadioItem, RsSelect, RsTag, useRsToast } from '@/ui'
import type { Workspace } from '@/stores/workspace'
import {
  autoBuildHistory,
  autoBuildLog,
  autoBuildStatus,
  errText,
  fetchMavenOrder,
  listProjects,
  startAutoBuild,
  type MavenAnalysis,
  type WorkspaceBuildRecord,
} from '../api'
import { typeLabel, typeVariant } from '../composables/use-project-list'
import { type AutoBuildScope } from './session'
import { autoBuildBag, autoBuildEpoch, bumpAutoBuild, isAutoBuildLive, stopAutoBuildPolls, type AutoBuildBag } from './runtime'
import { setAutoBuildBusy, useBatchActivity } from '../composables/use-batch-activity'
import { cyclicRecIds, isOrderedBuildType, orderProjectsForBuild } from '../lib/ordered-build'
import { syncSourcesWithLog } from '../lib/sync-log'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ workspace?: Workspace | null }>()
const emit = defineEmits<{ success: [] }>()

const { t } = useI18n()
const toast = useRsToast()
const wsId = computed(() => props.workspace?.rec_id || '')

function bag(id = wsId.value): AutoBuildBag {
  void autoBuildEpoch.value
  return autoBuildBag(id)
}

function touch(id = wsId.value): void {
  bumpAutoBuild()
  if (id) setAutoBuildBusy(id, isAutoBuildLive(id))
}

const scope = computed({
  get: () => bag().scope,
  set: (v: AutoBuildScope) => {
    bag().scope = v
    touch()
  },
})
const selectedRecIds = computed({
  get: () => bag().selectedRecIds,
  set: (v: string[]) => {
    bag().selectedRecIds = v
    touch()
  },
})
const includeNonMaven = computed({
  get: () => bag().includeNonMaven,
  set: (v: boolean) => {
    bag().includeNonMaven = v
    touch()
  },
})
const loadingPlan = computed(() => bag().loadingPlan)
const running = computed(() => bag().running)
const logText = computed({
  get: () => bag().logText,
  set: (v: string) => {
    bag().logText = v
    touch()
  },
})
const planned = computed(() => bag().planned)
const warnings = computed(() => bag().warnings)
const projects = computed(() => bag().projects)
const analysis = computed(() => bag().analysis)
const syncHint = computed(() => bag().syncHint)
const history = computed(() => bag().history)
const selectedHistory = computed(() => bag().selectedHistory)

const enabledProjects = computed(() => projects.value.filter((p) => p.is_enabled !== 0))
const selectable = computed(() => [...enabledProjects.value].sort((a, b) => a.project_name.localeCompare(b.project_name)))
const projectOptions = computed(() => selectable.value.map((p) => ({ value: p.rec_id, label: p.project_name })))
const { releaseBusy, refresh: refreshBatch } = useBatchActivity(() => wsId.value)
const occupyTip = computed(() => {
  if (releaseBusy.value && !running.value) return t('workspaces.autoBatchBusyRelease')
  if (running.value) return t('workspaces.autoBatchBusyBuild')
  return ''
})
const startBlocked = computed(() => releaseBusy.value && !running.value)
const busySelf = computed(() => running.value || loadingPlan.value)
const canSubmit = computed(() => planned.value.length > 0 && !busySelf.value && !startBlocked.value && (scope.value !== 'selected' || selectedRecIds.value.length > 0))
const cycleIds = computed(() => (analysis.value ? cyclicRecIds(analysis.value) : new Set<string>()))
const coordById = computed(() => {
  const map = new Map<string, string>()
  for (const m of analysis.value?.full_build_order ?? []) {
    if (m.coordinates) map.set(m.project_rec_id, m.coordinates)
  }
  return map
})
const selectedSet = computed(() => new Set(selectedRecIds.value))
const planRows = computed(() =>
  planned.value.map((p) => ({
    project: p,
    cyclic: cycleIds.value.has(p.rec_id),
    upstream: scope.value === 'selected' && !selectedSet.value.has(p.rec_id),
    coordinates: coordById.value.get(p.rec_id) || '',
  })),
)
const planCaption = computed(() => {
  if (loadingPlan.value && !analysis.value) return t('workspaces.autoBuildPlanProvisional')
  if (analysis.value) return t('workspaces.autoBuildPlanReady')
  return t('workspaces.autoBuildPlanProvisional')
})
const planExtra = computed(() => {
  const n = planned.value.length
  if (loadingPlan.value) {
    const hint = syncHint.value.trim()
    return hint ? `${n} · ${hint}` : `${n} · ${t('workspaces.autoBuildPlanLoading')}`
  }
  return String(n)
})
const planEmptyText = computed(() => {
  if (loadingPlan.value) return syncHint.value || t('workspaces.autoBuildPlanLoading')
  if (scope.value === 'selected' && !selectedRecIds.value.length) return t('workspaces.autoBuildTargetsPh')
  if (!enabledProjects.value.length) return t('workspaces.autoBuildPlanEmptyHint')
  return t('workspaces.autoBuildPlanEmpty')
})

function stamp(): string {
  return new Date().toLocaleString()
}

function isSyncInProgress(e: unknown): boolean {
  return /source sync already in progress/i.test(errText(e))
}

function appendLog(workspaceId: string, line: string): void {
  const slot = autoBuildBag(workspaceId)
  slot.logText = slot.logText ? `${slot.logText}\n${line}` : line
  touch(workspaceId)
}

async function syncSourcesFollow(workspaceId: string): Promise<Awaited<ReturnType<typeof syncSourcesWithLog>>> {
  const slot = autoBuildBag(workspaceId)
  for (;;) {
    try {
      return await syncSourcesWithLog(
        workspaceId,
        t,
        (line) => appendLog(workspaceId, line),
        (hint) => {
          slot.syncHint = hint
          touch(workspaceId)
        },
      )
    } catch (e) {
      if (!isSyncInProgress(e)) throw e
      slot.syncHint = t('workspaces.autoBuildSyncWait')
      touch(workspaceId)
      await new Promise((resolve) => setTimeout(resolve, 1500))
    }
  }
}

async function loadProjects(workspaceId: string): Promise<void> {
  autoBuildBag(workspaceId).projects = workspaceId ? await listProjects(workspaceId) : []
  touch(workspaceId)
}

function applyPlan(workspaceId: string, src: MavenAnalysis | null): void {
  const slot = autoBuildBag(workspaceId)
  const enabled = slot.projects.filter((p) => p.is_enabled !== 0)
  slot.planned = orderProjectsForBuild(enabled, src?.full_build_order ?? [], {
    targetRecIds: slot.scope === 'selected' ? slot.selectedRecIds : [],
    edges: src?.internal_edges,
    includeNonMaven: slot.scope === 'workspace' && slot.includeNonMaven,
  })
  touch(workspaceId)
}

function logPlan(workspaceId: string): void {
  const slot = autoBuildBag(workspaceId)
  if (!slot.planned.length) return
  appendLog(workspaceId, `[${stamp()}] ${t('workspaces.autoBuildPlanLine', { count: slot.planned.length })}`)
  const coords = new Map<string, string>()
  for (const m of slot.analysis?.full_build_order ?? []) {
    if (m.coordinates) coords.set(m.project_rec_id, m.coordinates)
  }
  slot.planned.forEach((p, i) => {
    const coord = coords.get(p.rec_id)
    const kind = typeLabel(p.project_type)
    appendLog(workspaceId, `  ${i + 1}. ${p.project_name}  [${kind}]${coord ? `  ${coord}` : ''}`)
  })
}

async function refreshPlan(workspaceId: string, resync: boolean): Promise<void> {
  const slot = autoBuildBag(workspaceId)
  if (!workspaceId || slot.running || (workspaceId === wsId.value && startBlocked.value)) return
  if (slot.planTask) return slot.planTask
  slot.loadingPlan = true
  touch(workspaceId)
  slot.planTask = (async () => {
    try {
      if (!slot.projects.length) await loadProjects(workspaceId)
      applyPlan(workspaceId, slot.analysis)
      const graph = slot.projects.filter((p) => p.is_enabled !== 0 && isOrderedBuildType(p.project_type))
      if (resync || !slot.analysis) {
        slot.warnings = []
        const sync = await syncSourcesFollow(workspaceId)
        appendLog(workspaceId, `[${stamp()}] ${t('workspaces.autoBuildSyncDone', { synced: sync.synced, skipped: sync.skipped, failed: sync.failed })}`)
        if (sync.failed_items?.length) slot.warnings = [...slot.warnings, ...sync.failed_items]
        if (graph.length) {
          slot.syncHint = t('workspaces.autoBuildOrderStart')
          touch(workspaceId)
          appendLog(workspaceId, `[${stamp()}] ${t('workspaces.autoBuildOrderStart')}`)
          slot.analysis = await fetchMavenOrder(workspaceId)
          slot.warnings = [...slot.warnings, ...(slot.analysis.warnings ?? [])]
          appendLog(workspaceId, `[${stamp()}] ${t('workspaces.autoBuildOrderDone', { count: slot.analysis.full_build_order?.length ?? 0 })}`)
        } else {
          slot.analysis = null
        }
      }
      applyPlan(workspaceId, slot.analysis)
      logPlan(workspaceId)
    } catch (e) {
      applyPlan(workspaceId, slot.analysis)
      appendLog(workspaceId, `[${stamp()}] ${errText(e)}`)
      toast.error(errText(e))
    } finally {
      slot.loadingPlan = false
      slot.syncHint = ''
      slot.planTask = null
      touch(workspaceId)
    }
  })()
  return slot.planTask
}

function stopPoll(id: string): void {
  const slot = autoBuildBag(id)
  if (slot.pollTimer) {
    clearInterval(slot.pollTimer)
    slot.pollTimer = null
  }
  touch(id)
}

async function pullLog(id: string, recId: string): Promise<void> {
  if (!id || !recId) return
  const view = await autoBuildLog(id, recId)
  autoBuildBag(id).logText = view.content
}

async function pollOnce(id: string, showToast: boolean): Promise<boolean> {
  if (!id) return true
  const slot = autoBuildBag(id)
  const st = await autoBuildStatus(id)
  if (st.status === 'running') {
    slot.running = true
    slot.followRecId = st.build_rec_id || slot.followRecId
    if (slot.followRecId) await pullLog(id, slot.followRecId)
    touch(id)
    return false
  }
  stopPoll(id)
  slot.running = false
  if (st.build_rec_id) await pullLog(id, st.build_rec_id)
  else if (slot.followRecId) await pullLog(id, slot.followRecId)
  touch(id)
  void loadHistory(id)
  if (st.status === 'completed') {
    const succeeded = st.succeeded ?? 0
    const failed = st.failed ?? 0
    if (showToast) {
      if (failed === 0) toast.success(t('workspaces.autoBuildSuccess', { count: succeeded }))
      else toast.warning(t('workspaces.autoBuildPartial', { succeeded, failed }))
    }
    if (succeeded > 0) emit('success')
  }
  return true
}

function startPoll(id: string, showToast: boolean): void {
  stopPoll(id)
  void pollOnce(id, showToast)
  const slot = autoBuildBag(id)
  slot.pollTimer = setInterval(() => {
    void pollOnce(id, showToast)
  }, 2000)
  touch(id)
}

async function loadHistory(id: string): Promise<void> {
  if (!id) return
  try {
    autoBuildBag(id).history = await autoBuildHistory(id)
  } catch {
    autoBuildBag(id).history = []
  }
  touch(id)
}

function historyTone(status?: string): 'ok' | 'fail' | 'run' | 'warn' | 'muted' {
  if (status === 'success') return 'ok'
  if (status === 'failed') return 'fail'
  if (status === 'partial') return 'warn'
  if (status === 'running') return 'run'
  return 'muted'
}

function historyTime(row: WorkspaceBuildRecord): string {
  const value = row.started_at || row.finished_at
  if (!value) return ''
  const iso = value.includes('T') ? value : value.replace(' ', 'T')
  const zoned = /(?:[zZ]|[+-]\d{2}:?\d{2})$/.test(iso)
  const d = new Date(zoned ? iso : `${iso}Z`)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function loadHistoryLog(recId: string): Promise<void> {
  const id = wsId.value
  if (!id) return
  const slot = autoBuildBag(id)
  slot.selectedHistory = recId
  try {
    const view = await autoBuildLog(id, recId)
    slot.logText = view.content
    touch(id)
  } catch (e) {
    toast.error(errText(e))
  }
}

async function onOpen(): Promise<void> {
  const id = wsId.value
  if (!id) return
  const slot = autoBuildBag(id)
  void refreshBatch()
  if (slot.running || slot.pollTimer) {
    touch(id)
    if (slot.running && !slot.pollTimer) startPoll(id, false)
    return
  }
  slot.selectedHistory = ''
  slot.logText = ''
  touch(id)
  await refreshBatch()
  await loadProjects(id)
  await loadHistory(id)
  const st = await autoBuildStatus(id)
  if (st.status === 'running') {
    slot.followRecId = st.build_rec_id || ''
    startPoll(id, false)
    return
  }
  if (startBlocked.value) return
  void refreshPlan(id, true)
}

async function submit(): Promise<void> {
  const id = wsId.value
  if (!id || !canSubmit.value) return
  const slot = autoBuildBag(id)
  try {
    const started = await startAutoBuild({
      workspace_rec_id: id,
      scope: slot.scope,
      project_rec_ids: slot.scope === 'selected' ? slot.selectedRecIds : undefined,
      include_non_maven: slot.includeNonMaven,
    })
    slot.followRecId = started.build_rec_id
    slot.running = true
    slot.logText = `[${stamp()}] ${t('workspaces.autoBuildStart')}`
    touch(id)
    startPoll(id, true)
  } catch (e) {
    toast.error(errText(e))
  }
}

watch(open, (v) => {
  if (v) void onOpen()
})
watch(wsId, (id, prev) => {
  if (prev) setAutoBuildBusy(prev, isAutoBuildLive(prev))
  if (id) setAutoBuildBusy(id, isAutoBuildLive(id))
  if (open.value && id && id !== prev) void onOpen()
})
watch([scope, selectedRecIds, includeNonMaven], () => {
  const id = wsId.value
  const slot = autoBuildBag(id)
  if (open.value && id && !slot.running) applyPlan(id, slot.analysis)
})
onUnmounted(() => {
  stopAutoBuildPolls()
})
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="window"
    width="80%"
    height="70%"
    draggable
    resizable
    fullscreenable
    teleport-to="body"
    :title="t('workspaces.autoBuildTitle')"
    :description="t('workspaces.autoBuildHint', { name: workspace?.workspace_name || '', count: workspace?.project_count ?? projects.length })"
  >
    <template #body>
      <RsAlert v-if="occupyTip" type="warning" class="occupy-alert">{{ occupyTip }}</RsAlert>
      <div class="auto-split">
        <div class="form-grid">
          <RsFieldset
            :legend="t('workspaces.autoBuildScope')"
            :tooltip="t('workspaces.autoBuildScopeTip')"
            :disabled="busySelf || startBlocked"
            border-style="dashed"
            title-tone="muted"
            size="sm"
          >
            <RsRadio v-model="scope">
              <RsRadioItem value="workspace">{{ t('workspaces.autoBuildScopeWorkspace') }}</RsRadioItem>
              <RsRadioItem value="selected">{{ t('workspaces.autoBuildScopeSelected') }}</RsRadioItem>
            </RsRadio>
            <RsFormItem v-if="scope === 'selected'" :label="t('workspaces.autoBuildTargets')">
              <RsSelect
                v-model="selectedRecIds"
                multiple
                searchable
                :options="projectOptions"
                :placeholder="t('workspaces.autoBuildTargetsPh')"
              />
            </RsFormItem>
            <RsFormItem v-else :label="t('workspaces.autoBuildNonMaven')" :tooltip="t('workspaces.autoBuildNonMavenTip')">
              <RsCheckbox v-model="includeNonMaven">
                {{ t('workspaces.autoBuildNonMavenLabel') }}
              </RsCheckbox>
            </RsFormItem>
          </RsFieldset>
          <RsFieldset
            :legend="t('workspaces.autoBuildPlanTitle')"
            :tooltip="t('workspaces.autoBuildPlanTip')"
            border-style="dashed"
            title-tone="muted"
            size="sm"
          >
            <template #extra>{{ planExtra }}</template>
            <p class="hint">{{ planCaption }}</p>
            <p v-for="(w, i) in warnings" :key="i" class="hint hint--warn">{{ w }}</p>
            <ol v-if="planRows.length" class="plan">
              <li v-for="row in planRows" :key="row.project.rec_id">
                <div class="plan-row">
                  <span class="plan-name">{{ row.project.project_name }}</span>
                  <RsTag :variant="typeVariant(row.project.project_type)" round>{{ typeLabel(row.project.project_type) }}</RsTag>
                  <RsTag v-if="row.upstream" variant="default" round>{{ t('workspaces.autoBuildPlanUpstream') }}</RsTag>
                  <RsTag v-if="row.cyclic" variant="warning" round>{{ t('workspaces.autoBuildPlanCyclic') }}</RsTag>
                  <span v-if="row.coordinates" class="plan-coord">{{ row.coordinates }}</span>
                </div>
              </li>
            </ol>
            <p v-else class="hint">{{ planEmptyText }}</p>
          </RsFieldset>
          <RsFieldset
            v-if="history.length"
            :legend="t('workspaces.autoBuildSectionHist')"
            :tooltip="t('workspaces.autoBuildSectionHistDesc')"
            border-style="dashed"
            title-tone="muted"
            size="sm"
          >
            <ol class="history">
              <li v-for="item in history" :key="item.rec_id">
                <button
                  type="button"
                  class="history__row"
                  :class="{ 'is-on': selectedHistory === item.rec_id }"
                  @click="loadHistoryLog(item.rec_id)"
                >
                  <span class="history__no" :class="`is-${historyTone(item.status)}`">#{{ item.build_num }}</span>
                  <span class="history__time">{{ historyTime(item) }}</span>
                </button>
              </li>
            </ol>
          </RsFieldset>
        </div>
        <div class="log-col">
          <RsLog
            v-model:lines="logText"
            class="log"
            wrap
            height="100%"
            :empty-text="t('workspaces.autoBuildLogEmpty')"
            :aria-label="t('workspaces.autoBuildTitle')"
          />
        </div>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" :disabled="running || startBlocked" @click="open = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="running || loadingPlan" :disabled="!canSubmit" @click="submit">{{ t('workspaces.autoBuildExecute') }}</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped src="./dialog.css"></style>
<style scoped>
.occupy-alert {
  flex: 0 0 auto;
  margin-bottom: 0.75rem;
}
.auto-split {
  flex: 1 1 0;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 24rem) minmax(0, 1fr);
  gap: 0.85rem;
  overflow: hidden;
  align-items: stretch;
}
.log-col {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.log-col :deep(.rs-log) {
  flex: 1 1 0;
  min-height: 0;
  max-height: 100%;
}
.plan {
  margin: 0;
  padding-left: 1.2rem;
  font-size: 0.82rem;
  line-height: 1.55;
}
.plan-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem 0.45rem;
}
.plan-name {
  font-weight: 650;
}
.plan-coord {
  flex: 1 1 100%;
  font-family: var(--rs-font-mono);
  font-size: 0.72rem;
  color: var(--shell-muted);
}
.hint--warn {
  color: #b45309;
}
.history {
  margin: 0;
  padding: 0;
  list-style: none;
}
.history__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
  padding: 0.35rem 0.15rem;
  border: 0;
  background: transparent;
  cursor: pointer;
  font: inherit;
  color: inherit;
}
.history__row.is-on {
  font-weight: 650;
}
.history__no.is-ok { color: var(--rs-success, #15803d); }
.history__no.is-fail { color: var(--rs-danger, #b91c1c); }
.history__no.is-warn { color: var(--rs-warning, #b45309); }
.history__no.is-run { color: var(--rs-info, #0369a1); }
.history__time {
  color: var(--shell-muted);
  font-size: 0.78rem;
}
@media (max-width: 800px) {
  .auto-split {
    grid-template-columns: 1fr;
    grid-template-rows: auto minmax(16rem, 1fr);
  }
}
</style>
