<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import {
  batchBranchClone,
  batchBranchCloneStatus,
  fetchWorkspaceBranchCloneHistory,
  fetchWorkspaceBranchCloneLog,
  type BatchBranchCloneResultItem,
  type BatchBranchCloneStatusResponse,
  type WorkspaceBranchCloneRecord,
} from '@/request/api/project'
import { ProjectReleaseLogPanel } from './panels'
import { formatBuildTime } from '../project-format'
import { releaseStatusTagType } from '../release-status'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  workspaceRecId: string
  workspaceName: string
  projectCount: number
}>()

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()
const message = useMessage()

const sourceBranch = ref('release/V10.0.0-P01')
const targetBranch = ref('release/V10.0.0-P02')
const saving = ref(false)
const running = ref(false)
const foreignJob = ref(false)
const progressText = ref('')
const logText = ref('')
const logHeader = ref('')
const activeBatchRecId = ref<string | null>(null)
const batchHistory = ref<WorkspaceBranchCloneRecord[]>([])
const historyLoading = ref(false)
const logViewMode = ref<'live' | 'history'>('live')
const selectedBatchId = ref<string | null>(null)
const historyLogContent = ref('')
const historyLogLoading = ref(false)
const historyLogComplete = ref(true)
const doneCount = ref(0)
const totalCount = ref(0)

let pollTimer: ReturnType<typeof setInterval> | null = null
const pollIntervalMs = 2000

const displayLogContent = computed(() =>
  logViewMode.value === 'history' ? historyLogContent.value : logText.value,
)

const progressPercent = computed(() => {
  if (totalCount.value <= 0) return 0
  return Math.min(100, Math.round((doneCount.value / totalCount.value) * 100))
})

const selectedBatch = computed(() =>
  batchHistory.value.find((r) => r.rec_id === selectedBatchId.value) ?? null,
)

const displayBatchNum = computed(() => {
  if (logViewMode.value === 'history') return selectedBatch.value?.batch_num ?? null
  const id = activeBatchRecId.value
  if (!id) return null
  return batchHistory.value.find((r) => r.rec_id === id)?.batch_num ?? null
})

const displayBatchStatus = computed(() => {
  if (logViewMode.value === 'history') return selectedBatch.value?.status ?? null
  if (running.value) return 'running'
  if (activeBatchRecId.value) {
    return batchHistory.value.find((r) => r.rec_id === activeBatchRecId.value)?.status ?? null
  }
  return null
})

const logPanelComplete = computed(() => {
  if (logViewMode.value === 'history') return historyLogComplete.value
  return !running.value
})

const logPanelLoading = computed(() => {
  if (logViewMode.value === 'history') return historyLogLoading.value
  return running.value && !logText.value
})

const showViewLive = computed(
  () => logViewMode.value === 'history' && (running.value || activeBatchRecId.value != null),
)

function formatLogTime(): string {
  return formatBuildTime(new Date().toISOString())
}

function statusLabel(status: string): string {
  switch (status) {
    case 'ok':
      return t('projectBulkBranchCloneStatusOk')
    case 'skipped':
      return t('projectBulkBranchCloneStatusSkipped')
    default:
      return t('projectBulkBranchCloneStatusFailed')
  }
}

function formatProjectResultLines(item: BatchBranchCloneResultItem): string[] {
  const lines: string[] = [`[${statusLabel(item.status)}] ${item.project_name}`]
  const steps = item.steps ?? []
  if (steps.length > 0) {
    for (const step of steps) {
      lines.push(`  · ${step}`)
    }
  }
  const msg = item.message?.trim()
  if (msg) {
    lines.push(`  → ${msg}`)
  }
  return lines
}

function syncLogFromStatus(st: BatchBranchCloneStatusResponse, final: boolean): void {
  if (st.foreign) return
  const body: string[] = []
  for (const item of st.results ?? []) {
    body.push(...formatProjectResultLines(item))
    body.push('')
  }
  if (st.status === 'running' && st.current_project?.trim()) {
    body.push(`[…] ${t('projectBulkBranchCloneRunningDetail', {
      done: st.done_count ?? 0,
      total: st.total_count ?? 0,
      project: st.current_project.trim(),
    })}`)
  }
  const progress = body.join('\n').trimEnd()
  if (logViewMode.value === 'live') {
    logText.value = logHeader.value
      ? progress
        ? `${logHeader.value}\n\n${progress}`
        : logHeader.value
      : progress
  }
  if (!final) {
    void refreshLiveLog(st.batch_rec_id)
  }
}

async function loadBatchHistory(): Promise<void> {
  if (!props.workspaceRecId) return
  historyLoading.value = true
  try {
    const res = await fetchWorkspaceBranchCloneHistory(props.workspaceRecId)
    batchHistory.value = res.items ?? []
  } catch {
    batchHistory.value = []
  } finally {
    historyLoading.value = false
  }
}

async function loadLogDrain(
  loader: (offset: number) => Promise<{ content: string; offset: number; size: number; total: number }>,
): Promise<string> {
  let content = ''
  let offset = 0
  for (;;) {
    const res = await loader(offset)
    if (res.content) content += res.content
    offset = res.offset + res.size
    if (offset >= res.total) break
  }
  return content
}

async function loadHistoryLog(batchId: string): Promise<void> {
  if (!props.workspaceRecId) return
  historyLogLoading.value = true
  try {
    const content = await loadLogDrain((offset) =>
      fetchWorkspaceBranchCloneLog(props.workspaceRecId, batchId, offset).then((res) => {
        historyLogComplete.value = res.complete
        const idx = batchHistory.value.findIndex((r) => r.rec_id === res.batch.rec_id)
        if (idx >= 0) batchHistory.value[idx] = res.batch
        return res
      }),
    )
    historyLogContent.value = content
  } catch (e) {
    message.error((e as Error).message)
    historyLogContent.value = ''
  } finally {
    historyLogLoading.value = false
  }
}

async function refreshLiveLog(batchId?: string): Promise<void> {
  const id = batchId ?? activeBatchRecId.value
  if (!props.workspaceRecId || !id || logViewMode.value !== 'live') return
  try {
    const content = await loadLogDrain((offset) =>
      fetchWorkspaceBranchCloneLog(props.workspaceRecId, id, offset).then((res) => {
        const idx = batchHistory.value.findIndex((r) => r.rec_id === res.batch.rec_id)
        if (idx >= 0) batchHistory.value[idx] = res.batch
        return res
      }),
    )
    if (content) logText.value = content
  } catch {
    /* ignore transient log read errors while polling */
  }
}

function viewLiveLog(): void {
  logViewMode.value = 'live'
  selectedBatchId.value = null
}

async function selectBatchRecord(batchId: string): Promise<void> {
  selectedBatchId.value = batchId
  logViewMode.value = 'history'
  historyLogContent.value = ''
  await loadHistoryLog(batchId)
}

function updateProgress(st: BatchBranchCloneStatusResponse): void {
  if (st.foreign) {
    foreignJob.value = true
    running.value = true
    doneCount.value = st.done_count ?? 0
    totalCount.value = st.total_count ?? 0
    progressText.value = t('projectBulkBranchCloneRunningForeign', {
      workspace: st.active_workspace_name ?? '—',
    })
    return
  }
  foreignJob.value = false
  doneCount.value = st.done_count ?? 0
  totalCount.value = st.total_count ?? 0
  if (st.status !== 'running') {
    running.value = false
    progressText.value = ''
    return
  }
  running.value = true
  const done = st.done_count ?? 0
  const total = st.total_count ?? 0
  const project = st.current_project?.trim()
  progressText.value = project
    ? t('projectBulkBranchCloneRunningDetail', { done, total, project })
    : t('projectBulkBranchCloneRunning', { done, total })
}

function stopPolling(): void {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function buildResumeLogHeader(st: BatchBranchCloneStatusResponse): string {
  const rec = st.batch_rec_id
    ? batchHistory.value.find((r) => r.rec_id === st.batch_rec_id)
    : undefined
  const source = st.source_branch?.trim() || rec?.source_branch?.trim() || sourceBranch.value
  const target = st.target_branch?.trim() || rec?.target_branch?.trim() || targetBranch.value
  return [
    `[${formatLogTime()}] ${t('projectBulkBranchCloneLogResume')}`,
    `${t('projectBulkBranchCloneSource')}: ${source} → ${t('projectBulkBranchCloneTarget')}: ${target}`,
  ].join('\n')
}

function restoreLiveSession(st: BatchBranchCloneStatusResponse, opts?: { startPoll?: boolean }): void {
  if (st.foreign) {
    if (st.status === 'running' && opts?.startPoll !== false) {
      startPolling(false)
    }
    return
  }
  if (st.source_branch?.trim()) sourceBranch.value = st.source_branch.trim()
  if (st.target_branch?.trim()) targetBranch.value = st.target_branch.trim()
  if (st.batch_rec_id) activeBatchRecId.value = st.batch_rec_id
  logHeader.value = buildResumeLogHeader(st)
  logViewMode.value = 'live'
  selectedBatchId.value = null
  syncLogFromStatus(st, st.status === 'completed')
  if (st.status === 'running' && opts?.startPoll !== false) {
    startPolling(false)
  }
}

async function pollOnce(showToast: boolean): Promise<boolean> {
  if (!props.workspaceRecId) return true
  const st = await batchBranchCloneStatus(props.workspaceRecId)
  updateProgress(st)
  if (!st.foreign) {
    if (st.batch_rec_id) activeBatchRecId.value = st.batch_rec_id
    if (logViewMode.value === 'live') {
      syncLogFromStatus(st, st.status === 'completed')
    }
  }
  if (st.status === 'running') {
    return false
  }
  stopPolling()
  running.value = false
  foreignJob.value = false
  progressText.value = ''
  void loadBatchHistory()
  if (!st.foreign) {
    const cloned = st.cloned ?? 0
    if ((st.failed ?? 0) > 0) {
      if (showToast) {
        message.warning(
          t('projectBulkBranchClonePartial', {
            cloned,
            skipped: st.skipped ?? 0,
            failed: st.failed ?? 0,
          }),
        )
      }
    } else if (showToast) {
      message.success(
        t('projectBulkBranchCloneSuccess', {
          cloned,
          skipped: st.skipped ?? 0,
        }),
      )
    }
    if (cloned > 0) emit('success')
  }
  return true
}

function startPolling(showToast: boolean): void {
  stopPolling()
  void pollOnce(showToast)
  pollTimer = setInterval(() => {
    void pollOnce(showToast)
  }, pollIntervalMs)
}

function resetDialogState(): void {
  sourceBranch.value = 'release/V10.0.0-P01'
  targetBranch.value = 'release/V10.0.0-P02'
  saving.value = false
  running.value = false
  foreignJob.value = false
  progressText.value = ''
  logText.value = ''
  logHeader.value = ''
  activeBatchRecId.value = null
  doneCount.value = 0
  totalCount.value = 0
  logViewMode.value = 'live'
  selectedBatchId.value = null
  historyLogContent.value = ''
}

async function initDialogOnOpen(): Promise<void> {
  resetDialogState()
  if (!props.workspaceRecId) return

  await loadBatchHistory()

  let st: BatchBranchCloneStatusResponse
  try {
    st = await batchBranchCloneStatus(props.workspaceRecId)
  } catch {
    return
  }

  updateProgress(st)

  if (!st.foreign && st.status === 'running') {
    restoreLiveSession(st, { startPoll: true })
    return
  }

  if (st.foreign && st.status === 'running') {
    startPolling(false)
  }
}

watch(open, (v) => {
  if (v) {
    void initDialogOnOpen()
  } else {
    stopPolling()
  }
})

onUnmounted(() => {
  stopPolling()
})

async function onSubmit(): Promise<void> {
  const source = sourceBranch.value.trim()
  const target = targetBranch.value.trim()
  if (!source) {
    message.warning(t('projectBulkBranchCloneSourceRequired'))
    return
  }
  if (!target) {
    message.warning(t('projectBulkBranchCloneTargetRequired'))
    return
  }
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  if (running.value || foreignJob.value) {
    message.warning(t('projectBulkBranchCloneAlreadyRunning'))
    return
  }

  logHeader.value = [
    `[${formatLogTime()}] ${t('projectBulkBranchCloneLogStart')}`,
    `${t('projectBulkBranchCloneSource')}: ${source} → ${t('projectBulkBranchCloneTarget')}: ${target}`,
  ].join('\n')
  logText.value = logHeader.value
  logViewMode.value = 'live'
  selectedBatchId.value = null

  saving.value = true
  running.value = true
  try {
    const start = await batchBranchClone({
      workspace_rec_id: props.workspaceRecId,
      source_branch: source,
      target_branch: target,
    })
    activeBatchRecId.value = start.batch_rec_id
    void loadBatchHistory()
    startPolling(true)
  } catch (e) {
    running.value = false
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectBulkBranchCloneTitle')"
    width="960px"
    initial-height="520px"
    :show-footer="true"
    :confirm-loading="saving"
    :confirm-disabled="running || foreignJob"
    @confirm="onSubmit"
  >
    <div class="project-bulk-branch-clone__layout">
      <div class="project-bulk-branch-clone__form-panel">
        <p class="project-bulk-branch-clone__hint">
          {{ $t('projectBulkBranchCloneHint', { name: workspaceName, count: projectCount }) }}
        </p>
        <el-form label-width="88px" @submit.prevent="onSubmit">
          <el-form-item :label="$t('projectBulkBranchCloneSource')" required>
            <el-input
              v-model="sourceBranch"
              :placeholder="$t('projectBranchPlaceholder')"
              :disabled="running || foreignJob"
              autocomplete="off"
            />
          </el-form-item>
          <el-form-item :label="$t('projectBulkBranchCloneTarget')" required>
            <el-input
              v-model="targetBranch"
              :placeholder="$t('projectBranchPlaceholder')"
              :disabled="running || foreignJob"
              autocomplete="off"
            />
          </el-form-item>
          <p class="project-bulk-branch-clone__note">{{ $t('projectBulkBranchCloneNote') }}</p>
        </el-form>
      </div>

      <aside class="project-bulk-branch-clone__right-panel">
        <section class="project-bulk-branch-clone__progress">
          <header class="project-bulk-branch-clone__progress-head">
            <span>{{ $t('projectBulkBranchCloneProgress') }}</span>
            <span v-if="running || foreignJob" class="project-bulk-branch-clone__progress-meta">
              {{ doneCount }} / {{ totalCount || '—' }}
            </span>
          </header>
          <el-alert
            v-if="foreignJob"
            :title="progressText"
            type="warning"
            :closable="false"
            show-icon
            class="project-bulk-branch-clone__alert"
          />
          <template v-else-if="running">
            <el-progress
              :percentage="progressPercent"
              :stroke-width="10"
              status="success"
              striped
              striped-flow
            />
            <p v-if="progressText" class="project-bulk-branch-clone__progress-text">{{ progressText }}</p>
          </template>
          <p v-else class="project-bulk-branch-clone__progress-idle">—</p>
        </section>

        <section class="project-bulk-branch-clone__history">
          <header class="project-bulk-branch-clone__history-head">
            <span>{{ $t('projectBulkBranchCloneHistory') }}</span>
            <el-button
              v-if="showViewLive"
              link
              type="primary"
              size="small"
              @click="viewLiveLog"
            >
              {{ $t('projectBulkBranchCloneViewLive') }}
            </el-button>
          </header>
          <ul v-loading="historyLoading" class="project-bulk-branch-clone__history-list">
            <li v-for="r in batchHistory" :key="r.rec_id">
              <button
                type="button"
                class="project-bulk-branch-clone__history-item"
                :class="{
                  'is-active': logViewMode === 'history' && selectedBatchId === r.rec_id,
                  'is-live': logViewMode === 'live' && activeBatchRecId === r.rec_id,
                }"
                @click="selectBatchRecord(r.rec_id)"
              >
                <el-tag :type="releaseStatusTagType(r.status)" size="small" effect="plain">
                  #{{ r.batch_num }}
                </el-tag>
                <span class="project-bulk-branch-clone__history-branches">
                  {{ r.source_branch }} → {{ r.target_branch }}
                </span>
              </button>
            </li>
          </ul>
          <p
            v-if="!historyLoading && !batchHistory.length"
            class="project-bulk-branch-clone__history-empty"
          >
            {{ $t('projectBulkBranchCloneHistoryEmpty') }}
          </p>
        </section>

        <ProjectReleaseLogPanel
          class="project-bulk-branch-clone__log-console"
          :release-num="displayBatchNum"
          :release-status="displayBatchStatus"
          :content="displayLogContent"
          :loading="logPanelLoading"
          :complete="logPanelComplete"
        />
      </aside>
    </div>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-bulk-branch-clone {
  &__layout {
    display: flex;
    gap: 16px;
    min-height: 360px;
    height: 100%;
    align-items: stretch;
  }

  &__form-panel {
    flex: 0 0 300px;
    min-width: 0;
  }

  &__right-panel {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  &__hint {
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__note {
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
  }

  &__alert {
    margin-top: 8px;
  }

  &__progress {
    flex: 0 0 auto;
    padding: 10px 12px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
    background: var(--el-fill-color-blank);
  }

  &__progress-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 12px;
    font-weight: 600;
    margin-bottom: 8px;
  }

  &__progress-meta {
    font-weight: 400;
    color: var(--el-text-color-secondary);
  }

  &__progress-text {
    margin: 8px 0 0;
    font-size: 12px;
    color: var(--el-text-color-regular);
  }

  &__progress-idle {
    margin: 4px 0 0;
    font-size: 12px;
    color: var(--el-text-color-placeholder);
  }

  &__history {
    flex: 0 0 auto;
    max-height: 100px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
    background: var(--el-fill-color-blank);
  }

  &__history-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    font-size: 12px;
    font-weight: 600;
    border-bottom: 1px solid var(--el-border-color-lighter);
  }

  &__history-list {
    list-style: none;
    margin: 0;
    padding: 4px;
    overflow-y: auto;
    flex: 1;
  }

  &__history-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 6px 8px;
    border: none;
    border-radius: 4px;
    background: transparent;
    cursor: pointer;
    text-align: left;
    font-size: 12px;
    color: var(--el-text-color-regular);

    &:hover {
      background: var(--el-fill-color-light);
    }

    &.is-active,
    &.is-live {
      background: var(--el-color-primary-light-9);
    }
  }

  &__history-branches {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__history-empty {
    margin: 0;
    padding: 8px 10px;
    font-size: 12px;
    color: var(--el-text-color-placeholder);
  }

  &__log-console {
    flex: 1;
    min-height: 180px;
  }
}
</style>
