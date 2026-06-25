<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import {
  batchUpdatePomVersion,
  batchPomVersionStatus,
  fetchWorkspacePomVersionHistory,
  fetchWorkspacePomVersionLog,
  type BatchPomVersionResultItem,
  type BatchPomVersionStatusResponse,
  type WorkspacePomVersionRecord,
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

const versionTag = ref('V10.0.0-P01')
const commitMessage = ref('')
const saving = ref(false)
const running = ref(false)
const foreignJob = ref(false)
const progressText = ref('')
const logText = ref('')
const logHeader = ref('')
const activeBatchRecId = ref<string | null>(null)
const batchHistory = ref<WorkspacePomVersionRecord[]>([])
const historyLoading = ref(false)
const logViewMode = ref<'live' | 'history'>('live')
const selectedBatchId = ref<string | null>(null)
const historyLogContent = ref('')
const historyLogLoading = ref(false)
const historyLogComplete = ref(true)

let pollTimer: ReturnType<typeof setInterval> | null = null
const pollIntervalMs = 2000

const displayLogContent = computed(() =>
  logViewMode.value === 'history' ? historyLogContent.value : logText.value,
)

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
      return t('projectBulkPomVersionStatusOk')
    case 'skipped':
      return t('projectBulkPomVersionStatusSkipped')
    default:
      return t('projectBulkPomVersionStatusFailed')
  }
}

function formatProjectResultLines(item: BatchPomVersionResultItem): string[] {
  const lines: string[] = [`[${statusLabel(item.status)}] ${item.project_name}`]
  const steps = item.steps ?? []
  if (steps.length > 0) {
    for (const step of steps) {
      lines.push(`  · ${step}`)
    }
  } else if (item.status !== 'skipped') {
    lines.push(`  · ${t('projectBulkPomVersionLogNoSteps')}`)
  }
  const msg = item.message?.trim()
  if (msg) {
    lines.push(`  → ${msg}`)
  } else if (item.status === 'failed') {
    lines.push(`  → ${t('projectBulkPomVersionLogNoMessage')}`)
  }
  return lines
}

function syncLogFromStatus(st: BatchPomVersionStatusResponse, tag: string, final: boolean): void {
  if (st.foreign) return
  const body: string[] = []
  if (final) {
    body.push(`[${formatLogTime()}] ${t('projectBulkPomVersionLogDone')}`)
    body.push(
      t('projectBulkPomVersionLogSummary', {
        updated: st.updated ?? 0,
        skipped: st.skipped ?? 0,
        failed: st.failed ?? 0,
        tag,
      }),
    )
    body.push('')
  } else if (st.status === 'running') {
    body.push(`[${formatLogTime()}] ${t('projectBulkPomVersionLogProgress')}`)
    body.push('')
  }
  for (const item of st.results ?? []) {
    body.push(...formatProjectResultLines(item))
    body.push('')
  }
  if (st.status === 'running' && st.current_project?.trim()) {
    body.push(
      `[…] ${t('projectBulkPomVersionLogInProgress', { project: st.current_project.trim() })}`,
    )
  }
  const progress = body.join('\n').trimEnd()
  if (logViewMode.value === 'live') {
    logText.value = logHeader.value
      ? progress
        ? `${logHeader.value}\n\n${progress}`
        : logHeader.value
      : progress
  }
}

async function loadBatchHistory(): Promise<void> {
  if (!props.workspaceRecId) return
  historyLoading.value = true
  try {
    const res = await fetchWorkspacePomVersionHistory(props.workspaceRecId)
    batchHistory.value = res.items ?? []
  } catch {
    batchHistory.value = []
  } finally {
    historyLoading.value = false
  }
}

async function loadHistoryLog(batchId: string): Promise<void> {
  if (!props.workspaceRecId) return
  historyLogLoading.value = true
  try {
    const res = await fetchWorkspacePomVersionLog(props.workspaceRecId, batchId, 0)
    historyLogContent.value = res.content
    historyLogComplete.value = res.complete
    const idx = batchHistory.value.findIndex((r) => r.rec_id === res.batch.rec_id)
    if (idx >= 0) batchHistory.value[idx] = res.batch
  } catch (e) {
    message.error((e as Error).message)
    historyLogContent.value = ''
  } finally {
    historyLogLoading.value = false
  }
}

async function refreshLiveLog(batchId: string): Promise<void> {
  if (!props.workspaceRecId || logViewMode.value !== 'live') return
  try {
    const res = await fetchWorkspacePomVersionLog(props.workspaceRecId, batchId, 0)
    if (res.content) {
      logText.value = res.content
    }
    const idx = batchHistory.value.findIndex((r) => r.rec_id === res.batch.rec_id)
    if (idx >= 0) batchHistory.value[idx] = res.batch
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

function updateProgress(st: BatchPomVersionStatusResponse): void {
  if (st.foreign) {
    foreignJob.value = true
    running.value = true
    progressText.value = t('projectBulkPomVersionRunningForeign', {
      workspace: st.active_workspace_name ?? '—',
    })
    return
  }
  foreignJob.value = false
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
    ? t('projectBulkPomVersionRunningDetail', { done, total, project })
    : t('projectBulkPomVersionRunning', { done, total })
}

function stopPolling(): void {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function buildResumeLogHeader(st: BatchPomVersionStatusResponse): string {
  const rec = st.batch_rec_id
    ? batchHistory.value.find((r) => r.rec_id === st.batch_rec_id)
    : undefined
  const tag = st.tag?.trim() || rec?.version_tag?.trim() || versionTag.value || '—'
  const commit =
    st.commit_message?.trim() || rec?.commit_message?.trim() || commitMessage.value.trim() || '—'
  return [
    `[${formatLogTime()}] ${t('projectBulkPomVersionLogResume')}`,
    t('projectBulkPomVersionLogParams', { tag, commit }),
  ].join('\n')
}

function restoreLiveSession(st: BatchPomVersionStatusResponse, opts?: { startPoll?: boolean }): void {
  if (st.foreign) {
    if (st.status === 'running' && opts?.startPoll !== false) {
      startPolling(false)
    }
    return
  }
  if (st.tag?.trim()) versionTag.value = st.tag.trim()
  if (st.commit_message?.trim()) commitMessage.value = st.commit_message.trim()
  if (st.batch_rec_id) activeBatchRecId.value = st.batch_rec_id
  logHeader.value = buildResumeLogHeader(st)
  logViewMode.value = 'live'
  selectedBatchId.value = null
  const tag = st.tag?.trim() || versionTag.value
  syncLogFromStatus(st, tag, st.status === 'completed')
  if (st.batch_rec_id) void refreshLiveLog(st.batch_rec_id)
  if (st.status === 'running' && opts?.startPoll !== false) {
    startPolling(false)
  }
}

async function pollOnce(showToast: boolean): Promise<boolean> {
  if (!props.workspaceRecId) return true
  const st = await batchPomVersionStatus(props.workspaceRecId)
  updateProgress(st)
  if (!st.foreign) {
    if (st.batch_rec_id) activeBatchRecId.value = st.batch_rec_id
    const tag = st.tag?.trim() || versionTag.value.trim()
    if (logViewMode.value === 'live') {
      syncLogFromStatus(st, tag, st.status === 'completed')
      if (st.batch_rec_id) await refreshLiveLog(st.batch_rec_id)
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
    const tag = st.tag?.trim() || versionTag.value.trim()
    const updated = st.updated ?? 0
    if ((st.failed ?? 0) > 0) {
      if (showToast) {
        message.warning(
          t('projectBulkPomVersionPartial', {
            updated,
            skipped: st.skipped ?? 0,
            failed: st.failed ?? 0,
          }),
        )
      }
    } else if (showToast) {
      message.success(
        t('projectBulkPomVersionSuccess', {
          updated,
          skipped: st.skipped ?? 0,
          tag,
        }),
      )
    }
    if (updated > 0) emit('success')
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
  versionTag.value = 'V10.0.0-P01'
  commitMessage.value = ''
  saving.value = false
  running.value = false
  foreignJob.value = false
  progressText.value = ''
  logText.value = ''
  logHeader.value = ''
  activeBatchRecId.value = null
  logViewMode.value = 'live'
  selectedBatchId.value = null
  historyLogContent.value = ''
}

async function initDialogOnOpen(): Promise<void> {
  resetDialogState()
  if (!props.workspaceRecId) return

  await loadBatchHistory()

  let st: BatchPomVersionStatusResponse
  try {
    st = await batchPomVersionStatus(props.workspaceRecId)
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
  const tag = versionTag.value.trim()
  if (!tag) {
    message.warning(t('projectBulkPomVersionTagRequired'))
    return
  }
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  if (running.value || foreignJob.value) {
    message.warning(t('projectBulkPomVersionAlreadyRunning'))
    return
  }

  const commit = commitMessage.value.trim()
  logHeader.value = [
    `[${formatLogTime()}] ${t('projectBulkPomVersionLogStart')}`,
    t('projectBulkPomVersionLogParams', { tag, commit: commit || t('projectBulkPomVersionCommitAuto') }),
  ].join('\n')
  logText.value = logHeader.value
  logViewMode.value = 'live'
  selectedBatchId.value = null

  saving.value = true
  running.value = true
  try {
    const start = await batchUpdatePomVersion({
      workspace_rec_id: props.workspaceRecId,
      version_tag: tag,
      commit_message: commit || undefined,
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
    :title="$t('projectBulkPomVersionTitle')"
    width="960px"
    initial-height="520px"
    :show-footer="true"
    :confirm-loading="saving"
    :confirm-disabled="running || foreignJob"
    @confirm="onSubmit"
  >
    <div class="project-bulk-pom-version__layout">
      <div class="project-bulk-pom-version__form-panel">
        <p class="project-bulk-pom-version__hint">
          {{ $t('projectBulkPomVersionHint', { name: workspaceName, count: projectCount }) }}
        </p>
        <el-alert
          v-if="foreignJob"
          :title="progressText"
          type="warning"
          :closable="false"
          show-icon
          class="project-bulk-pom-version__alert"
        />
        <el-alert
          v-else-if="running && progressText"
          :title="progressText"
          type="info"
          :closable="false"
          show-icon
          class="project-bulk-pom-version__alert"
        />
        <el-form label-width="108px" @submit.prevent="onSubmit">
          <el-form-item :label="$t('projectBulkPomVersionTag')" required>
            <el-input
              v-model="versionTag"
              :placeholder="$t('projectAutoReleaseTagPlaceholder')"
              :disabled="running || foreignJob"
              autocomplete="off"
            />
          </el-form-item>
          <el-form-item :label="$t('projectBulkPomVersionCommit')">
            <el-input
              v-model="commitMessage"
              type="textarea"
              :rows="2"
              :placeholder="$t('projectBulkPomVersionCommitPlaceholder')"
              :disabled="running || foreignJob"
            />
          </el-form-item>
          <p class="project-bulk-pom-version__note">{{ $t('projectBulkPomVersionNote') }}</p>
        </el-form>
      </div>

      <aside class="project-bulk-pom-version__log-panel">
        <section class="project-bulk-pom-version__history">
          <header class="project-bulk-pom-version__history-head">
            <span>{{ $t('projectBulkPomVersionHistory') }}</span>
            <el-button
              v-if="showViewLive"
              link
              type="primary"
              size="small"
              @click="viewLiveLog"
            >
              {{ $t('projectBulkPomVersionViewLive') }}
            </el-button>
          </header>
          <ul v-loading="historyLoading" class="project-bulk-pom-version__history-list">
            <li v-for="r in batchHistory" :key="r.rec_id">
              <button
                type="button"
                class="project-bulk-pom-version__history-item"
                :class="{
                  'is-active': logViewMode === 'history' && selectedBatchId === r.rec_id,
                  'is-live': logViewMode === 'live' && activeBatchRecId === r.rec_id,
                }"
                @click="selectBatchRecord(r.rec_id)"
              >
                <el-tag :type="releaseStatusTagType(r.status)" size="small" effect="plain">
                  #{{ r.batch_num }}
                </el-tag>
                <span class="project-bulk-pom-version__history-tag">{{ r.version_tag }}</span>
                <span class="project-bulk-pom-version__history-time">
                  {{ formatBuildTime(r.started_at ?? r.created_at) }}
                </span>
              </button>
            </li>
          </ul>
          <p
            v-if="!historyLoading && !batchHistory.length"
            class="project-bulk-pom-version__history-empty"
          >
            {{ $t('projectBulkPomVersionHistoryEmpty') }}
          </p>
        </section>
        <ProjectReleaseLogPanel
          class="project-bulk-pom-version__log-console"
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
.project-bulk-pom-version {
  &__layout {
    display: flex;
    gap: 16px;
    min-height: 360px;
    height: 100%;
    align-items: stretch;
  }

  &__form-panel {
    flex: 0 0 340px;
    min-width: 0;
  }

  &__log-panel {
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
    margin-bottom: 12px;
  }

  &__history {
    flex: 0 0 auto;
    max-height: 120px;
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

  &__history-tag {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__history-time {
    flex-shrink: 0;
    color: var(--el-text-color-secondary);
    font-size: 11px;
  }

  &__history-empty {
    margin: 0;
    padding: 8px 10px;
    font-size: 12px;
    color: var(--el-text-color-placeholder);
  }

  &__log-console {
    flex: 1;
    min-height: 200px;
  }
}
</style>
