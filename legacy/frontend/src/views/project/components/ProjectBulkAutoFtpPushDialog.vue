<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import { useMessage } from '@/components/feedback/message'
import {
  appendProjectWorkspaceFtpUpdateNote,
  fetchProjectWorkspace,
  fetchProjectWorkspaceFtpPushStatus,
  fetchProjectWorkspaceFtpRemoteNote,
  pushProjectWorkspaceFtp,
  type WorkspaceFtpPushResult,
} from '@/request/api/build'
import { fetchEnvSettings } from '@/request/api/env'
import { fetchProjects } from '@/request/api/project'
import type { EnvSetting, PackProject } from '@/types'
import {
  backupFilePath,
  buildManifestPreviewPairs,
  defaultBackupDir,
  mergeUpdateNotePreview,
  resolvePushLayout,
} from '@/utils/workspace-ftp'
import {
  isVdatahubStaticDirName,
  isVdatahubStaticPath,
  pickTargetArtifact,
  VDATAHUB_STATIC_ROOT,
  vdatahubZipFileName,
} from '@/utils/workspace-artifact'
import {
  buildMergedFtpNotifyFields,
  type FtpNotifyEmailFields,
} from '@/utils/ftp-notify-email'
import { mergeBuildConfig } from '../project-defaults'
import { ProjectBuildLogPanel } from './panels'
import ProjectFtpNotifyEmailDialog from './ProjectFtpNotifyEmailDialog.vue'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  workspaceRecId: string
  workspaceName: string
  workspaceCode?: string
  projectCount: number
  envOptions?: (envType: string) => EnvSetting[]
}>()

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()
const message = useMessage()

interface PushTarget {
  project: PackProject
  path: string
  fileName: string
  resolving: boolean
}

interface PushSuccessItem {
  project: PackProject
  path: string
  fileName: string
  result: WorkspaceFtpPushResult
  pushPath: string
  taskRef: string
}

const workspaceProjects = ref<PackProject[]>([])
const projectsLoading = ref(false)
const projectPicker = ref('')
const targets = ref<PushTarget[]>([])

const ftpEnvs = ref<EnvSetting[]>([])
const loadingFtpEnvs = ref(false)
const ftpEnvRecId = ref('')
const pushPath = ref('')
const backupPath = ref('')
const backupManual = ref(false)
const pusher = ref('')
const taskRef = ref('')
const detail = ref('')
const mergeVdatahub = ref(false)

const remoteNoteContent = ref('')
const remoteNoteExists = ref(false)
const remoteNoteLoading = ref(false)
let remoteNoteTimer: ReturnType<typeof setTimeout> | null = null

const saving = ref(false)
const logText = ref('')
const successItems = ref<PushSuccessItem[]>([])

const notifyOpen = ref(false)
const notifyInitial = ref<FtpNotifyEmailFields | null>(null)

const ctxMenu = ref({ visible: false, x: 0, y: 0 })

let pushPollTimer: ReturnType<typeof setTimeout> | null = null

const targetIdSet = computed(() => new Set(targets.value.map((t) => t.project.rec_id)))

const availableProjectOptions = computed(() =>
  workspaceProjects.value.filter((p) => !targetIdSet.value.has(p.rec_id)),
)

const vdatahubTargets = computed(() =>
  targets.value.filter(
    (row) => isVdatahubStaticPath(row.path) || isVdatahubStaticDirName(row.fileName),
  ),
)

const canMergeVdatahub = computed(() => vdatahubTargets.value.length >= 2)

const mergeVdatahubActive = computed(() => canMergeVdatahub.value && mergeVdatahub.value)

const otherTargets = computed(() =>
  targets.value.filter((row) => !vdatahubTargets.value.some((v) => v.project.rec_id === row.project.rec_id && v.path === row.path)),
)

const previewFileName = computed(() => {
  if (mergeVdatahubActive.value) return vdatahubZipFileName()
  return targets.value[0]?.fileName || 'package.war'
})

const pushLayout = computed(() => resolvePushLayout(pushPath.value, previewFileName.value))

const defaultBackupPath = computed(() => {
  if (!pushPath.value.trim()) return ''
  return defaultBackupDir(pushPath.value, previewFileName.value)
})

const backupFilePathPreview = computed(() => {
  if (!pushPath.value.trim()) return ''
  return backupFilePath(pushPath.value, previewFileName.value, backupPath.value)
})

const previewPathPairs = computed(() => {
  const rows = mergeVdatahubActive.value
    ? [
        { fileName: vdatahubZipFileName(), path: vdatahubZipFileName() },
        ...otherTargets.value,
      ]
    : targets.value
  return rows.map((row) => {
    const name = row.fileName || row.path.split('/').pop() || 'package'
    const layout = resolvePushLayout(pushPath.value, name)
    return {
      backupPath: backupFilePath(pushPath.value, name, backupPath.value),
      updatePath: pushPath.value.trim() ? layout.updatePath : name,
    }
  })
})

const newManifestBlock = computed(() =>
  buildManifestPreviewPairs(
    new Date(),
    pusher.value,
    taskRef.value,
    previewPathPairs.value.length
      ? previewPathPairs.value
      : [
          {
            backupPath: backupFilePathPreview.value,
            updatePath: pushPath.value.trim() ? pushLayout.value.updatePath : previewFileName.value,
          },
        ],
    detail.value,
  ),
)

const previewContent = computed(() =>
  mergeUpdateNotePreview(remoteNoteContent.value, newManifestBlock.value),
)

const logComplete = computed(() => !saving.value)

watch(open, (v) => {
  if (!v) {
    hideCtxMenu()
    return
  }
  resetForm()
  Promise.all([loadWorkspaceProjects(), loadFtpEnvs()]).catch(() => undefined)
})

watch(canMergeVdatahub, (ok) => {
  if (!open.value) return
  if (ok && !mergeVdatahub.value) mergeVdatahub.value = true
  if (!ok) mergeVdatahub.value = false
})

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

watch([ftpEnvRecId, pushPath, previewFileName], () => {
  scheduleRemoteNoteLoad()
})

onUnmounted(() => {
  if (remoteNoteTimer) clearTimeout(remoteNoteTimer)
  stopPushPolling()
  hideCtxMenu()
})

function resetForm(): void {
  projectPicker.value = ''
  targets.value = []
  ftpEnvRecId.value = ''
  pushPath.value = ''
  backupPath.value = ''
  backupManual.value = false
  pusher.value = ''
  taskRef.value = ''
  detail.value = ''
  mergeVdatahub.value = false
  remoteNoteContent.value = ''
  remoteNoteExists.value = false
  logText.value = t('projectBulkAutoFtpLogReady')
  successItems.value = []
  hideCtxMenu()
}

async function loadWorkspaceProjects(): Promise<void> {
  if (!props.workspaceRecId) {
    workspaceProjects.value = []
    return
  }
  projectsLoading.value = true
  try {
    const res = await fetchProjects({ workspace_rec_id: props.workspaceRecId })
    workspaceProjects.value = (res.items ?? []).filter((p) => p.is_enabled !== 0)
  } catch {
    workspaceProjects.value = []
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

function ftpEnvOptionLabel(item: EnvSetting): string {
  if (item.is_enabled === 1) return item.env_name
  return `${item.env_name} (${t('disabled')})`
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

async function resolveArtifactPath(project: PackProject): Promise<{ path: string; fileName: string } | null> {
  const cfg = mergeBuildConfig(project.project_type, project.config_json)
  const dir = (cfg.output_dir || 'target').trim() || 'target'
  try {
    const listing = await fetchProjectWorkspace(project.rec_id, dir)
    const entry = pickTargetArtifact(listing.entries ?? [])
    if (!entry) return null
    return { path: entry.path, fileName: entry.name }
  } catch {
    return null
  }
}

async function onAddProject(recId: string | undefined): Promise<void> {
  if (!recId || targetIdSet.value.has(recId)) {
    projectPicker.value = ''
    return
  }
  const project = workspaceProjects.value.find((p) => p.rec_id === recId)
  projectPicker.value = ''
  if (!project) return

  if (targets.value.some((t) => t.project.rec_id === recId)) return

  const row: PushTarget = {
    project,
    path: '',
    fileName: '',
    resolving: true,
  }
  targets.value = [...targets.value, row]
  const found = await resolveArtifactPath(project)
  const idx = targets.value.findIndex((t) => t.project.rec_id === recId)
  if (idx < 0) return
  if (found) {
    targets.value[idx] = { ...targets.value[idx], path: found.path, fileName: found.fileName, resolving: false }
  } else {
    const cfg = mergeBuildConfig(project.project_type, project.config_json)
    const dir = (cfg.output_dir || 'target').trim() || 'target'
    targets.value[idx] = { ...targets.value[idx], path: dir, fileName: '', resolving: false }
    message.warning(
      t('projectBulkAutoFtpArtifactPending', { name: project.project_name, dir }),
    )
  }
}

function removeTarget(recId: string): void {
  targets.value = targets.value.filter((t) => t.project.rec_id !== recId)
}

async function redetectTarget(recId: string): Promise<void> {
  const idx = targets.value.findIndex((t) => t.project.rec_id === recId)
  if (idx < 0) return
  targets.value[idx] = { ...targets.value[idx], resolving: true }
  const found = await resolveArtifactPath(targets.value[idx].project)
  if (idx >= targets.value.length || targets.value[idx].project.rec_id !== recId) return
  if (found) {
    targets.value[idx] = { ...targets.value[idx], path: found.path, fileName: found.fileName, resolving: false }
  } else {
    targets.value[idx] = { ...targets.value[idx], resolving: false }
    message.warning(t('projectBulkAutoReleaseArtifactNotFound', {
      name: targets.value[idx].project.project_name,
      dir: mergeBuildConfig(targets.value[idx].project.project_type, targets.value[idx].project.config_json).output_dir || 'target',
    }))
  }
}

function onTargetPathInput(recId: string, pathRaw: string): void {
  const idx = targets.value.findIndex((t) => t.project.rec_id === recId)
  if (idx < 0) return
  const path = pathRaw.trim().replaceAll('\\', '/')
  const fileName = path.split('/').pop() ?? ''
  targets.value[idx] = { ...targets.value[idx], path, fileName }
}

function scheduleRemoteNoteLoad(): void {
  if (remoteNoteTimer) clearTimeout(remoteNoteTimer)
  remoteNoteTimer = setTimeout(() => {
    void loadRemoteNote()
  }, 400)
}

function isLikelyFtpPushPath(path: string): boolean {
  const p = path.trim()
  if (!p || p.length > 512) return false
  if (p.includes('\n') || p.includes('\r') || p.includes(' ')) return false
  // Reject console/error dumps accidentally written into the path input.
  if (p.includes('Unhandled error') || p.includes('Message compilation')) return false
  if (!p.startsWith('/')) return false
  return true
}

async function loadRemoteNote(): Promise<void> {
  const projectRecId = targets.value[0]?.project.rec_id
  const path = pushPath.value.trim()
  if (!projectRecId || !ftpEnvRecId.value || !isLikelyFtpPushPath(path)) {
    remoteNoteContent.value = ''
    remoteNoteExists.value = false
    return
  }
  remoteNoteLoading.value = true
  try {
    const res = await fetchProjectWorkspaceFtpRemoteNote(projectRecId, {
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: path,
      file_name: previewFileName.value,
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

function formatLogTime(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function appendLog(lines: string[]): void {
  const chunk = lines.join('\n')
  logText.value = logText.value ? `${logText.value.trimEnd()}\n${chunk}` : chunk
}

function stopPushPolling(): void {
  if (pushPollTimer) {
    clearTimeout(pushPollTimer)
    pushPollTimer = null
  }
}

function validate(): boolean {
  if (targets.value.length === 0) {
    message.warning(t('projectBulkAutoFtpProjectsRequired'))
    return false
  }
  for (const row of targets.value) {
    if (!row.path.trim()) {
      message.warning(t('projectBulkAutoFtpPathRequired', { name: row.project.project_name }))
      return false
    }
  }
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
  return true
}

async function waitForFtpPushTask(
  projectRecId: string,
  taskId: string,
): Promise<WorkspaceFtpPushResult> {
  for (;;) {
    const st = await fetchProjectWorkspaceFtpPushStatus(projectRecId, taskId)
    if (st.log_content) logText.value = st.log_content
    if (st.status === 'success' && st.result) return st.result
    if (st.status === 'failed') throw new Error(st.message || t('projectFtpPushLogFailed'))
    await new Promise<void>((resolve) => {
      pushPollTimer = setTimeout(() => resolve(), 800)
    })
  }
}

async function pushMergedVdatahub(rows: PushTarget[]): Promise<PushSuccessItem | null> {
  const anchor = rows[0]
  const envName = ftpEnvs.value.find((e) => e.rec_id === ftpEnvRecId.value)?.env_name ?? ftpEnvRecId.value
  const fileName = vdatahubZipFileName()
  appendLog([
    `[${formatLogTime()}] ${t('projectFtpPushMergeVdatahubTarget', { count: rows.length })}`,
    `  ${t('projectFtpPushLogEnv', { name: envName })}`,
    `  ${t('projectFtpPushLogPushPath', { path: pushPath.value.trim() })}`,
    `  ${t('projectFtpPushLogUploading')}`,
  ])
  try {
    const sameProjectPaths = rows.filter((r) => r.project.rec_id === anchor.project.rec_id).map((r) => r.path)
    const start = await pushProjectWorkspaceFtp(anchor.project.rec_id, {
      path: anchor.path,
      paths: sameProjectPaths,
      merge_zip_root: VDATAHUB_STATIC_ROOT,
      merge_sources: rows
        .filter((r) => r.project.rec_id !== anchor.project.rec_id)
        .map((r) => ({ project_rec_id: r.project.rec_id, path: r.path })),
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      backup_path: backupPath.value.trim() || undefined,
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim(),
      skip_update_note: true,
    })
    const result = await waitForFtpPushTask(anchor.project.rec_id, start.task_id)
    const plannedBackup = backupFilePath(pushPath.value.trim(), fileName, backupPath.value)
    appendLog([
      `[${formatLogTime()}] ${t('projectFtpPushLogSuccess')}`,
      `  ${t('projectFtpPushLogBackupPath', { path: result.backup_path || plannedBackup || '—' })}`,
      `  ${t('projectFtpPushLogUpdatePath', { path: result.update_path })}`,
      `  ${t('projectBulkAutoFtpNoteDeferred')}`,
    ])
    return {
      project: anchor.project,
      path: fileName,
      fileName,
      result: {
        ...result,
        backup_path: result.backup_path || plannedBackup,
      },
      pushPath: pushPath.value.trim(),
      taskRef: taskRef.value.trim(),
    }
  } catch (e) {
    appendLog([
      `[${formatLogTime()}] ${t('projectFtpPushLogFailed')}`,
      `  ${(e as Error).message}`,
    ])
    return null
  }
}

async function pushOne(row: PushTarget): Promise<PushSuccessItem | null> {
  const envName = ftpEnvs.value.find((e) => e.rec_id === ftpEnvRecId.value)?.env_name ?? ftpEnvRecId.value
  const fileLabel = row.fileName || row.path
  appendLog([
    `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpProject', {
      name: row.project.project_name,
      file: fileLabel,
    })}`,
    `  ${t('projectFtpPushLogEnv', { name: envName })}`,
    `  ${t('projectFtpPushLogPushPath', { path: pushPath.value.trim() })}`,
    `  ${t('projectFtpPushLogUploading')}`,
  ])
  try {
    const start = await pushProjectWorkspaceFtp(row.project.rec_id, {
      path: row.path,
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      backup_path: backupPath.value.trim() || undefined,
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim(),
      skip_update_note: true,
    })
    const result = await waitForFtpPushTask(row.project.rec_id, start.task_id)
    const plannedBackup = backupFilePath(
      pushPath.value.trim(),
      row.fileName || fileLabel,
      backupPath.value,
    )
    appendLog([
      `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpOk', { name: row.project.project_name })}`,
      `  ${t('projectFtpPushLogBackupPath', { path: result.backup_path || plannedBackup || '—' })}`,
      `  ${t('projectFtpPushLogUpdatePath', { path: result.update_path })}`,
      `  ${t('projectBulkAutoFtpNoteDeferred')}`,
    ])
    return {
      project: row.project,
      path: row.path,
      fileName: row.fileName || fileLabel,
      result: {
        ...result,
        backup_path: result.backup_path || plannedBackup,
      },
      pushPath: pushPath.value.trim(),
      taskRef: taskRef.value.trim(),
    }
  } catch (e) {
    appendLog([
      `[${formatLogTime()}] ${t('projectBulkAutoReleaseFtpFailed', {
        name: row.project.project_name,
        msg: (e as Error).message,
      })}`,
    ])
    return null
  }
}

async function writeMergedUpdateNote(items: PushSuccessItem[], mergedDetail: string): Promise<void> {
  if (!items.length) return
  const anchor = items[0]
  appendLog([`[${formatLogTime()}] ${t('projectBulkAutoFtpNoteMergeStart', { count: items.length })}`])
  try {
    const res = await appendProjectWorkspaceFtpUpdateNote(anchor.project.rec_id, {
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim(),
      detail: mergedDetail || undefined,
      paths: items.map((item) => ({
        backup_path:
          item.result.backup_path ||
          backupFilePath(item.pushPath, item.fileName, backupPath.value),
        update_path: item.result.update_path,
      })),
    })
    appendLog([
      `[${formatLogTime()}] ${t('projectBulkAutoFtpNoteMergeOk')}`,
      `  ${t('projectFtpPushLogInfoPath', { path: res.info_file_path })}`,
    ])
  } catch (e) {
    appendLog([
      `[${formatLogTime()}] ${t('projectBulkAutoFtpNoteMergeFailed', {
        msg: (e as Error).message,
      })}`,
    ])
    throw e
  }
}

async function onSubmit(): Promise<void> {
  if (!validate() || saving.value) return
  saving.value = true
  successItems.value = []
  hideCtxMenu()
  logText.value = `[${formatLogTime()}] ${t('projectBulkAutoFtpStart')}`
  try {
    const mergeList = mergeVdatahubActive.value ? vdatahubTargets.value : []
    const rest = mergeVdatahubActive.value ? otherTargets.value : targets.value
    const ok: PushSuccessItem[] = []
    if (mergeList.length) {
      const item = await pushMergedVdatahub(mergeList)
      if (item) ok.push(item)
    }
    for (const row of rest) {
      const item = await pushOne(row)
      if (item) ok.push(item)
    }
    successItems.value = ok
    if (ok.length) {
      try {
        await writeMergedUpdateNote(ok, detail.value.trim())
      } catch {
        // packages already uploaded; keep success list for notify
      }
    }
    appendLog([
      `[${formatLogTime()}] ${t('projectBulkAutoFtpDone', {
        succeeded: ok.length,
        failed: (mergeList.length ? 1 : 0) + rest.length - ok.length,
      })}`,
    ])
    if (ok.length) {
      message.success(t('projectBulkAutoFtpSuccess', { count: ok.length }))
      emit('success')
    } else {
      message.error(t('projectBulkAutoFtpAllFailed'))
    }
  } finally {
    saving.value = false
    stopPushPolling()
  }
}

function hideCtxMenu(): void {
  ctxMenu.value = { visible: false, x: 0, y: 0 }
}

function onSuccessContextMenu(e: MouseEvent): void {
  e.preventDefault()
  e.stopPropagation()
  if (!successItems.value.length) return
  ctxMenu.value = {
    visible: true,
    x: e.clientX,
    y: e.clientY,
  }
}

function openNotifyFromCtx(): void {
  hideCtxMenu()
  openNotify()
}

async function openNotify(): Promise<void> {
  const fields = buildMergedFtpNotifyFields(
    successItems.value.map((item) => ({
      projectName: item.project.project_name,
      projectVersion: item.project.project_version,
      projectType: String(item.project.project_type),
      fileName: item.fileName,
      path: item.path,
      pushPath: item.pushPath,
      taskRef: item.taskRef,
      backupPath:
        item.result.backup_path?.trim() ||
        backupFilePath(item.pushPath, item.fileName, ''),
    })),
    {
      workspaceCode: props.workspaceCode,
      workspaceName: props.workspaceName,
    },
  )
  if (!fields) {
    message.warning(t('projectBulkAutoFtpNotifyEmpty'))
    return
  }
  hideCtxMenu()
  notifyInitial.value = fields
  // Open nested dialog after parent patch settles (avoids ElSelect/ElDialog vnode crash).
  await nextTick()
  notifyOpen.value = true
}

function onWindowClick(): void {
  if (ctxMenu.value.visible) hideCtxMenu()
}

watch(
  () => ctxMenu.value.visible,
  (v) => {
    if (v) {
      window.addEventListener('click', onWindowClick)
      window.addEventListener('scroll', hideCtxMenu, true)
    } else {
      window.removeEventListener('click', onWindowClick)
      window.removeEventListener('scroll', hideCtxMenu, true)
    }
  },
)
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectBulkAutoFtpTitle')"
    width="1000px"
    :show-footer="true"
    :confirm-text="$t('projectBulkAutoFtpExecute')"
    :confirm-loading="saving"
    :confirm-disabled="saving"
    @confirm="onSubmit"
  >
    <div class="auto-ftp__layout">
      <div v-loading="projectsLoading || loadingFtpEnvs" class="auto-ftp__left">
        <p class="auto-ftp__scope">
          {{ $t('projectBulkAutoFtpScope', { name: workspaceName, count: projectCount }) }}
        </p>
        <p class="auto-ftp__hint">{{ $t('projectBulkAutoFtpHint') }}</p>

        <el-form label-width="108px" class="auto-ftp__form" @submit.prevent="onSubmit">
          <el-form-item :label="$t('projectBulkAutoFtpProjects')" required>
            <div class="auto-ftp__projects">
              <el-select
                v-model="projectPicker"
                filterable
                clearable
                teleported
                :disabled="!availableProjectOptions.length || saving"
                :popper-class="APP_DIALOG_POPPER_CLASS"
                class="auto-ftp__full"
                :placeholder="$t('projectBulkAutoFtpProjectsAddPlaceholder')"
                @change="onAddProject"
              >
                <el-option
                  v-for="p in availableProjectOptions"
                  :key="p.rec_id"
                  :label="p.project_name"
                  :value="p.rec_id"
                />
              </el-select>
              <ul v-if="targets.length" class="auto-ftp__target-list">
                <li v-for="row in targets" :key="row.project.rec_id" class="auto-ftp__target-item">
                  <div class="auto-ftp__target-head">
                    <span class="auto-ftp__target-name">{{ row.project.project_name }}</span>
                    <div class="auto-ftp__target-actions">
                      <el-button
                        link
                        type="primary"
                        size="small"
                        :loading="row.resolving"
                        :disabled="saving"
                        @click="redetectTarget(row.project.rec_id)"
                      >
                        {{ $t('projectBulkAutoFtpRedetect') }}
                      </el-button>
                      <button
                        type="button"
                        class="auto-ftp__target-remove"
                        :aria-label="$t('delete')"
                        :disabled="saving"
                        @click="removeTarget(row.project.rec_id)"
                      >
                        ×
                      </button>
                    </div>
                  </div>
                  <el-input
                    :model-value="row.path"
                    size="small"
                    :disabled="saving || row.resolving"
                    :placeholder="$t('projectBulkAutoFtpPathPlaceholder')"
                    @update:model-value="onTargetPathInput(row.project.rec_id, $event)"
                  />
                </li>
              </ul>
              <p v-else class="auto-ftp__hint">{{ $t('projectBulkAutoFtpProjectsNone') }}</p>
            </div>
            <el-checkbox
              v-if="canMergeVdatahub"
              v-model="mergeVdatahub"
              class="auto-ftp__merge"
              :disabled="saving"
            >
              {{ $t('projectFtpPushMergeVdatahub') }}
            </el-checkbox>
            <p v-if="canMergeVdatahub" class="auto-ftp__hint">
              {{ $t('projectFtpPushMergeVdatahubHint') }}
            </p>
          </el-form-item>

          <el-form-item :label="$t('projectFtpPushEnv')" required>
            <el-select
              v-model="ftpEnvRecId"
              filterable
              teleported
              :disabled="saving"
              :loading="loadingFtpEnvs"
              :popper-class="APP_DIALOG_POPPER_CLASS"
              class="auto-ftp__full"
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
              :disabled="saving"
              :placeholder="$t('projectFtpPushPathPlaceholder')"
            />
            <p class="auto-ftp__hint">{{ $t('projectFtpPushPathHint') }}</p>
          </el-form-item>
          <el-form-item :label="$t('projectFtpPushBackupPath')">
            <el-input
              v-model="backupPath"
              :disabled="saving"
              :placeholder="defaultBackupPath || $t('projectFtpPushBackupPathPlaceholder')"
              @input="backupManual = true"
            />
          </el-form-item>
          <el-form-item :label="$t('projectFtpPushPusher')" required>
            <el-input
              v-model="pusher"
              :disabled="saving"
              :placeholder="$t('projectFtpPushPusherPlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="$t('projectFtpPushTaskRef')" required>
            <el-input
              v-model="taskRef"
              :disabled="saving"
              :placeholder="$t('projectFtpPushTaskRefPlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="$t('projectFtpPushDetail')">
            <el-input
              v-model="detail"
              type="textarea"
              :rows="2"
              :disabled="saving"
              :placeholder="$t('projectFtpPushDetailPlaceholder')"
            />
          </el-form-item>
          <el-form-item v-loading="remoteNoteLoading" :label="$t('projectFtpPushInfoPreview')">
            <el-input
              :model-value="previewContent"
              type="textarea"
              :rows="remoteNoteExists ? 6 : 3"
              readonly
            />
          </el-form-item>
        </el-form>
      </div>

      <aside class="auto-ftp__right">
        <ProjectBuildLogPanel
          class="auto-ftp__log-console"
          :content="logText"
          :loading="saving && !logText"
          :complete="logComplete"
          :console-label="$t('projectBulkAutoFtpLog')"
          :empty-description="$t('projectBulkAutoFtpLogReady')"
        />
        <section
          v-if="successItems.length"
          class="auto-ftp__success"
          @contextmenu="onSuccessContextMenu"
        >
          <header class="auto-ftp__success-head">
            <div class="auto-ftp__success-title">
              {{ $t('projectBulkAutoFtpSuccessList') }}
              <span class="auto-ftp__success-tip">{{ $t('projectBulkAutoFtpSuccessTip') }}</span>
            </div>
            <el-button type="primary" link size="small" @click="openNotify()">
              {{ $t('projectFtpNotifyAction') }}
            </el-button>
          </header>
          <ul class="auto-ftp__success-list">
            <li
              v-for="item in successItems"
              :key="item.project.rec_id + item.result.update_path"
              class="auto-ftp__success-item"
            >
              <div class="auto-ftp__success-main">
                <strong>{{ item.project.project_name }}</strong>
                <span>{{ item.fileName || item.path }}</span>
              </div>
            </li>
          </ul>
        </section>
      </aside>
    </div>

  </AppDialog>

  <Teleport to="body">
    <ul
      v-if="ctxMenu.visible"
      class="auto-ftp__ctx"
      :style="{ left: `${ctxMenu.x}px`, top: `${ctxMenu.y}px` }"
      @click.stop
    >
      <li class="auto-ftp__ctx-item" @click="openNotifyFromCtx">
        {{ $t('projectFtpNotifyAction') }}
      </li>
    </ul>
  </Teleport>

  <ProjectFtpNotifyEmailDialog
    v-model:open="notifyOpen"
    :initial="notifyInitial"
    :env-options="envOptions"
  />
</template>

<style scoped lang="scss">
.auto-ftp {
  &__layout {
    display: flex;
    gap: 16px;
    min-height: 420px;
    height: 100%;
    align-items: stretch;
  }

  &__left {
    flex: 0 0 440px;
    min-width: 0;
    min-height: 0;
    overflow-y: auto;
    padding: 4px 4px 8px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    background: var(--el-fill-color-blank);
  }

  &__right {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  &__scope {
    margin: 8px 12px 4px;
    font-size: 13px;
    color: var(--el-text-color-primary);
  }

  &__hint {
    margin: 4px 12px 8px;
    font-size: 12px;
    line-height: 1.45;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
  }

  &__form {
    padding: 0 8px 8px;
  }

  &__full {
    width: 100%;
  }

  &__projects {
    width: 100%;
  }

  &__merge {
    margin-top: 8px;
    font-size: 13px;
  }

  &__target-list {
    list-style: none;
    margin: 8px 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  &__target-item {
    padding: 8px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
    background: var(--el-fill-color-lighter);
  }

  &__target-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 6px;
  }

  &__target-name {
    font-size: 13px;
    font-weight: 600;
  }

  &__target-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  &__target-remove {
    border: 0;
    background: transparent;
    cursor: pointer;
    font-size: 16px;
    line-height: 1;
    color: var(--el-text-color-secondary);
    padding: 0 4px;
  }

  &__success {
    flex: 0 0 auto;
    max-height: 180px;
    overflow: auto;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    background: var(--el-fill-color-blank);
    padding: 8px 10px;
    cursor: context-menu;
  }

  &__success-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 6px;
    font-size: 12px;
    font-weight: 600;
  }

  &__success-title {
    display: flex;
    align-items: baseline;
    gap: 8px;
    min-width: 0;
  }

  &__success-tip {
    font-weight: 400;
    color: var(--el-text-color-secondary);
  }

  &__success-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  &__success-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 4px;
    border-radius: 4px;
  }

  &__success-main {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-size: 12px;

    span {
      color: var(--el-text-color-secondary);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  &__ctx {
    position: fixed;
    z-index: 4000;
    margin: 0;
    padding: 4px 0;
    list-style: none;
    min-width: 140px;
    border-radius: 6px;
    border: 1px solid var(--el-border-color-light);
    background: var(--el-bg-color-overlay);
    box-shadow: var(--el-box-shadow-light);
  }

  &__ctx-item {
    padding: 8px 14px;
    font-size: 13px;
    cursor: pointer;

    &:hover {
      background: var(--el-fill-color-light);
      color: var(--el-color-primary);
    }
  }
}
</style>
