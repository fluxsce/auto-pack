<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import {
  batchAutoRelease,
  batchAutoReleaseStatus,
  fetchWorkspaceReleaseHistory,
  fetchWorkspaceReleaseLog,
  type BatchAutoReleaseResultItem,
  type BatchAutoReleaseStatusResponse,
  type WorkspaceReleaseRecord,
} from '@/request/api/project'
import { formatBuildTime } from '../project-format'
import { releaseStatusTagType } from '../release-status'
import { ProjectReleaseLogPanel } from './panels'

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

const releaseBranch = ref('')
const tagBranch = ref('')
const releaseNote = ref('')
const releaseDate = ref('')
const updatePomVersion = ref(true)
const createTargetBranch = ref(false)
const targetBranch = ref('')
const saving = ref(false)
const releasing = ref(false)
const foreignRelease = ref(false)
const releaseProgress = ref('')
const logText = ref('')
const logHeader = ref('')
const activeLogParams = ref<ReleaseLogParams | null>(null)
const activeReleaseRecId = ref<string | null>(null)
const releaseHistory = ref<WorkspaceReleaseRecord[]>([])
const historyLoading = ref(false)
const logViewMode = ref<'live' | 'history'>('live')
const selectedReleaseId = ref<string | null>(null)
const historyLogContent = ref('')
const historyLogLoading = ref(false)
const historyLogComplete = ref(true)
let pollTimer: ReturnType<typeof setInterval> | null = null
const pollIntervalMs = 2000

const displayLogContent = computed(() =>
  logViewMode.value === 'history' ? historyLogContent.value : logText.value,
)

const selectedRelease = computed(() =>
  releaseHistory.value.find((r) => r.rec_id === selectedReleaseId.value) ?? null,
)

const displayReleaseNum = computed(() => {
  if (logViewMode.value === 'history') return selectedRelease.value?.release_num ?? null
  const id = activeReleaseRecId.value
  if (!id) return null
  return releaseHistory.value.find((r) => r.rec_id === id)?.release_num ?? null
})

const displayReleaseStatus = computed(() => {
  if (logViewMode.value === 'history') return selectedRelease.value?.status ?? null
  if (releasing.value) return 'running'
  if (activeReleaseRecId.value) {
    return releaseHistory.value.find((r) => r.rec_id === activeReleaseRecId.value)?.status ?? null
  }
  return null
})

const logPanelComplete = computed(() => {
  if (logViewMode.value === 'history') return historyLogComplete.value
  return !releasing.value
})

const logPanelLoading = computed(() => {
  if (logViewMode.value === 'history') return historyLogLoading.value
  return releasing.value && !logText.value
})

const showViewLive = computed(
  () => logViewMode.value === 'history' && (releasing.value || activeReleaseRecId.value != null),
)

type ReleaseLogParams = {
  tag: string
  branch: string
  note: string
  date: string
  updatePom: boolean
}

function todayISO(): string {
  const d = new Date()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

function releaseNoteFromTag(tag: string): string {
  const t = tag.trim()
  return t ? `${t}自动发版说明` : ''
}

function formatLogTime(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function statusLabel(status: BatchAutoReleaseResultItem['status']): string {
  if (status === 'ok') return t('projectAutoReleaseLogStatusOk')
  if (status === 'skipped') return t('projectAutoReleaseLogStatusSkipped')
  return t('projectAutoReleaseLogStatusFailed')
}

function appendLog(lines: string[]): void {
  const block = lines.join('\n')
  logText.value = logText.value ? `${logText.value}\n\n${block}` : block
}

async function loadReleaseHistory(): Promise<void> {
  if (!props.workspaceRecId) return
  historyLoading.value = true
  try {
    const res = await fetchWorkspaceReleaseHistory(props.workspaceRecId)
    releaseHistory.value = res.items ?? []
  } catch {
    releaseHistory.value = []
  } finally {
    historyLoading.value = false
  }
}

async function loadHistoryLog(releaseId: string): Promise<void> {
  if (!props.workspaceRecId) return
  historyLogLoading.value = true
  try {
    const res = await fetchWorkspaceReleaseLog(props.workspaceRecId, releaseId, 0)
    historyLogContent.value = res.content
    historyLogComplete.value = res.complete
    const idx = releaseHistory.value.findIndex((r) => r.rec_id === res.release.rec_id)
    if (idx >= 0) releaseHistory.value[idx] = res.release
  } catch (e) {
    message.error((e as Error).message)
    historyLogContent.value = ''
  } finally {
    historyLogLoading.value = false
  }
}

function viewLiveLog(): void {
  logViewMode.value = 'live'
  selectedReleaseId.value = null
}

async function selectReleaseRecord(releaseId: string): Promise<void> {
  selectedReleaseId.value = releaseId
  logViewMode.value = 'history'
  historyLogContent.value = ''
  await loadHistoryLog(releaseId)
}

function paramsFromStatus(st: BatchAutoReleaseStatusResponse): ReleaseLogParams | null {
  const tag = st.tag?.trim()
  if (!tag) return null
  return {
    tag,
    branch: st.release_branch?.trim() ?? '',
    note: releaseNoteFromTag(tag),
    date: releaseDate.value.trim() || todayISO(),
    updatePom: updatePomVersion.value,
  }
}

function formatProjectResultLines(item: BatchAutoReleaseResultItem): string[] {
  const lines: string[] = [`[${statusLabel(item.status)}] ${item.project_name}`]
  const steps = item.steps ?? []
  if (steps.length > 0) {
    for (const step of steps) {
      lines.push(`  · ${step}`)
    }
  } else if (item.status !== 'skipped') {
    lines.push(`  · ${t('projectAutoReleaseLogNoSteps')}`)
  }
  const msg = item.message?.trim()
  if (msg) {
    lines.push(`  → ${msg}`)
  } else if (item.status === 'failed') {
    lines.push(`  → ${t('projectAutoReleaseLogNoMessage')}`)
  }
  return lines
}

/** Rebuilds progress/result section under the fixed header (called each poll). */
function syncReleaseLog(
  st: BatchAutoReleaseStatusResponse,
  params: ReleaseLogParams | null,
  final: boolean,
): void {
  if (st.foreign) return
  const body: string[] = []
  if (final && params) {
    body.push(`[${formatLogTime()}] ${t('projectAutoReleaseLogDone')}`)
    body.push(
      t('projectAutoReleaseLogSummary', {
        released: st.released ?? 0,
        skipped: st.skipped ?? 0,
        failed: st.failed ?? 0,
        tag: params.tag,
      }),
    )
    body.push('')
  } else if (st.status === 'running') {
    body.push(`[${formatLogTime()}] ${t('projectAutoReleaseLogProgress')}`)
    body.push('')
  }
  for (const item of st.results ?? []) {
    body.push(...formatProjectResultLines(item))
    body.push('')
  }
  if (st.status === 'running' && st.current_project?.trim()) {
    body.push(`[…] ${t('projectAutoReleaseLogInProgress', { project: st.current_project.trim() })}`)
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

function updateReleaseProgress(st: BatchAutoReleaseStatusResponse): void {
  if (st.foreign) {
    foreignRelease.value = true
    releasing.value = true
    releaseProgress.value = t('projectAutoReleaseRunningForeign', {
      workspace: st.active_workspace_name ?? '—',
    })
    return
  }
  foreignRelease.value = false
  if (st.status !== 'running') {
    releasing.value = false
    releaseProgress.value = ''
    return
  }
  releasing.value = true
  const done = st.done_count ?? 0
  const total = st.total_count ?? 0
  const project = st.current_project?.trim()
  releaseProgress.value = project
    ? t('projectAutoReleaseRunningDetail', { done, total, project })
    : t('projectAutoReleaseRunning', { done, total })
}

function stopPolling(): void {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function pollReleaseOnce(
  logParams: ReleaseLogParams | null,
  showToast: boolean,
): Promise<boolean> {
  if (!props.workspaceRecId) return true
  const st = await batchAutoReleaseStatus(props.workspaceRecId)
  updateReleaseProgress(st)
  const params = logParams ?? activeLogParams.value ?? paramsFromStatus(st)
  if (!st.foreign) {
    if (st.release_rec_id) activeReleaseRecId.value = st.release_rec_id
    if (logViewMode.value === 'live') {
      syncReleaseLog(st, params, st.status === 'completed')
    }
  }
  if (st.status === 'running') {
    return false
  }
  stopPolling()
  releasing.value = false
  foreignRelease.value = false
  releaseProgress.value = ''
  void loadReleaseHistory()
  if (st.status === 'completed' && params) {
    const tag = params.tag
    if ((st.failed ?? 0) > 0) {
      if (showToast) {
        message.warning(
          t('projectBulkAutoReleasePartial', {
            released: st.released ?? 0,
            skipped: st.skipped ?? 0,
            failed: st.failed ?? 0,
            detail: '',
          }),
        )
      }
      if ((st.released ?? 0) > 0) {
        emit('success')
      }
    } else if (showToast) {
      message.success(
        t('projectBulkAutoReleaseSuccess', {
          released: st.released ?? 0,
          skipped: st.skipped ?? 0,
          tag,
        }),
      )
      emit('success')
    }
  }
  return true
}

function startPolling(logParams: ReleaseLogParams | null, showToast: boolean): void {
  stopPolling()
  void pollReleaseOnce(logParams, showToast)
  pollTimer = setInterval(() => {
    void pollReleaseOnce(logParams, showToast)
  }, pollIntervalMs)
}

async function refreshReleaseStatus(): Promise<void> {
  if (!props.workspaceRecId || !open.value) return
  try {
    const st = await batchAutoReleaseStatus(props.workspaceRecId)
    updateReleaseProgress(st)
    if (st.release_rec_id) activeReleaseRecId.value = st.release_rec_id
    if (st.status === 'running') {
      if (!st.foreign) {
        const params = paramsFromStatus(st)
        if (params) {
          activeLogParams.value = params
        }
        if (!logHeader.value) {
          logHeader.value = [
            `[${formatLogTime()}] ${t('projectAutoReleaseLogResume')}`,
            t('projectAutoReleaseLogParams', {
              branch: st.release_branch ?? '—',
              tag: st.tag ?? '—',
              target: '—',
            }),
          ].join('\n')
        }
        syncReleaseLog(st, activeLogParams.value, false)
      }
      startPolling(activeLogParams.value, false)
    } else if (st.status === 'completed' && !st.foreign) {
      const params = activeLogParams.value ?? paramsFromStatus(st)
      if (params) {
        activeLogParams.value = params
        syncReleaseLog(st, params, true)
      }
    }
  } catch {
    /* ignore status probe errors */
  }
}

watch(tagBranch, (tag) => {
  releaseNote.value = releaseNoteFromTag(tag)
})

watch(open, (v) => {
  if (v) {
    releaseBranch.value = ''
    tagBranch.value = ''
    releaseNote.value = ''
    releaseDate.value = todayISO()
    updatePomVersion.value = true
    createTargetBranch.value = false
    targetBranch.value = ''
    logText.value = ''
    logHeader.value = ''
    activeLogParams.value = null
    activeReleaseRecId.value = null
    logViewMode.value = 'live'
    selectedReleaseId.value = null
    historyLogContent.value = ''
    historyLogComplete.value = true
    releasing.value = false
    foreignRelease.value = false
    releaseProgress.value = ''
    stopPolling()
    void loadReleaseHistory()
    void refreshReleaseStatus()
  } else {
    stopPolling()
  }
})

onUnmounted(() => {
  stopPolling()
})

async function onSubmit(): Promise<void> {
  const branch = releaseBranch.value.trim()
  const tag = tagBranch.value.trim()
  if (!branch) {
    message.warning(t('projectAutoReleaseBranchRequired'))
    return
  }
  if (!tag) {
    message.warning(t('projectAutoReleaseTagRequired'))
    return
  }
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  if (createTargetBranch.value && !targetBranch.value.trim()) {
    message.warning(t('projectAutoReleaseTargetRequired'))
    return
  }

  const note = releaseNote.value.trim() || releaseNoteFromTag(tag)
  const date = releaseDate.value.trim() || todayISO()
  const logParams: ReleaseLogParams = { tag, branch, note, date, updatePom: updatePomVersion.value }
  activeLogParams.value = logParams
  logHeader.value = [
    `[${formatLogTime()}] ${t('projectAutoReleaseLogStart')}`,
    t('projectAutoReleaseLogParams', {
      branch,
      tag,
      target: createTargetBranch.value ? targetBranch.value.trim() : '—',
    }),
    t('projectAutoReleaseLogParamsNote', { note }),
    t('projectAutoReleaseLogParamsDate', { date }),
    t('projectAutoReleaseLogParamsPom', {
      on: updatePomVersion.value ? t('enabled') : t('disabled'),
    }),
  ].join('\n')
  logText.value = logHeader.value
  logViewMode.value = 'live'
  selectedReleaseId.value = null

  saving.value = true
  releasing.value = true
  try {
    const start = await batchAutoRelease({
      workspace_rec_id: props.workspaceRecId,
      release_branch: branch,
      tag_branch: tag,
      release_note: note,
      release_date: date,
      allow_repeat: true,
      update_pom_version: updatePomVersion.value,
      create_target_branch: createTargetBranch.value,
      target_branch: createTargetBranch.value ? targetBranch.value.trim() : undefined,
    })
    activeReleaseRecId.value = start.release_rec_id
    void loadReleaseHistory()
    releaseProgress.value = t('projectAutoReleaseRunning', {
      done: 0,
      total: props.projectCount,
    })
    startPolling(logParams, true)
  } catch (e) {
    releasing.value = false
    const err = (e as Error).message
    appendLog([`[${formatLogTime()}] ${t('projectAutoReleaseLogError')}`, err])
    message.error(err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectBulkAutoReleaseTitle')"
    width="1000px"
    :show-footer="true"
    :confirm-text="$t('projectAutoReleaseExecute')"
    :confirm-loading="saving || releasing"
    :confirm-disabled="releasing || foreignRelease"
    @confirm="onSubmit"
  >
    <div class="project-bulk-release__layout">
      <div class="project-bulk-release__main">
        <el-alert
          v-if="releasing"
          :title="releaseProgress || $t('projectAutoReleaseRunningTitle')"
          type="warning"
          :closable="false"
          show-icon
          class="project-bulk-release__running-alert"
        />
        <p class="project-bulk-release__hint">
          {{ $t('projectBulkAutoReleaseHint', { name: workspaceName, count: projectCount }) }}
        </p>
        <el-form label-width="108px" class="project-bulk-release__form" @submit.prevent="onSubmit">
          <el-form-item :label="$t('projectAutoReleaseBranch')" required>
            <el-input
              v-model="releaseBranch"
              :placeholder="$t('projectAutoReleaseBranchPlaceholder')"
              autocomplete="off"
            />
          </el-form-item>
          <el-form-item :label="$t('projectAutoReleaseTag')" required class="project-bulk-release__tag-item">
            <div class="project-bulk-release__field">
              <el-input
                v-model="tagBranch"
                :placeholder="$t('projectAutoReleaseTagPlaceholder')"
                autocomplete="off"
              />
              <p class="project-bulk-release__switch-hint">{{ $t('projectAutoReleaseTagHint') }}</p>
            </div>
          </el-form-item>
          <el-form-item :label="$t('projectAutoReleaseDate')">
            <el-date-picker
              v-model="releaseDate"
              type="date"
              value-format="YYYY-MM-DD"
              class="project-bulk-release__full"
            />
          </el-form-item>
          <el-form-item :label="$t('projectAutoReleaseNote')">
            <el-input
              v-model="releaseNote"
              type="textarea"
              :rows="2"
              readonly
              :placeholder="$t('projectAutoReleaseNotePlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="$t('projectAutoReleaseTargetBranch')" class="project-bulk-release__switch-item">
            <div class="project-bulk-release__field">
              <div class="project-bulk-release__switch-row">
                <el-switch v-model="createTargetBranch" />
                <span class="project-bulk-release__switch-label">
                  {{ $t('projectAutoReleaseTargetBranchEnable') }}
                </span>
              </div>
              <el-input
                v-model="targetBranch"
                class="project-bulk-release__full"
                :disabled="!createTargetBranch"
                :placeholder="$t('projectAutoReleaseTargetBranchPlaceholder')"
                autocomplete="off"
              />
              <p class="project-bulk-release__switch-hint">
                {{ $t('projectAutoReleaseTargetBranchHint') }}
              </p>
            </div>
          </el-form-item>
          <el-form-item :label="$t('projectAutoReleaseUpdatePom')" class="project-bulk-release__switch-item">
            <div class="project-bulk-release__field">
              <el-switch v-model="updatePomVersion" />
              <p class="project-bulk-release__switch-hint">
                {{ $t('projectAutoReleaseUpdatePomHint') }}
              </p>
            </div>
          </el-form-item>
        </el-form>
        <p class="project-bulk-release__note">{{ $t('projectBulkAutoReleaseNote') }}</p>
      </div>

      <aside class="project-bulk-release__log-panel">
        <section class="project-bulk-release__history">
          <header class="project-bulk-release__history-head">
            <span>{{ $t('projectAutoReleaseHistory') }}</span>
            <el-button
              v-if="showViewLive"
              link
              type="primary"
              size="small"
              @click="viewLiveLog"
            >
              {{ $t('projectAutoReleaseViewLive') }}
            </el-button>
          </header>
          <ul v-loading="historyLoading" class="project-bulk-release__history-list">
            <li v-for="r in releaseHistory" :key="r.rec_id">
              <button
                type="button"
                class="project-bulk-release__history-item"
                :class="{
                  'is-active':
                    logViewMode === 'history' && selectedReleaseId === r.rec_id,
                  'is-live': logViewMode === 'live' && activeReleaseRecId === r.rec_id,
                }"
                @click="selectReleaseRecord(r.rec_id)"
              >
                <el-tag :type="releaseStatusTagType(r.status)" size="small" effect="plain">
                  #{{ r.release_num }}
                </el-tag>
                <span class="project-bulk-release__history-tag">{{ r.tag_branch }}</span>
                <span class="project-bulk-release__history-time">
                  {{ formatBuildTime(r.started_at ?? r.created_at) }}
                </span>
              </button>
            </li>
          </ul>
          <p v-if="!historyLoading && !releaseHistory.length" class="project-bulk-release__history-empty">
            {{ $t('projectAutoReleaseHistoryEmpty') }}
          </p>
        </section>
        <ProjectReleaseLogPanel
          class="project-bulk-release__log-console"
          :release-num="displayReleaseNum"
          :release-status="displayReleaseStatus"
          :content="displayLogContent"
          :loading="logPanelLoading"
          :complete="logPanelComplete"
        />
      </aside>
    </div>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-bulk-release {
  &__layout {
    display: flex;
    gap: 16px;
    min-height: 360px;
    height: 100%;
    align-items: stretch;
  }

  &__main {
    flex: 0 0 420px;
    min-width: 0;
    min-height: 0;
    overflow-y: auto;
  }

  &__running-alert {
    margin-bottom: 12px;
  }

  &__hint {
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__note {
    margin: 12px 0 0;
    font-size: 12px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
  }

  &__full {
    width: 100%;
  }

  &__switch-item :deep(.el-form-item__label) {
    white-space: nowrap;
    line-height: 32px;
  }

  &__switch-item :deep(.el-form-item__content) {
    align-items: flex-start;
  }

  &__field {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
  }

  &__switch-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 32px;
  }

  &__switch-label {
    font-size: 13px;
    line-height: 1.4;
    color: var(--shell-content-text-secondary, var(--el-text-color-regular));
  }

  &__switch-hint {
    margin: 0;
    font-size: 12px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
    word-break: break-word;
  }

  &__log-panel {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    background: var(--el-fill-color-blank);
    overflow: hidden;
  }

  &__history {
    flex-shrink: 0;
    border-bottom: 1px solid var(--el-border-color-lighter);
    max-height: 140px;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  &__history-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 10px;
    font-size: 12px;
    font-weight: 600;
    color: var(--el-text-color-primary);
  }

  &__history-list {
    list-style: none;
    margin: 0;
    padding: 0 6px 6px;
    overflow-y: auto;
    flex: 1;
    min-height: 0;
  }

  &__history-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 6px 8px;
    border: none;
    border-radius: 6px;
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
      background: rgba(14, 165, 233, 0.1);
    }
  }

  &__history-tag {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: ui-monospace, monospace;
  }

  &__history-time {
    flex-shrink: 0;
    color: var(--el-text-color-secondary);
    font-size: 11px;
  }

  &__history-empty {
    margin: 0;
    padding: 4px 10px 8px;
    font-size: 12px;
    color: var(--el-text-color-placeholder);
  }

  &__log-console {
    flex: 1;
    min-height: 0;
    border: none;
    border-radius: 0;
  }
}
</style>

<style lang="scss">
.app-dialog--windowed.app-dialog--initial-height .app-dialog__body:has(.project-bulk-release__layout) {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.app-dialog--windowed.app-dialog--initial-height .app-dialog__content:has(.project-bulk-release__layout) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.app-dialog--windowed.app-dialog--initial-height
  .app-dialog__content:has(.project-bulk-release__layout)
  .project-bulk-release__layout {
  flex: 1;
  min-height: 0;
}

.app-dialog.is-fullscreen .app-dialog__content:has(.project-bulk-release__layout),
.app-dialog--fullscreen .app-dialog__content:has(.project-bulk-release__layout) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  max-width: none;
  width: 100%;
  margin: 0;
}
</style>
