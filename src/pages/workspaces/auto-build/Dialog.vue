<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsAlert, RsButton, RsCheckbox, RsDialog, RsFieldset, RsFormItem, RsLog, RsRadio, RsRadioItem, RsSelect, RsTag, useRsToast } from '@/ui'
import type { Workspace } from '@/stores/workspace'
import { errText, fetchMavenOrder, fetchServerBootAt, listProjects, submitBuild, waitForJob, type MavenAnalysis, type Project } from '../api'
import { typeLabel, typeVariant } from '../composables/use-project-list'
import {
  clearAutoBuildSession,
  createAutoBuildSession,
  hasRunningAutoBuildSession,
  isAutoBuildSessionStale,
  loadAutoBuildSession,
  saveAutoBuildSession,
  type AutoBuildScope,
  type AutoBuildSession,
} from './session'
import { setAutoBuildBusy, useBatchActivity } from '../composables/use-batch-activity'
import { cyclicRecIds, isOrderedBuildType, orderProjectsForBuild } from '../lib/ordered-build'
import { syncSourcesWithLog } from '../lib/sync-log'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ workspace?: Workspace | null }>()
const emit = defineEmits<{ success: []; sessionChange: [] }>()

const { t } = useI18n()
const toast = useRsToast()
const scope = ref<AutoBuildScope>('workspace')
const selectedRecIds = ref<string[]>([])
const includeNonMaven = ref(true)
const loadingPlan = ref(false)
const running = ref(false)
const logText = ref('')
const planned = ref<Project[]>([])
const warnings = ref<string[]>([])
const projects = ref<Project[]>([])
const analysis = ref<MavenAnalysis | null>(null)
const active = ref<AutoBuildSession | null>(null)
const syncHint = ref('')
const syncWaiting = ref(false)
let loop: Promise<void> | null = null
let planTask: Promise<void> | null = null

const enabledProjects = computed(() => projects.value.filter((p) => p.is_enabled !== 0))
const selectable = computed(() => [...enabledProjects.value].sort((a, b) => a.project_name.localeCompare(b.project_name)))
const projectOptions = computed(() => selectable.value.map((p) => ({ value: p.rec_id, label: p.project_name })))
const { releaseBusy, refresh: refreshBatch } = useBatchActivity(() => props.workspace?.rec_id || '')
const sessionRunning = computed(() => active.value?.status === 'running' || hasRunningAutoBuildSession(props.workspace?.rec_id || ''))
const occupyTip = computed(() => {
  if (releaseBusy.value && !running.value) return t('workspaces.autoBatchBusyRelease')
  if (running.value || sessionRunning.value) return t('workspaces.autoBatchBusyBuild')
  if (syncWaiting.value) return t('workspaces.autoBuildSyncWait')
  return ''
})
const startBlocked = computed(() => releaseBusy.value && !running.value)
const busySelf = computed(() => running.value || loadingPlan.value || sessionRunning.value)
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

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function appendLog(line: string): void {
  logText.value = logText.value ? `${logText.value}\n${line}` : line
  if (active.value) {
    active.value.logText = logText.value
    saveAutoBuildSession(active.value)
  }
}

async function syncSourcesOrWait(workspaceId: string): Promise<Awaited<ReturnType<typeof syncSourcesWithLog>>> {
  let noted = false
  for (;;) {
    try {
      const result = await syncSourcesWithLog(workspaceId, t, appendLog, (hint) => {
        syncHint.value = hint
      })
      syncWaiting.value = false
      return result
    } catch (e) {
      if (!isSyncInProgress(e) || props.workspace?.rec_id !== workspaceId) {
        syncWaiting.value = false
        throw e
      }
      syncWaiting.value = true
      if (!noted) {
        appendLog(`[${stamp()}] ${t('workspaces.autoBuildSyncWait')}`)
        noted = true
      }
      syncHint.value = t('workspaces.autoBuildSyncWait')
      await sleep(2000)
    }
  }
}

async function loadProjects(): Promise<void> {
  const id = props.workspace?.rec_id
  projects.value = id ? await listProjects(id) : []
}

function applyPlan(src: MavenAnalysis | null): void {
  planned.value = orderProjectsForBuild(enabledProjects.value, src?.full_build_order ?? [], {
    targetRecIds: scope.value === 'selected' ? selectedRecIds.value : [],
    edges: src?.internal_edges,
    includeNonMaven: scope.value === 'workspace' && includeNonMaven.value,
  })
}

function logPlan(): void {
  if (!planned.value.length) return
  appendLog(`[${stamp()}] ${t('workspaces.autoBuildPlanLine', { count: planned.value.length })}`)
  planned.value.forEach((p, i) => {
    const coord = coordById.value.get(p.rec_id)
    const kind = typeLabel(p.project_type)
    appendLog(`  ${i + 1}. ${p.project_name}  [${kind}]${coord ? `  ${coord}` : ''}`)
  })
}

async function refreshPlan(resync: boolean): Promise<void> {
  if (!props.workspace?.rec_id || running.value || startBlocked.value) return
  if (planTask) return planTask
  const workspaceId = props.workspace.rec_id
  loadingPlan.value = true
  planTask = (async () => {
    try {
      if (!projects.value.length) await loadProjects()
      applyPlan(analysis.value)
      const graph = enabledProjects.value.filter((p) => isOrderedBuildType(p.project_type))
      if (resync || !analysis.value) {
        warnings.value = []
        const sync = await syncSourcesOrWait(workspaceId)
        if (props.workspace?.rec_id !== workspaceId) return
        appendLog(`[${stamp()}] ${t('workspaces.autoBuildSyncDone', { synced: sync.synced, skipped: sync.skipped, failed: sync.failed })}`)
        if (sync.failed_items?.length) warnings.value = [...warnings.value, ...sync.failed_items]
        if (graph.length) {
          syncHint.value = t('workspaces.autoBuildOrderStart')
          appendLog(`[${stamp()}] ${t('workspaces.autoBuildOrderStart')}`)
          analysis.value = await fetchMavenOrder(workspaceId)
          warnings.value = [...warnings.value, ...(analysis.value.warnings ?? [])]
          appendLog(`[${stamp()}] ${t('workspaces.autoBuildOrderDone', { count: analysis.value.full_build_order?.length ?? 0 })}`)
        } else {
          analysis.value = null
        }
      }
      applyPlan(analysis.value)
      logPlan()
    } catch (e) {
      applyPlan(analysis.value)
      appendLog(`[${stamp()}] ${errText(e)}`)
      toast.error(errText(e))
    } finally {
      loadingPlan.value = false
      syncHint.value = ''
      syncWaiting.value = false
      planTask = null
    }
  })()
  return planTask
}

async function runLoop(startIndex: number, analysisCycles: Set<string>): Promise<void> {
  if (loop) return loop
  loop = (async () => {
    running.value = true
    const session = active.value
    let succeeded = session?.succeeded ?? 0
    let failed = session?.failed ?? 0
    try {
      for (let i = startIndex; i < planned.value.length; i++) {
        const p = planned.value[i]
        const prev = session?.steps[p.rec_id]
        if (prev?.outcome === 'success') continue
        appendLog(`[${stamp()}] ${t('workspaces.autoReleaseBuildProject', { name: p.project_name, num: i + 1, total: planned.value.length })}`)
        try {
          const job = await submitBuild(p.workspace_rec_id || props.workspace?.rec_id || '', p.rec_id)
          let lastStep = ''
          const done = await waitForJob(job.rec_id, 2000, (view) => {
            const runningStep = (view.steps ?? []).find((s) => s.step_status === 'running')
            const label = runningStep?.step_name || runningStep?.step_key || ''
            if (label && label !== lastStep) {
              lastStep = label
              appendLog(`[${stamp()}] ${t('workspaces.autoBuildJobStep', { name: p.project_name, step: label })}`)
            }
          })
          if (done.job_status === 'succeeded') {
            succeeded++
            appendLog(`[${stamp()}] ${t('workspaces.autoReleaseBuildOk', { name: p.project_name, num: i + 1 })}`)
            if (session) session.steps[p.rec_id] = { buildRecId: job.rec_id, outcome: 'success' }
          } else {
            failed++
            appendLog(`[${stamp()}] ${t('workspaces.autoReleaseBuildFailed', { name: p.project_name, msg: done.error_text || done.job_status })}`)
            if (session) session.steps[p.rec_id] = { buildRecId: job.rec_id, outcome: 'failed', message: done.error_text }
          }
        } catch (e) {
          failed++
          appendLog(`[${stamp()}] ${t('workspaces.autoReleaseBuildFailed', { name: p.project_name, msg: errText(e) })}`)
          if (session) session.steps[p.rec_id] = { outcome: 'failed', message: errText(e) }
        }
        if (session) {
          session.currentIndex = i + 1
          session.succeeded = succeeded
          session.failed = failed
          saveAutoBuildSession(session)
        }
      }
      if (failed > 0 && analysisCycles.size) {
        const retry = planned.value.filter((p) => analysisCycles.has(p.rec_id) && session?.steps[p.rec_id]?.outcome === 'failed')
        if (retry.length) {
          appendLog(`[${stamp()}] ${t('workspaces.autoBuildCyclicRetry', { pass: 2, maxPasses: 2 })}`)
          for (const p of retry) {
            try {
              const job = await submitBuild(p.workspace_rec_id || props.workspace?.rec_id || '', p.rec_id)
              const done = await waitForJob(job.rec_id)
              if (done.job_status === 'succeeded') {
                failed--
                succeeded++
                appendLog(`[${stamp()}] ${t('workspaces.autoReleaseBuildOk', { name: p.project_name, num: 0 })}`)
              }
            } catch (e) {
              appendLog(`[${stamp()}] ${t('workspaces.autoReleaseBuildFailed', { name: p.project_name, msg: errText(e) })}`)
            }
          }
        }
      }
      if (session) {
        session.status = 'completed'
        session.succeeded = succeeded
        session.failed = failed
        saveAutoBuildSession(session)
      }
      appendLog(`[${stamp()}] ${t('workspaces.autoBuildDone', { succeeded, failed })}`)
      if (failed === 0) {
        toast.success(t('workspaces.autoBuildSuccess', { count: succeeded }))
        emit('success')
      } else {
        toast.warning(t('workspaces.autoBuildPartial', { succeeded, failed }))
      }
      emit('sessionChange')
    } finally {
      running.value = false
      loop = null
    }
  })()
  return loop
}

async function onOpen(): Promise<void> {
  if (running.value || loadingPlan.value || planTask) return
  const id = props.workspace?.rec_id || ''
  const saved = id ? loadAutoBuildSession(id) : null
  if (saved?.status === 'running') {
    logText.value = saved.logText
    active.value = saved
    scope.value = saved.scope
    selectedRecIds.value = [...saved.selectedRecIds]
    includeNonMaven.value = saved.includeNonMaven
    running.value = true
    await refreshBatch()
    await loadProjects()
    const boot = await fetchServerBootAt()
    if (await isAutoBuildSessionStale(saved, boot)) {
      running.value = false
      active.value = null
      clearAutoBuildSession(id)
      toast.warning(t('workspaces.autoBuildStaleSession'))
      if (!startBlocked.value) void refreshPlan(true)
      emit('sessionChange')
      return
    }
    planned.value = saved.planRecIds.map((rid) => projects.value.find((p) => p.rec_id === rid)).filter((p): p is Project => !!p)
    appendLog(`[${stamp()}] ${t('workspaces.autoBuildResume')}`)
    void runLoop(saved.currentIndex, analysis.value ? cyclicRecIds(analysis.value) : new Set())
    return
  }
  logText.value = ''
  planned.value = []
  warnings.value = []
  await refreshBatch()
  await loadProjects()
  active.value = null
  if (startBlocked.value) return
  void refreshPlan(true)
}

async function submit(): Promise<void> {
  if (!props.workspace?.rec_id || !canSubmit.value) return
  const header = [`[${stamp()}] ${t('workspaces.autoBuildStart')}`, t('workspaces.autoBuildPlanLine', { count: planned.value.length }), ...planned.value.map((p, i) => `  ${i + 1}. ${p.project_name}`)].join('\n')
  const boot = await fetchServerBootAt()
  active.value = createAutoBuildSession(props.workspace.rec_id, boot, {
    scope: scope.value,
    selectedRecIds: selectedRecIds.value,
    includeNonMaven: includeNonMaven.value,
    planRecIds: planned.value.map((p) => p.rec_id),
    logText: header,
  })
  logText.value = header
  saveAutoBuildSession(active.value)
  emit('sessionChange')
  await runLoop(0, analysis.value ? cyclicRecIds(analysis.value) : new Set())
}

watch(open, (v) => {
  if (v) void onOpen()
})
watch([scope, selectedRecIds, includeNonMaven], () => {
  if (open.value && !running.value && !active.value) applyPlan(analysis.value)
})
watch(
  [running, loadingPlan, () => props.workspace?.rec_id],
  ([run, load, id], prev) => {
    const prevId = String(prev?.[2] ?? '')
    if (prevId && prevId !== id) setAutoBuildBusy(prevId, false)
    if (!id) return
    setAutoBuildBusy(id, Boolean(run || load || hasRunningAutoBuildSession(id)))
  },
)
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
@media (max-width: 800px) {
  .auto-split {
    grid-template-columns: 1fr;
    grid-template-rows: auto minmax(16rem, 1fr);
  }
}
</style>
