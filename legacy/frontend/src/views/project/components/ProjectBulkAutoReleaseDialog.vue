<script setup lang="ts">
import { InfoFilled } from '@element-plus/icons-vue'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import {
  batchAutoRelease,
  batchAutoReleaseStatus,
  fetchBatchAutoReleaseDefaults,
  saveBatchAutoReleasePostOptions,
  type BatchReleasePostOptions,
  fetchProjects,
  fetchWorkspaceReleaseHistory,
  fetchWorkspaceReleaseLog,
  type BatchAutoReleaseResultItem,
  type BatchAutoReleaseStatusResponse,
  type WorkspaceReleaseRecord,
} from '@/request/api/project'
import {
  fetchProjectBuild,
  fetchProjectWorkspaceFtpPushStatus,
  fetchProjectWorkspace,
  fetchProjectWorkspaceFtpRemoteNote,
  pushProjectWorkspaceFtp,
  triggerProjectBuild,
} from '@/request/api/build'
import { fetchWorkspaceMavenDependencyAnalysis } from '@/request/api/workspace'
import { waitWorkspaceSourceSync } from '@/utils/workspace-source-sync'
import { orderProjectsForBuild, waitForBuildComplete } from '@/utils/ordered-project-build'
import { fetchEnvSettings } from '@/request/api/env'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import FormLabelTip from '@/components/form/FormLabelTip.vue'
import { isValidGitTagName, normalizeGitTagName } from '@/utils/git-ref'
import { pickTargetArtifact } from '@/utils/workspace-artifact'
import {
  backupFilePath,
  buildManifestPreview,
  defaultBackupDir,
  mergeUpdateNotePreview,
  resolvePushLayout,
} from '@/utils/workspace-ftp'
import { mergeBuildConfig } from '../project-defaults'
import type { EnvSetting, PackProject, ProjectWorkspaceEntry } from '@/types'
import { formatBuildTime } from '../project-format'
import { releaseStatusTagType } from '../release-status'
import { ProjectReleaseLogPanel } from './panels'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  workspaceRecId: string
  workspaceName: string
  workspaceCode: string
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
const autoBuildAfterRelease = ref(true)
const autoFtpPushAfterBuild = ref(true)
const workspaceProjects = ref<PackProject[]>([])
const projectsLoading = ref(false)
const postProcessing = ref(false)
const ftpEnvs = ref<EnvSetting[]>([])
const loadingFtpEnvs = ref(false)
const ftpEnvRecId = ref('')
const pushPath = ref('')
const backupPath = ref('')
const backupManual = ref(false)
const pusher = ref('')
const taskRef = ref('')
const detail = ref('')
const ftpPushProjectIds = ref<string[]>([])
const ftpPushPicker = ref('')
const remoteNoteContent = ref('')
const remoteNoteExists = ref(false)
const remoteNoteLoading = ref(false)
let remoteNoteTimer: ReturnType<typeof setTimeout> | null = null
let postReleaseHandledFor: string | null = null
const defaultsLoading = ref(false)
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
  return !releasing.value && !postProcessing.value
})

const logPanelLoading = computed(() => {
  if (logViewMode.value === 'history') return historyLogLoading.value
  return (releasing.value || postProcessing.value) && !logText.value
})

const showViewLive = computed(
  () => logViewMode.value === 'history' && (releasing.value || activeReleaseRecId.value != null),
)

const projectByName = computed(() => {
  const map = new Map<string, PackProject>()
  for (const p of workspaceProjects.value) {
    map.set(p.project_name, p)
  }
  return map
})

const projectByRecId = computed(() => {
  const map = new Map<string, PackProject>()
  for (const p of workspaceProjects.value) {
    map.set(p.rec_id, p)
  }
  return map
})

const ftpPushEnabled = computed(() => autoBuildAfterRelease.value && autoFtpPushAfterBuild.value)

const ftpPushProjectSet = computed(() => new Set(ftpPushProjectIds.value))

const enabledWorkspaceProjects = computed(() =>
  workspaceProjects.value.filter((p) => p.is_enabled === 1),
)

const availableFtpPushOptions = computed(() =>
  enabledWorkspaceProjects.value.filter((p) => !ftpPushProjectSet.value.has(p.rec_id)),
)

const selectedFtpProjects = computed(() =>
  ftpPushProjectIds.value
    .map((id) => projectByRecId.value.get(id))
    .filter((p): p is PackProject => p != null),
)

const uploadNamePreview = computed(() => 'app.war')

const pushLayout = computed(() => resolvePushLayout(pushPath.value, uploadNamePreview.value))

const defaultBackupPath = computed(() => {
  if (!pushPath.value.trim()) return ''
  return defaultBackupDir(pushPath.value, uploadNamePreview.value)
})

const ftpPreviewProjectRecId = computed(() => ftpPushProjectIds.value[0] ?? '')

const ftpNewManifestBlock = computed(() =>
  buildManifestPreview(
    new Date(),
    pusher.value,
    taskRef.value || releaseNote.value,
    backupFilePath(pushPath.value, uploadNamePreview.value, backupPath.value),
    pushPath.value.trim() ? pushLayout.value.updatePath : uploadNamePreview.value,
    detail.value,
  ),
)

const ftpPreviewContent = computed(() =>
  mergeUpdateNotePreview(remoteNoteContent.value, ftpNewManifestBlock.value),
)

const bulkReleaseTips = computed(() =>
  [
    t('projectBulkAutoReleaseHint', { name: props.workspaceName, count: props.projectCount }),
    t('projectBulkAutoReleaseNote'),
  ]
    .filter(Boolean)
    .join('\n\n'),
)

type ReleaseLogParams = {
  tag: string
  branch: string
  note: string
  date: string
  updatePom: boolean
}

let postOptionsSaveTimer: ReturnType<typeof setTimeout> | null = null

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

function applyReleaseFormFromJob(st: BatchAutoReleaseStatusResponse): void {
  if (st.foreign) return
  const branch = st.release_branch?.trim()
  const tag = st.tag?.trim()
  if (branch) releaseBranch.value = branch
  if (tag) {
    tagBranch.value = tag
    releaseNote.value = releaseNoteFromTag(tag)
    if (!taskRef.value.trim()) taskRef.value = releaseNoteFromTag(tag)
  }
}

function collectPostReleaseOptions(): BatchReleasePostOptions {
  return {
    auto_build_after_release: autoBuildAfterRelease.value,
    auto_ftp_push_after_build: autoFtpPushAfterBuild.value,
    ftp_push_project_ids: [...ftpPushProjectIds.value],
    ftp_env_rec_id: ftpEnvRecId.value,
    push_path: pushPath.value,
    backup_path: backupPath.value,
    backup_manual: backupManual.value,
    pusher: pusher.value,
    task_ref: taskRef.value,
    detail: detail.value,
    release_date: releaseDate.value.trim() || undefined,
    update_pom_version: updatePomVersion.value,
    create_target_branch: createTargetBranch.value,
    target_branch: createTargetBranch.value ? targetBranch.value.trim() : undefined,
  }
}

function applyPostReleaseOptions(opts: BatchReleasePostOptions | undefined): void {
  if (!opts) return
  autoBuildAfterRelease.value = opts.auto_build_after_release
  autoFtpPushAfterBuild.value = opts.auto_ftp_push_after_build
  ftpPushProjectIds.value = [...(opts.ftp_push_project_ids ?? [])]
  ftpEnvRecId.value = opts.ftp_env_rec_id ?? ''
  pushPath.value = opts.push_path ?? ''
  backupPath.value = opts.backup_path ?? ''
  backupManual.value = Boolean(opts.backup_manual)
  pusher.value = opts.pusher ?? ''
  taskRef.value = opts.task_ref ?? ''
  detail.value = opts.detail ?? ''
  if (opts.release_date?.trim()) releaseDate.value = opts.release_date.trim()
  if (opts.update_pom_version != null) updatePomVersion.value = opts.update_pom_version
  if (opts.create_target_branch != null) {
    createTargetBranch.value = opts.create_target_branch
    targetBranch.value = opts.target_branch?.trim() ?? ''
  }
  syncFtpPushProjectSelection()
  scheduleRemoteNoteLoad()
}

function applyPostOptionsFromStatus(st: BatchAutoReleaseStatusResponse): void {
  applyPostReleaseOptions(st.post_options)
}

async function persistPostReleaseOptions(): Promise<void> {
  if (!props.workspaceRecId) return
  if (!releasing.value && !postProcessing.value && !activeReleaseRecId.value) {
    return
  }
  try {
    await saveBatchAutoReleasePostOptions(props.workspaceRecId, collectPostReleaseOptions())
  } catch {
    /* ignore background save errors */
  }
}

function schedulePersistPostReleaseOptions(): void {
  if (!open.value || !props.workspaceRecId) return
  if (!releasing.value && !postProcessing.value && !activeReleaseRecId.value) {
    return
  }
  if (postOptionsSaveTimer) clearTimeout(postOptionsSaveTimer)
  postOptionsSaveTimer = setTimeout(() => {
    void persistPostReleaseOptions()
  }, 400)
}

function resetPostReleaseForm(): void {
  autoBuildAfterRelease.value = true
  autoFtpPushAfterBuild.value = true
  ftpPushProjectIds.value = []
  ftpPushPicker.value = ''
  resetFtpForm()
}

function applyReleaseFormFromRecord(rec: WorkspaceReleaseRecord): void {
  if (rec.release_branch?.trim()) releaseBranch.value = rec.release_branch.trim()
  if (rec.tag_branch?.trim()) {
    tagBranch.value = rec.tag_branch.trim()
    releaseNote.value = releaseNoteFromTag(rec.tag_branch)
    if (!taskRef.value.trim()) taskRef.value = releaseNoteFromTag(rec.tag_branch)
  }
  const target = rec.target_branch?.trim()
  if (target) {
    createTargetBranch.value = true
    targetBranch.value = target
  } else {
    createTargetBranch.value = false
    targetBranch.value = ''
  }
}

function buildResumeLogHeader(st: BatchAutoReleaseStatusResponse): string {
  const rec = st.release_rec_id
    ? releaseHistory.value.find((r) => r.rec_id === st.release_rec_id)
    : undefined
  return [
    `[${formatLogTime()}] ${t('projectAutoReleaseLogResume')}`,
    t('projectAutoReleaseLogParams', {
      branch: st.release_branch?.trim() || rec?.release_branch?.trim() || releaseBranch.value || '—',
      tag: st.tag?.trim() || rec?.tag_branch?.trim() || tagBranch.value || '—',
      target: rec?.target_branch?.trim() || '—',
    }),
  ].join('\n')
}

function restoreLiveReleaseSession(
  st: BatchAutoReleaseStatusResponse,
  opts?: { startPoll?: boolean; runPostOnCompleted?: boolean },
): void {
  if (st.foreign) {
    if (st.status === 'running' && opts?.startPoll !== false) {
      startPolling(null, false)
    }
    return
  }
  applyReleaseFormFromJob(st)
  if (st.release_rec_id) {
    const rec = releaseHistory.value.find((r) => r.rec_id === st.release_rec_id)
    if (rec) applyReleaseFormFromRecord(rec)
  }
  applyPostOptionsFromStatus(st)
  const params = paramsFromStatus(st)
  if (params) activeLogParams.value = params
  logHeader.value = buildResumeLogHeader(st)
  if (logViewMode.value === 'live') {
    syncReleaseLog(st, params, st.status === 'completed')
  }
  if (st.status === 'running' && opts?.startPoll !== false) {
    startPolling(activeLogParams.value, false)
  } else if (
    opts?.runPostOnCompleted !== false &&
    st.status === 'completed' &&
    params &&
    (st.released ?? 0) > 0 &&
    autoBuildAfterRelease.value
  ) {
    void runPostReleaseBuildAndFtp(st, params.tag)
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
    if (st.status === 'running' && st.post_options) applyPostReleaseOptions(st.post_options)
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
    const released = st.released ?? 0
    if ((st.failed ?? 0) > 0) {
      if (showToast) {
        message.warning(
          t('projectBulkAutoReleasePartial', {
            released,
            skipped: st.skipped ?? 0,
            failed: st.failed ?? 0,
            detail: '',
          }),
        )
      }
      if (released > 0 && !autoBuildAfterRelease.value) {
        emit('success')
      }
    } else if (showToast) {
      message.success(
        t('projectBulkAutoReleaseSuccess', {
          released,
          skipped: st.skipped ?? 0,
          tag,
        }),
      )
      if (!autoBuildAfterRelease.value) {
        emit('success')
      }
    }
    if (released > 0 && autoBuildAfterRelease.value && logViewMode.value === 'live') {
      void runPostReleaseBuildAndFtp(st, tag)
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
    if (st.status === 'running' && !st.foreign) {
      restoreLiveReleaseSession(st)
    }
  } catch {
    /* ignore status probe errors */
  }
}

watch(tagBranch, (tag) => {
  releaseNote.value = releaseNoteFromTag(tag)
  if (!taskRef.value.trim() && tag.trim()) {
    taskRef.value = releaseNoteFromTag(tag)
  }
})

watch(autoBuildAfterRelease, (on) => {
  if (!on) autoFtpPushAfterBuild.value = false
})

watch(
  [
    autoBuildAfterRelease,
    autoFtpPushAfterBuild,
    ftpPushProjectIds,
    ftpEnvRecId,
    pushPath,
    backupPath,
    backupManual,
    pusher,
    taskRef,
    detail,
  ],
  () => schedulePersistPostReleaseOptions(),
  { deep: true },
)

watch(pushPath, (v) => {
  if (!backupManual.value) {
    backupPath.value = v.trim() ? defaultBackupPath.value : ''
  }
})

watch(defaultBackupPath, (v) => {
  if (!backupManual.value && pushPath.value.trim()) {
    backupPath.value = v
  }
})

watch([ftpEnvRecId, pushPath, ftpPreviewProjectRecId], () => {
  scheduleRemoteNoteLoad()
})

function scheduleRemoteNoteLoad(): void {
  if (remoteNoteTimer) clearTimeout(remoteNoteTimer)
  remoteNoteTimer = setTimeout(() => {
    void loadRemoteNote()
  }, 400)
}

async function loadRemoteNote(): Promise<void> {
  if (!ftpPushEnabled.value || !ftpPreviewProjectRecId.value || !ftpEnvRecId.value || !pushPath.value.trim()) {
    remoteNoteContent.value = ''
    remoteNoteExists.value = false
    return
  }
  remoteNoteLoading.value = true
  try {
    const res = await fetchProjectWorkspaceFtpRemoteNote(ftpPreviewProjectRecId.value, {
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      file_name: uploadNamePreview.value,
    })
    remoteNoteExists.value = res.exists
    remoteNoteContent.value = res.exists ? res.content : ''
  } catch {
    remoteNoteContent.value = ''
    remoteNoteExists.value = false
  } finally {
    remoteNoteLoading.value = false
  }
}

function ftpEnvOptionLabel(item: EnvSetting): string {
  if (item.is_enabled === 1) return item.env_name
  return `${item.env_name} (${t('disabled')})`
}

function syncFtpPushProjectSelection(): void {
  const valid = new Set(enabledWorkspaceProjects.value.map((p) => p.rec_id))
  ftpPushProjectIds.value = ftpPushProjectIds.value.filter((id) => valid.has(id))
}

function onAddFtpPushProject(recId: string | undefined): void {
  if (!recId || ftpPushProjectSet.value.has(recId)) {
    ftpPushPicker.value = ''
    return
  }
  ftpPushProjectIds.value = [...ftpPushProjectIds.value, recId]
  ftpPushPicker.value = ''
}

function removeFtpPushProject(recId: string): void {
  ftpPushProjectIds.value = ftpPushProjectIds.value.filter((id) => id !== recId)
}

async function loadWorkspaceProjects(): Promise<void> {
  if (!props.workspaceRecId) {
    workspaceProjects.value = []
    ftpPushProjectIds.value = []
    return
  }
  projectsLoading.value = true
  try {
    const res = await fetchProjects({ workspace_rec_id: props.workspaceRecId })
    workspaceProjects.value = res.items ?? []
    syncFtpPushProjectSelection()
  } catch {
    workspaceProjects.value = []
    ftpPushProjectIds.value = []
  } finally {
    projectsLoading.value = false
  }
}

async function loadFtpEnvs(): Promise<void> {
  loadingFtpEnvs.value = true
  try {
    const res = await fetchEnvSettings('ftp')
    ftpEnvs.value = Array.isArray(res.items) ? res.items : []
  } catch {
    ftpEnvs.value = []
  } finally {
    loadingFtpEnvs.value = false
  }
}

function onFtpEnvChange(recId: string): void {
  const env = ftpEnvs.value.find((e) => e.rec_id === recId)
  if (!env) return
  try {
    const cfg = JSON.parse(env.config_json || '{}') as { base_path?: string }
    const base = (cfg.base_path ?? '/').trim() || '/'
    if (!pushPath.value.trim()) {
      pushPath.value = base === '/' ? '/update' : base
    }
  } catch {
    /* ignore */
  }
}

function resetFtpForm(): void {
  ftpEnvRecId.value = ''
  pushPath.value = ''
  backupPath.value = ''
  backupManual.value = false
  pusher.value = ''
  taskRef.value = ''
  detail.value = ''
}

function validateFtpConfig(): boolean {
  if (!ftpPushEnabled.value) return true
  if (!ftpEnvRecId.value) {
    message.warning(t('projectFtpPushEnvRequired'))
    return false
  }
  if (!pushPath.value.trim()) {
    message.warning(t('projectFtpPushPathRequired'))
    return false
  }
  if (!pusher.value.trim()) {
    message.warning(t('projectFtpPushPusherRequired'))
    return false
  }
  if (!taskRef.value.trim()) {
    message.warning(t('projectFtpPushTaskRefRequired'))
    return false
  }
  if (ftpPushProjectIds.value.length === 0) {
    message.warning(t('projectBulkAutoReleaseFtpProjectsRequired'))
    return false
  }
  return true
}

async function findDeployArtifact(
  project: PackProject,
): Promise<{ entry: ProjectWorkspaceEntry; outputDir: string } | null> {
  const cfg = mergeBuildConfig(project.project_type, project.config_json)
  const outputDir = (cfg.output_dir || 'target').trim() || 'target'
  const listing = await fetchProjectWorkspace(project.rec_id, outputDir)
  const entry = pickTargetArtifact(listing.entries ?? [])
  if (!entry) return null
  return { entry, outputDir }
}

async function pushArtifactFtp(
  project: PackProject,
  artifact: ProjectWorkspaceEntry,
  tag: string,
): Promise<void> {
  const envName = ftpEnvs.value.find((e) => e.rec_id === ftpEnvRecId.value)?.env_name ?? ftpEnvRecId.value
  appendLog([
    `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpProject', {
      name: project.project_name,
      file: artifact.name,
    })}`,
    `  ${t('projectFtpPushLogEnv', { name: envName })}`,
    `  ${t('projectFtpPushLogPushPath', { path: pushPath.value.trim() })}`,
    ...(remoteNoteExists.value ? [`  ${t('projectFtpPushLogNoteAppend')}`] : []),
    `  ${t('projectFtpPushLogUploading')}`,
  ])
  const detailLines = detail.value.trim()
    ? detail.value
    : `    1.${project.project_name}\n        ${artifact.path}`
  try {
    const start = await pushProjectWorkspaceFtp(project.rec_id, {
      path: artifact.path,
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      backup_path: backupPath.value.trim(),
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim() || `${tag}自动发版`,
      detail: detailLines,
    })
    const res = await waitForFtpPushTask(project.rec_id, start.task_id)
    appendLog([
      `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpOk', { name: project.project_name })}`,
      `  ${t('projectFtpPushLogBackupPath', { path: res.backup_path || '—' })}`,
      `  ${t('projectFtpPushLogUpdatePath', { path: res.update_path })}`,
      `  ${t('projectFtpPushLogInfoPath', { path: res.info_file_path })}`,
    ])
  } catch (e) {
    const msg = (e as Error).message
    appendLog([
      `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpFailed', {
        name: project.project_name,
        msg,
      })}`,
    ])
  }
}

async function waitForFtpPushTask(
  projectRecId: string,
  taskId: string,
): Promise<import('@/request/api/build').WorkspaceFtpPushResult> {
  for (;;) {
    const st = await fetchProjectWorkspaceFtpPushStatus(projectRecId, taskId)
    if (st.status === 'running' || st.status === 'idle') {
      await new Promise((resolve) => setTimeout(resolve, 1000))
      continue
    }
    if (st.status === 'success' && st.result) {
      return st.result
    }
    throw new Error(st.message || 'FTP push failed')
  }
}

async function runPostReleaseBuildAndFtp(
  st: BatchAutoReleaseStatusResponse,
  tag: string,
): Promise<void> {
  const releaseId = st.release_rec_id ?? activeReleaseRecId.value
  if (!releaseId || postReleaseHandledFor === releaseId) return
  if (!autoBuildAfterRelease.value || (st.released ?? 0) <= 0) return
  postReleaseHandledFor = releaseId

  if (ftpPushEnabled.value && !validateFtpConfig()) {
    appendLog([
      `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpConfigRequired')}`,
    ])
    return
  }

  const okItems = (st.results ?? []).filter((r) => r.status === 'ok')
  if (!okItems.length) return

  postProcessing.value = true
  appendLog(['', `[${formatLogTime()}] ${t('projectBulkAutoReleaseBuildStart')}`])
  if (ftpPushEnabled.value) {
    const names = ftpPushProjectIds.value
      .map((id) => projectByRecId.value.get(id)?.project_name ?? id)
      .join('、')
    appendLog([`  ${t('projectBulkAutoReleaseLogFtpTargets', { names: names || '—' })}`])
  }

  const releasedRecIds = okItems
    .map((item) => projectByName.value.get(item.project_name)?.rec_id)
    .filter((id): id is string => !!id)

  let buildQueue = releasedRecIds
    .map((id) => projectByRecId.value.get(id))
    .filter((p): p is PackProject => !!p)

  try {
    if (props.workspaceRecId && releasedRecIds.length > 0) {
      try {
        appendLog([`  ${t('projectBulkAutoBuildSyncStart')}`])
        await waitWorkspaceSourceSync(props.workspaceRecId, (st) => {
          if (st.status !== 'running') return
          const done = st.done_count ?? 0
          const total = st.total_count ?? 0
          const project = st.current_project?.trim() || '—'
          appendLog([
            `  ${t('projectBulkAutoBuildSyncProgress', { done, total, project })}`,
          ])
        })
      } catch {
        /* continue with best-effort dependency order */
      }
      const analysisRes = await fetchWorkspaceMavenDependencyAnalysis(props.workspaceRecId)
      const ordered = orderProjectsForBuild(
        workspaceProjects.value,
        analysisRes.analysis.full_build_order ?? [],
        {
          targetRecIds: releasedRecIds,
          edges: analysisRes.analysis.internal_edges,
        },
      )
      if (ordered.length > 0) {
        buildQueue = ordered
        appendLog([
          `  ${t('projectBulkAutoBuildPlanLine', { count: ordered.length })}`,
          ...ordered.map((p, i) => `    ${i + 1}. ${p.project_name}`),
        ])
      }
    }
  } catch {
    /* fallback: build released projects in release result order */
  }

  const releasedRecIdSet = new Set(releasedRecIds)

  try {
    for (const project of buildQueue) {
      appendLog([`[${formatLogTime()}] ${t('projectBulkAutoReleaseBuildProject', { name: project.project_name })}`])
      let buildNum = 0
      try {
        const build = await triggerProjectBuild(project.rec_id)
        buildNum = build.build_num
        const outcome = await waitForBuildComplete(project.rec_id, build.rec_id)
        if (outcome !== 'success') {
          const latest = await fetchProjectBuild(project.rec_id, build.rec_id)
          appendLog([
            `[${formatLogTime()}] ${t('projectBulkAutoReleaseBuildFailed', {
              name: project.project_name,
              msg: latest.message?.trim() || t('projectAutoReleaseLogNoMessage'),
            })}`,
          ])
          continue
        }
        appendLog([
          `[${formatLogTime()}] ${t('projectBulkAutoReleaseBuildOk', {
            name: project.project_name,
            num: buildNum,
          })}`,
        ])
      } catch (e) {
        appendLog([
          `[${formatLogTime()}] ${t('projectBulkAutoReleaseBuildFailed', {
            name: project.project_name,
            msg: (e as Error).message,
          })}`,
        ])
        continue
      }

      if (!ftpPushEnabled.value) continue
      if (!releasedRecIdSet.has(project.rec_id)) continue

      if (!ftpPushProjectSet.value.has(project.rec_id)) {
        appendLog([
          `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpSkipped', {
            name: project.project_name,
          })}`,
        ])
        continue
      }

      try {
        const found = await findDeployArtifact(project)
        if (!found) {
          const cfg = mergeBuildConfig(project.project_type, project.config_json)
          const dir = (cfg.output_dir || 'target').trim() || 'target'
          appendLog([
            `[${formatLogTime()}] ${t('projectBulkAutoReleaseArtifactNotFound', {
              name: project.project_name,
              dir,
            })}`,
          ])
          continue
        }
        await pushArtifactFtp(project, found.entry, tag)
      } catch (e) {
        appendLog([
          `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpFailed', {
            name: project.project_name,
            msg: (e as Error).message,
          })}`,
        ])
      }
    }
    appendLog([`[${formatLogTime()}] ${t('projectBulkAutoReleasePostDone')}`])
    emit('success')
  } finally {
    postProcessing.value = false
  }
}

function defaultReleaseBranch(code: string): string {
  const c = code.trim()
  return c ? `release/${c}` : ''
}

async function loadFormDefaults(): Promise<void> {
  const code = props.workspaceCode.trim()
  if (!props.workspaceRecId) {
    releaseBranch.value = defaultReleaseBranch(code)
    tagBranch.value = code
    releaseNote.value = releaseNoteFromTag(tagBranch.value)
    return
  }
  defaultsLoading.value = true
  try {
    const res = await fetchBatchAutoReleaseDefaults(props.workspaceRecId)
    releaseBranch.value = res.release_branch?.trim() || defaultReleaseBranch(code)
    tagBranch.value = res.tag_branch?.trim() || code
  } catch {
    releaseBranch.value = defaultReleaseBranch(code)
    tagBranch.value = code
  } finally {
    defaultsLoading.value = false
    releaseNote.value = releaseNoteFromTag(tagBranch.value)
  }
}

async function initDialogOnOpen(): Promise<void> {
  releaseDate.value = todayISO()
  updatePomVersion.value = true
  createTargetBranch.value = false
  targetBranch.value = ''
  remoteNoteContent.value = ''
  remoteNoteExists.value = false
  postReleaseHandledFor = null
  postProcessing.value = false
  logText.value = ''
  logHeader.value = ''
  activeLogParams.value = null
  activeReleaseRecId.value = null
  logViewMode.value = 'live'
  selectedReleaseId.value = null
  historyLogContent.value = ''
  historyLogComplete.value = true
  stopPolling()

  if (!props.workspaceRecId) {
    await loadFormDefaults()
    return
  }

  await Promise.all([loadWorkspaceProjects(), loadFtpEnvs(), loadReleaseHistory()])

  let st: BatchAutoReleaseStatusResponse
  try {
    st = await batchAutoReleaseStatus(props.workspaceRecId)
  } catch {
    await loadFormDefaults()
    return
  }

  updateReleaseProgress(st)

  const resumeSession = !st.foreign && st.status === 'running'

  if (resumeSession) {
    if (st.release_rec_id) activeReleaseRecId.value = st.release_rec_id
    restoreLiveReleaseSession(st, { runPostOnCompleted: true })
    return
  }

  activeReleaseRecId.value = null
  releasing.value = false
  foreignRelease.value = false
  releaseProgress.value = ''
  resetPostReleaseForm()

  if (st.foreign && st.status === 'running') {
    startPolling(null, false)
  }

  await loadFormDefaults()
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
  if (postOptionsSaveTimer) clearTimeout(postOptionsSaveTimer)
})

async function onSubmit(): Promise<void> {
  const branch = releaseBranch.value.trim()
  const tag = normalizeGitTagName(tagBranch.value)
  if (!branch) {
    message.warning(t('projectAutoReleaseBranchRequired'))
    return
  }
  if (!tag) {
    message.warning(t('projectAutoReleaseTagRequired'))
    return
  }
  if (!isValidGitTagName(tag)) {
    message.warning(t('projectAutoReleaseTagInvalid'))
    return
  }
  tagBranch.value = tag
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  if (createTargetBranch.value && !targetBranch.value.trim()) {
    message.warning(t('projectAutoReleaseTargetRequired'))
    return
  }
  if (autoBuildAfterRelease.value && ftpPushEnabled.value && !validateFtpConfig()) {
    return
  }

  const note = releaseNote.value.trim() || releaseNoteFromTag(tag)
  if (!taskRef.value.trim()) {
    taskRef.value = note
  }
  postReleaseHandledFor = null
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
      post_options: collectPostReleaseOptions(),
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
    :confirm-loading="saving || releasing || postProcessing"
    :confirm-disabled="releasing || foreignRelease || postProcessing"
    @confirm="onSubmit"
  >
    <div class="project-bulk-release__layout">
      <div class="project-bulk-release__left">
        <section class="project-bulk-release__panel-upper">
          <el-alert
            v-if="releasing"
            :title="releaseProgress || $t('projectAutoReleaseRunningTitle')"
            type="warning"
            :closable="false"
            show-icon
            class="project-bulk-release__running-alert"
          />
          <el-alert
            v-else-if="postProcessing"
            :title="$t('projectBulkAutoReleaseBuildStart')"
            type="info"
            :closable="false"
            show-icon
            class="project-bulk-release__running-alert"
          />
          <p class="project-bulk-release__scope">
            {{ $t('projectBulkAutoReleaseScope', { name: workspaceName, count: projectCount }) }}
            <el-tooltip
              :content="bulkReleaseTips"
              placement="top-start"
              :show-after="200"
              :popper-class="`${APP_DIALOG_POPPER_CLASS} form-label-tip__popper`"
            >
              <button
                type="button"
                class="project-bulk-release__scope-tip"
                :aria-label="bulkReleaseTips"
              >
                <el-icon :size="14"><InfoFilled /></el-icon>
              </button>
            </el-tooltip>
          </p>
          <el-form
            v-loading="defaultsLoading"
            label-width="108px"
            class="project-bulk-release__form"
            @submit.prevent="onSubmit"
          >
            <el-form-item :label="$t('projectAutoReleaseBranch')" required>
              <el-input
                v-model="releaseBranch"
                :placeholder="$t('projectAutoReleaseBranchPlaceholder')"
                autocomplete="off"
              />
            </el-form-item>
            <el-form-item required class="project-bulk-release__tag-item">
              <template #label>
                <FormLabelTip
                  :text="$t('projectAutoReleaseTag')"
                  :tip="$t('projectAutoReleaseTagHint')"
                />
              </template>
              <el-input
                v-model="tagBranch"
                :placeholder="$t('projectAutoReleaseTagPlaceholder')"
                autocomplete="off"
              />
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
            <el-form-item class="project-bulk-release__switch-item">
              <template #label>
                <FormLabelTip
                  :text="$t('projectAutoReleaseTargetBranch')"
                  :tip="$t('projectAutoReleaseTargetBranchHint')"
                />
              </template>
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
              </div>
            </el-form-item>
            <el-form-item class="project-bulk-release__switch-item">
              <template #label>
                <FormLabelTip
                  :text="$t('projectAutoReleaseUpdatePom')"
                  :tip="$t('projectAutoReleaseUpdatePomHint')"
                />
              </template>
              <el-switch v-model="updatePomVersion" />
            </el-form-item>
          </el-form>
        </section>

        <section
          v-loading="projectsLoading || loadingFtpEnvs"
          class="project-bulk-release__panel-lower"
        >
          <header class="project-bulk-release__panel-head">
            {{ $t('projectBulkAutoReleaseBuildFtpSection') }}
          </header>
          <el-form label-width="108px" class="project-bulk-release__form" @submit.prevent="onSubmit">
            <el-form-item
              class="project-bulk-release__switch-item project-bulk-release__switch-item--head"
              label-width="0"
            >
              <div class="project-bulk-release__switch-head">
                <FormLabelTip
                  :text="$t('projectBulkAutoReleaseAutoBuild')"
                  :tip="$t('projectBulkAutoReleaseAutoBuildHint')"
                />
                <el-switch v-model="autoBuildAfterRelease" />
              </div>
            </el-form-item>
            <el-form-item
              class="project-bulk-release__switch-item project-bulk-release__switch-item--head"
              label-width="0"
            >
              <div class="project-bulk-release__switch-head">
                <FormLabelTip
                  :text="$t('projectBulkAutoReleaseAutoFtp')"
                  :tip="$t('projectBulkAutoReleaseAutoFtpHint')"
                />
                <el-switch v-model="autoFtpPushAfterBuild" :disabled="!autoBuildAfterRelease" />
              </div>
            </el-form-item>
            <template v-if="ftpPushEnabled">
              <el-form-item required class="project-bulk-release__ftp-projects">
                <template #label>
                  <FormLabelTip
                    :text="$t('projectBulkAutoReleaseFtpProjects')"
                    :tip="$t('projectBulkAutoReleaseFtpProjectsHint')"
                  />
                </template>
                <div v-if="!workspaceProjects.length" class="project-bulk-release__ftp-projects-field">
                  <p class="project-bulk-release__hint">
                    {{ $t('projectBulkAutoReleaseFtpProjectsEmpty') }}
                  </p>
                </div>
                <div v-else class="project-bulk-release__ftp-projects-field">
                  <el-select
                    v-model="ftpPushPicker"
                    filterable
                    clearable
                    teleported
                    :disabled="!availableFtpPushOptions.length"
                    :popper-class="APP_DIALOG_POPPER_CLASS"
                    class="project-bulk-release__full"
                    :placeholder="$t('projectBulkAutoReleaseFtpProjectsAddPlaceholder')"
                    @change="onAddFtpPushProject"
                  >
                    <el-option
                      v-for="p in availableFtpPushOptions"
                      :key="p.rec_id"
                      :label="p.project_name"
                      :value="p.rec_id"
                    />
                  </el-select>
                  <ul
                    v-if="selectedFtpProjects.length"
                    class="project-bulk-release__ftp-selected-list"
                  >
                    <li
                      v-for="p in selectedFtpProjects"
                      :key="p.rec_id"
                      class="project-bulk-release__ftp-selected-item"
                    >
                      <span class="project-bulk-release__ftp-selected-name">{{ p.project_name }}</span>
                      <button
                        type="button"
                        class="project-bulk-release__ftp-selected-remove"
                        :aria-label="$t('delete')"
                        @click="removeFtpPushProject(p.rec_id)"
                      >
                        ×
                      </button>
                    </li>
                  </ul>
                  <p v-else class="project-bulk-release__hint">
                    {{ $t('projectBulkAutoReleaseFtpProjectsNone') }}
                  </p>
                </div>
              </el-form-item>
              <el-form-item :label="$t('projectFtpPushEnv')" required>
                <el-select
                  v-model="ftpEnvRecId"
                  filterable
                  teleported
                  :loading="loadingFtpEnvs"
                  :popper-class="APP_DIALOG_POPPER_CLASS"
                  class="project-bulk-release__full"
                  :placeholder="$t('projectFtpPushEnvPlaceholder')"
                  @change="onFtpEnvChange"
                >
                  <el-option
                    v-for="e in ftpEnvs"
                    :key="e.rec_id"
                    :label="ftpEnvOptionLabel(e)"
                    :value="e.rec_id"
                    :disabled="e.is_enabled !== 1"
                  />
                </el-select>
              </el-form-item>
              <el-form-item :label="$t('projectFtpPushPath')" required>
                <el-input
                  v-model="pushPath"
                  :placeholder="$t('projectFtpPushPathPlaceholder')"
                />
                <p class="project-bulk-release__hint">{{ $t('projectFtpPushPathHint') }}</p>
              </el-form-item>
              <el-form-item :label="$t('projectFtpPushBackupPath')">
                <el-input
                  v-model="backupPath"
                  :placeholder="defaultBackupPath || $t('projectFtpPushBackupPathPlaceholder')"
                  @input="backupManual = true"
                />
                <p class="project-bulk-release__hint">{{ $t('projectFtpPushBackupPathHint') }}</p>
              </el-form-item>
              <el-form-item :label="$t('projectFtpPushPusher')" required>
                <el-input v-model="pusher" :placeholder="$t('projectFtpPushPusherPlaceholder')" />
              </el-form-item>
              <el-form-item :label="$t('projectFtpPushTaskRef')" required>
                <el-input v-model="taskRef" :placeholder="$t('projectFtpPushTaskRefPlaceholder')" />
              </el-form-item>
              <el-form-item :label="$t('projectFtpPushDetail')">
                <el-input
                  v-model="detail"
                  type="textarea"
                  :rows="2"
                  :placeholder="$t('projectFtpPushDetailPlaceholder')"
                />
              </el-form-item>
              <el-form-item v-loading="remoteNoteLoading" :label="$t('projectFtpPushInfoPreview')">
                <el-input
                  :model-value="ftpPreviewContent"
                  type="textarea"
                  :rows="remoteNoteExists ? 6 : 3"
                  readonly
                  class="project-bulk-release__preview"
                />
                <p
                  v-if="remoteNoteExists"
                  class="project-bulk-release__hint project-bulk-release__hint--append"
                >
                  {{ $t('projectFtpPushNoteAppendPreview') }}
                </p>
                <p class="project-bulk-release__hint">{{ $t('projectFtpPushInfoFormatHint') }}</p>
              </el-form-item>
            </template>
          </el-form>
        </section>
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

  &__left {
    flex: 0 0 420px;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    overflow: hidden;
    background: var(--el-fill-color-blank);
  }

  &__panel-upper {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 12px 12px 4px;
  }

  &__panel-lower {
    flex: 0 0 42%;
    max-height: 48%;
    min-height: 200px;
    overflow-y: auto;
    border-top: 1px solid var(--el-border-color-lighter);
    padding: 8px 12px 12px;
    background: var(--el-fill-color-lighter);
  }

  &__panel-head {
    margin: 0 0 8px;
    font-size: 12px;
    font-weight: 600;
    color: var(--el-text-color-primary);
  }

  &__hint {
    margin: 4px 0 0;
    font-size: 12px;
    line-height: 1.45;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));

    &--append {
      color: var(--el-color-primary);
    }
  }

  &__preview :deep(textarea) {
    font-family: ui-monospace, monospace;
    font-size: 12px;
  }

  &__running-alert {
    margin-bottom: 12px;
  }

  &__scope {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.4;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__scope-tip {
    display: inline-flex;
    align-items: center;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--el-text-color-secondary);
    cursor: help;
    outline: none;
    border-radius: 4px;

    &:focus-visible {
      box-shadow: 0 0 0 2px var(--el-color-primary-light-7);
    }
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

  &__switch-item--head {
    margin-bottom: 10px;

    :deep(.el-form-item__label) {
      display: none;
      width: 0 !important;
      padding: 0;
    }

    :deep(.el-form-item__content) {
      margin-left: 0 !important;
      width: 100%;
      line-height: normal;
    }
  }

  &__switch-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    width: 100%;
    min-height: 32px;

    .form-label-tip {
      flex: 1;
      min-width: 0;
      font-size: 13px;
    }

    :deep(.el-switch) {
      flex-shrink: 0;
    }
  }

  &__panel-lower .project-bulk-release__form > .el-form-item:first-of-type {
    margin-top: 2px;
  }

  &__ftp-projects :deep(.el-form-item__label) {
    line-height: 1.4;
    align-self: flex-start;
    padding-top: 6px;
  }

  &__ftp-projects :deep(.el-form-item__content) {
    width: 100%;
  }

  &__ftp-projects-field {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
  }

  &__ftp-selected-list {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 96px;
    overflow-y: auto;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
    background: var(--el-fill-color-blank);
  }

  &__ftp-selected-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 8px;
    font-size: 13px;
    line-height: 1.4;

    & + & {
      border-top: 1px solid var(--el-border-color-extra-light);
    }
  }

  &__ftp-selected-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__ftp-selected-remove {
    flex-shrink: 0;
    width: 20px;
    height: 20px;
    padding: 0;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--el-text-color-secondary);
    font-size: 16px;
    line-height: 1;
    cursor: pointer;

    &:hover {
      color: var(--el-color-danger);
      background: var(--el-color-danger-light-9);
    }
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
