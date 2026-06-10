<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import { useMessage } from '@/components/feedback/message'
import { fetchProjects } from '@/request/api/project'
import {
  fetchWorkspaceMavenDependencyAnalysis,
} from '@/request/api/workspace'
import { waitWorkspaceSourceSync } from '@/utils/workspace-source-sync'
import type { WorkspaceSourceSyncStatus } from '@/request/api/workspace'
import {
  clearAutoBuildSession,
  createAutoBuildSession,
  fetchServerBootAt,
  isAutoBuildSessionStale,
  loadAutoBuildSession,
  saveAutoBuildSession,
  type AutoBuildScope,
  type AutoBuildSession,
} from '@/utils/auto-build-session'
import type { MavenCyclicGroup, MavenWorkspaceModule } from '@/request/api/project'
import {
  orderProjectsForBuild,
  runOrderedProjectBuilds,
  type OrderedBuildStepState,
} from '@/utils/ordered-project-build'
import type { PackProject, PackBuildStatus } from '@/types'
import { formatBuildTime } from '../project-format'
import { ProjectBuildLogPanel } from './panels'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  workspaceRecId: string
  workspaceName: string
  projectCount: number
}>()

const emit = defineEmits<{
  success: []
  sessionChange: []
}>()

const { t } = useI18n()
const message = useMessage()

const scope = ref<AutoBuildScope>('workspace')
const selectedRecIds = ref<string[]>([])
const includeNonMaven = ref(true)
const loadingPlan = ref(false)
const running = ref(false)
const resuming = ref(false)
const logText = ref('')
const plannedProjects = ref<PackProject[]>([])
const warnings = ref<string[]>([])
const cyclicGroups = ref<MavenCyclicGroup[]>([])
const fullBuildOrder = ref<MavenWorkspaceModule[]>([])
const activeSession = ref<AutoBuildSession | null>(null)
const syncProgress = ref('')
const lastBuildOutcome = ref<{ succeeded: number; failed: number } | null>(null)

let buildLoopPromise: Promise<void> | null = null

const workspaceProjects = ref<PackProject[]>([])

const projectByRecId = computed(
  () => new Map(workspaceProjects.value.map((p) => [p.rec_id, p])),
)

const selectableProjects = computed(() =>
  [...workspaceProjects.value].sort((a, b) => a.project_name.localeCompare(b.project_name)),
)

const selectedModePlanEmpty = computed(
  () => scope.value === 'selected' && selectedRecIds.value.length === 0,
)

const displayLogContent = computed(() => {
  const parts: string[] = []
  if (logText.value) parts.push(logText.value)
  if (syncProgress.value) parts.push(syncProgress.value)
  return parts.join('\n')
})

const logPanelLoading = computed(
  () => (loadingPlan.value || running.value) && !displayLogContent.value,
)

const logPanelComplete = computed(() => !loadingPlan.value && !running.value)

const aggregateBuildStatus = computed((): PackBuildStatus | null => {
  if (loadingPlan.value || running.value) return null
  const failed = activeSession.value?.failed ?? lastBuildOutcome.value?.failed ?? 0
  if (failed > 0) return 'failed'
  if (lastBuildOutcome.value || activeSession.value?.status === 'completed') return 'success'
  return null
})

const buildProgressTitle = computed(() => {
  if (!running.value) return ''
  const done = activeSession.value?.currentIndex ?? 0
  const total = plannedProjects.value.length
  return t('projectBulkAutoBuildRunning', { done, total })
})

const planLoadingTitle = computed(() => {
  if (!loadingPlan.value) return ''
  return syncProgress.value || t('projectBulkAutoBuildPlanLoading')
})

function appendLog(lines: string[]): void {
  const chunk = lines.join('\n')
  logText.value = logText.value ? `${logText.value}\n${chunk}` : chunk
  if (activeSession.value) {
    activeSession.value.logText = logText.value
    saveAutoBuildSession(activeSession.value)
  }
}

function resetFormState(): void {
  scope.value = 'workspace'
  selectedRecIds.value = []
  includeNonMaven.value = true
  logText.value = ''
  plannedProjects.value = []
  warnings.value = []
  cyclicGroups.value = []
  fullBuildOrder.value = []
  running.value = false
  resuming.value = false
  activeSession.value = null
  syncProgress.value = ''
  lastBuildOutcome.value = null
}

function projectsFromPlanRecIds(recIds: string[]): PackProject[] {
  const out: PackProject[] = []
  for (const id of recIds) {
    const p = projectByRecId.value.get(id)
    if (p) out.push(p)
  }
  return out
}

async function loadWorkspaceProjects(): Promise<void> {
  if (!props.workspaceRecId) {
    workspaceProjects.value = []
    return
  }
  const res = await fetchProjects({ workspace_rec_id: props.workspaceRecId })
  workspaceProjects.value = res.items ?? []
}

function formatSyncProgress(st: WorkspaceSourceSyncStatus): string {
  if (st.status !== 'running') return ''
  const done = st.done_count ?? 0
  const total = st.total_count ?? 0
  const project = st.current_project?.trim() || '—'
  return t('projectBulkAutoBuildSyncProgress', { done, total, project })
}

async function preSyncWorkspaceSources(): Promise<void> {
  if (!props.workspaceRecId) return
  syncProgress.value = t('projectBulkAutoBuildSyncStart')
  appendLog([`[${formatBuildTime()}] ${t('projectBulkAutoBuildSyncStart')}`])
  const st = await waitWorkspaceSourceSync(props.workspaceRecId, (progress) => {
    syncProgress.value = formatSyncProgress(progress) || t('projectBulkAutoBuildSyncStart')
  })
  syncProgress.value = ''
  appendLog([
    `[${formatBuildTime()}] ${t('projectBulkAutoBuildSyncDone', {
      synced: st.synced ?? 0,
      skipped: st.skipped ?? 0,
      failed: st.failed ?? 0,
    })}`,
  ])
  if (st.failed_items?.length) {
    warnings.value = [
      ...warnings.value,
      t('projectBulkAutoBuildSyncPartial', {
        synced: st.synced ?? 0,
        failed: st.failed ?? st.failed_items.length,
      }),
      ...st.failed_items.map((line) => `  - ${line}`),
    ]
  }
}

async function refreshBuildPlan(): Promise<void> {
  if (!props.workspaceRecId || running.value) return
  loadingPlan.value = true
  warnings.value = []
  cyclicGroups.value = []
  fullBuildOrder.value = []
  try {
    if (workspaceProjects.value.length === 0) {
      await loadWorkspaceProjects()
    }
    const maven = workspaceProjects.value.filter(
      (p) => String(p.project_type).toLowerCase() === 'maven',
    )
    if (maven.length === 0) {
      plannedProjects.value =
        scope.value === 'workspace' && includeNonMaven.value ? workspaceProjects.value : []
      return
    }
    await preSyncWorkspaceSources()
    const res = await fetchWorkspaceMavenDependencyAnalysis(props.workspaceRecId)
    const analysis = res.analysis
    warnings.value = analysis.warnings ?? []
    cyclicGroups.value = analysis.cyclic_groups ?? []
    fullBuildOrder.value = analysis.full_build_order ?? []
    const targets = scope.value === 'selected' ? selectedRecIds.value : []
    plannedProjects.value = orderProjectsForBuild(
      workspaceProjects.value,
      analysis.full_build_order ?? [],
      {
        targetRecIds: targets,
        edges: analysis.internal_edges,
        includeNonMaven: scope.value === 'workspace' && includeNonMaven.value,
      },
    )
  } catch (e) {
    plannedProjects.value = []
    if (e instanceof Error && e.name === 'WorkspaceSourceSyncForeign') {
      message.error(t('projectBulkAutoBuildSyncForeign', { name: e.message || '—' }))
    } else {
      message.error((e as Error).message)
    }
  } finally {
    loadingPlan.value = false
  }
}

function restoreFromSession(session: AutoBuildSession): void {
  activeSession.value = session
  scope.value = session.scope
  selectedRecIds.value = [...session.selectedRecIds]
  includeNonMaven.value = session.includeNonMaven
  logText.value = session.logText
  plannedProjects.value = projectsFromPlanRecIds(session.planRecIds)
  running.value = session.status === 'running'
  resuming.value = session.status === 'running'
}

function persistStep(index: number, step: OrderedBuildStepState): void {
  const session = activeSession.value
  const project = plannedProjects.value[index]
  if (!session || !project) return
  session.steps[project.rec_id] = step
  session.currentIndex = index + 1
  saveAutoBuildSession(session)
}

async function runBuildLoop(startIndex = 0): Promise<void> {
  if (buildLoopPromise) return buildLoopPromise
  if (plannedProjects.value.length === 0) return

  buildLoopPromise = (async () => {
    running.value = true
    const session = activeSession.value
    const stepStates = session?.steps ?? {}

    try {
      let succeeded = 0
      let failed = 0
      for (const p of plannedProjects.value) {
        const st = stepStates[p.rec_id]
        if (st?.outcome === 'success') succeeded++
        else if (st?.outcome === 'failed') failed++
      }

      if (activeSession.value) {
        activeSession.value.succeeded = succeeded
        activeSession.value.failed = failed
        saveAutoBuildSession(activeSession.value)
      }

      const result = await runOrderedProjectBuilds(
        plannedProjects.value,
        {
          onProjectStart: (project) => {
            appendLog([
              `[${formatBuildTime()}] ${t('projectBulkAutoReleaseBuildProject', { name: project.project_name })}`,
            ])
          },
          onProjectDone: (project, outcome, detail) => {
            if (outcome === 'success') {
              succeeded++
              appendLog([
                `[${formatBuildTime()}] ${t('projectBulkAutoReleaseBuildOk', {
                  name: project.project_name,
                  num: detail.buildNum ?? 0,
                })}`,
              ])
            } else {
              failed++
              appendLog([
                `[${formatBuildTime()}] ${t('projectBulkAutoReleaseBuildFailed', {
                  name: project.project_name,
                  msg: detail.message || t('projectAutoReleaseLogNoMessage'),
                })}`,
              ])
            }
            if (activeSession.value) {
              activeSession.value.succeeded = succeeded
              activeSession.value.failed = failed
              saveAutoBuildSession(activeSession.value)
            }
          },
        },
        {
          startIndex,
          stepStates,
          replayCompleted: false,
          onStepPersist: persistStep,
          cyclicGroups: cyclicGroups.value,
          fullBuildOrder: fullBuildOrder.value,
          onCyclicRetryStart: (pass, maxPasses) => {
            appendLog([
              `[${formatBuildTime()}] ${t('projectBulkAutoBuildCyclicRetry', { pass, maxPasses })}`,
            ])
          },
        },
      )

      appendLog([
        '',
        `[${formatBuildTime()}] ${t('projectBulkAutoBuildDone', {
          succeeded: result.succeeded,
          failed: result.failed,
        })}`,
      ])

      if (activeSession.value) {
        activeSession.value.status = 'completed'
        activeSession.value.succeeded = result.succeeded
        activeSession.value.failed = result.failed
        saveAutoBuildSession(activeSession.value)
        lastBuildOutcome.value = { succeeded: result.succeeded, failed: result.failed }
        clearAutoBuildSession(props.workspaceRecId)
        activeSession.value = null
      }

      if (result.failed === 0) {
        message.success(t('projectBulkAutoBuildSuccess', { count: result.succeeded }))
        emit('success')
      } else {
        message.warning(
          t('projectBulkAutoBuildPartial', {
            succeeded: result.succeeded,
            failed: result.failed,
          }),
        )
      }
      emit('sessionChange')
    } finally {
      running.value = false
      resuming.value = false
      buildLoopPromise = null
    }
  })()

  return buildLoopPromise
}

async function initDialogOnOpen(): Promise<void> {
  await loadWorkspaceProjects()

  const saved = loadAutoBuildSession(props.workspaceRecId)
  if (saved?.status === 'running') {
    if (await isAutoBuildSessionStale(saved)) {
      clearAutoBuildSession(props.workspaceRecId)
      message.warning(t('projectBulkAutoBuildStaleSession'))
      resetFormState()
      void refreshBuildPlan()
      emit('sessionChange')
      return
    }
    restoreFromSession(saved)
    if (plannedProjects.value.length === 0) {
      plannedProjects.value = projectsFromPlanRecIds(saved.planRecIds)
    }
    if (plannedProjects.value.length > 0) {
      const resumeLine = t('projectBulkAutoBuildResume')
      if (!logText.value.includes(resumeLine)) {
        appendLog([`[${formatBuildTime()}] ${resumeLine}`])
      }
      void runBuildLoop(saved.currentIndex)
    } else {
      message.warning(t('projectBulkAutoBuildResumeFailed'))
      clearAutoBuildSession(props.workspaceRecId)
      resetFormState()
      void refreshBuildPlan()
    }
    return
  }

  resetFormState()
  void refreshBuildPlan()
}

watch(open, (v) => {
  if (v) void initDialogOnOpen()
})

watch([scope, selectedRecIds, includeNonMaven], () => {
  if (open.value && !running.value && !activeSession.value) void refreshBuildPlan()
}, { deep: true })

async function onSubmit(): Promise<void> {
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  if (scope.value === 'selected' && selectedRecIds.value.length === 0) {
    message.warning(t('projectBulkAutoBuildSelectRequired'))
    return
  }
  if (plannedProjects.value.length === 0) {
    message.warning(t('projectBulkAutoBuildPlanEmpty'))
    return
  }

  const header = [
    `[${formatBuildTime()}] ${t('projectBulkAutoBuildStart')}`,
    t('projectBulkAutoBuildPlanLine', { count: plannedProjects.value.length }),
    ...plannedProjects.value.map((p, i) => `  ${i + 1}. ${p.project_name}`),
    '',
  ].join('\n')

  const serverBootAt = await fetchServerBootAt()
  activeSession.value = createAutoBuildSession(props.workspaceRecId, serverBootAt, {
    scope: scope.value,
    selectedRecIds: selectedRecIds.value,
    includeNonMaven: includeNonMaven.value,
    planRecIds: plannedProjects.value.map((p) => p.rec_id),
    logText: header,
  })
  logText.value = header
  saveAutoBuildSession(activeSession.value)
  emit('sessionChange')

  await runBuildLoop(0)
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectBulkAutoBuildTitle')"
    width="1000px"
    :show-footer="true"
    :confirm-text="$t('projectBulkAutoBuildExecute')"
    :confirm-loading="running"
    :confirm-disabled="
      running || loadingPlan || plannedProjects.length === 0 || selectedModePlanEmpty
    "
    @confirm="onSubmit"
  >
    <div class="project-bulk-auto-build__layout">
      <div class="project-bulk-auto-build__left">
        <el-alert
          v-if="resuming"
          :title="$t('projectBulkAutoBuildResuming')"
          type="info"
          :closable="false"
          show-icon
          class="project-bulk-auto-build__running-alert"
        />
        <el-alert
          v-else-if="running"
          :title="buildProgressTitle || $t('projectBulkAutoBuildStart')"
          type="warning"
          :closable="false"
          show-icon
          class="project-bulk-auto-build__running-alert"
        />
        <el-alert
          v-else-if="loadingPlan"
          :title="planLoadingTitle"
          type="info"
          :closable="false"
          show-icon
          class="project-bulk-auto-build__running-alert"
        />

        <p class="project-bulk-auto-build__hint">
          {{ $t('projectBulkAutoBuildHint', { name: workspaceName, count: projectCount }) }}
        </p>

        <el-form label-width="120px" class="project-bulk-auto-build__form" @submit.prevent="onSubmit">
          <el-form-item :label="$t('projectBulkAutoBuildScope')">
            <el-radio-group v-model="scope" :disabled="running || loadingPlan">
              <el-radio value="workspace">{{ $t('projectBulkAutoBuildScopeWorkspace') }}</el-radio>
              <el-radio value="selected">{{ $t('projectBulkAutoBuildScopeSelected') }}</el-radio>
            </el-radio-group>
          </el-form-item>

          <el-form-item v-if="scope === 'selected'" :label="$t('projectBulkAutoBuildTargets')">
            <el-select
              v-model="selectedRecIds"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              teleported
              :popper-class="APP_DIALOG_POPPER_CLASS"
              class="project-bulk-auto-build__full"
              :placeholder="$t('projectBulkAutoBuildTargetsPlaceholder')"
              :disabled="running || loadingPlan"
            >
              <el-option
                v-for="p in selectableProjects"
                :key="p.rec_id"
                :label="`${p.project_name} @ ${p.project_version}`"
                :value="p.rec_id"
              />
            </el-select>
            <p v-if="selectedModePlanEmpty" class="project-bulk-auto-build__field-hint">
              {{ $t('projectBulkAutoBuildSelectRequired') }}
            </p>
          </el-form-item>

          <el-form-item v-if="scope === 'workspace'" :label="$t('projectBulkAutoBuildNonMaven')">
            <el-checkbox v-model="includeNonMaven" :disabled="running || loadingPlan">
              {{ $t('projectBulkAutoBuildNonMavenLabel') }}
            </el-checkbox>
          </el-form-item>
        </el-form>

        <el-alert
          v-for="(w, idx) in warnings"
          :key="idx"
          :title="w"
          type="warning"
          :closable="false"
          show-icon
          class="project-bulk-auto-build__alert"
        />

        <section v-loading="loadingPlan" class="project-bulk-auto-build__plan">
          <h4 class="project-bulk-auto-build__plan-title">
            {{ $t('projectBulkAutoBuildPlanTitle') }}
            <span class="project-bulk-auto-build__plan-count">({{ plannedProjects.length }})</span>
          </h4>
          <p class="project-bulk-auto-build__plan-desc">{{ $t('projectBulkAutoBuildPlanDesc') }}</p>
          <ol v-if="plannedProjects.length" class="project-bulk-auto-build__plan-list">
            <li v-for="p in plannedProjects" :key="p.rec_id">
              {{ p.project_name }}
              <span class="project-bulk-auto-build__plan-meta">@ {{ p.project_version }}</span>
            </li>
          </ol>
          <p v-else class="project-bulk-auto-build__plan-empty">{{ $t('projectBulkAutoBuildPlanEmpty') }}</p>
        </section>
      </div>

      <aside class="project-bulk-auto-build__right">
        <ProjectBuildLogPanel
          class="project-bulk-auto-build__log-console"
          :content="displayLogContent"
          :loading="logPanelLoading"
          :complete="logPanelComplete"
          :build-status="aggregateBuildStatus"
          :console-label="$t('projectBulkAutoBuildLog')"
          :empty-description="$t('projectBulkAutoBuildLogEmpty')"
        />
      </aside>
    </div>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-bulk-auto-build {
  &__layout {
    display: flex;
    gap: 16px;
    min-height: 420px;
    align-items: stretch;
  }

  &__left {
    flex: 0 0 420px;
    min-width: 0;
    min-height: 0;
    overflow-y: auto;
    padding-right: 4px;
  }

  &__right {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  &__log-console {
    flex: 1;
    min-height: 0;
    border: none;
    border-radius: 0;
  }

  &__running-alert {
    margin-bottom: 12px;
  }

  &__hint {
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.6;
    color: var(--el-text-color-secondary);
  }

  &__form {
    margin-bottom: 12px;
  }

  &__full {
    width: 100%;
  }

  &__field-hint {
    margin: 6px 0 0;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  &__alert {
    margin-bottom: 12px;
  }

  &__plan {
    margin-bottom: 0;
    padding: 12px 14px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: var(--el-border-radius-base);
    background: var(--el-fill-color-blank);
  }

  &__plan-title {
    margin: 0 0 6px;
    font-size: 14px;
    font-weight: 600;
  }

  &__plan-count {
    font-weight: 400;
    color: var(--el-text-color-secondary);
  }

  &__plan-desc {
    margin: 0 0 10px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  &__plan-list {
    margin: 0;
    padding-left: 1.25rem;
    font-size: 13px;
    line-height: 1.7;
  }

  &__plan-meta {
    color: var(--el-text-color-secondary);
    font-size: 12px;
  }

  &__plan-empty {
    margin: 0;
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }
}
</style>

<style lang="scss">
.app-dialog--windowed.app-dialog--initial-height .app-dialog__body:has(.project-bulk-auto-build__layout) {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.app-dialog--windowed.app-dialog--initial-height .app-dialog__content:has(.project-bulk-auto-build__layout) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.app-dialog--windowed.app-dialog--initial-height
  .app-dialog__content:has(.project-bulk-auto-build__layout)
  .project-bulk-auto-build__layout {
  flex: 1;
  min-height: 0;
}
</style>
