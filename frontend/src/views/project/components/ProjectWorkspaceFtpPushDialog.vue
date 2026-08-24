<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import { fetchEnvSettings } from '@/request/api/env'
import {
  appendProjectWorkspaceFtpUpdateNote,
  fetchProjectWorkspaceFtpPushStatus,
  fetchProjectWorkspaceFtpRemoteNote,
  pushProjectWorkspaceFtp,
  type WorkspaceFtpPushResult,
} from '@/request/api/build'
import type { EnvSetting, PackBuildStatus, ProjectWorkspaceEntry } from '@/types'
import {
  backupFilePath,
  buildManifestPreviewPairs,
  defaultBackupDir,
  mergeUpdateNotePreview,
  resolvePushLayout,
} from '@/utils/workspace-ftp'
import {
  isVdatahubStaticDirName,
  VDATAHUB_STATIC_ROOT,
  vdatahubZipFileName,
} from '@/utils/workspace-artifact'
import {
  buildMergedFtpNotifyFields,
  type FtpNotifyEmailFields,
} from '@/utils/ftp-notify-email'
import { ProjectBuildLogPanel } from './panels'
import ProjectFtpNotifyEmailDialog from './ProjectFtpNotifyEmailDialog.vue'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  projectRecId: string
  entries: ProjectWorkspaceEntry[]
  projectName?: string
  projectVersion?: string
  projectType?: string
  workspaceCode?: string
  workspaceName?: string
}>()

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()
const message = useMessage()

interface PushSuccessItem {
  entry: ProjectWorkspaceEntry
  fileName: string
  result: WorkspaceFtpPushResult
  pushPath: string
  taskRef: string
}

const ftpEnvs = ref<EnvSetting[]>([])
const loadingEnvs = ref(false)
const saving = ref(false)
const logText = ref('')
const pushStatus = ref<PackBuildStatus | null>(null)
const successItems = ref<PushSuccessItem[]>([])

const notifyOpen = ref(false)
const notifyInitial = ref<FtpNotifyEmailFields | null>(null)
const ctxMenu = ref({ visible: false, x: 0, y: 0 })

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
let pushPollTimer: ReturnType<typeof setTimeout> | null = null

const activeEntries = computed(() => props.entries.filter(Boolean))

const isMulti = computed(() => activeEntries.value.length > 1)

const vdatahubEntries = computed(() =>
  activeEntries.value.filter((e) => e.is_dir && isVdatahubStaticDirName(e.name)),
)

const canMergeVdatahub = computed(() => vdatahubEntries.value.length >= 2)

const mergeVdatahubActive = computed(() => canMergeVdatahub.value && mergeVdatahub.value)

const otherEntries = computed(() =>
  activeEntries.value.filter((e) => !vdatahubEntries.value.some((v) => v.path === e.path)),
)

const entryLabel = computed(() => {
  const list = activeEntries.value
  if (!list.length) return ''
  if (mergeVdatahubActive.value) {
    return t('projectFtpPushMergeVdatahubTarget', { count: vdatahubEntries.value.length })
  }
  if (list.length === 1) {
    const e = list[0]
    return e.is_dir ? `${e.name}/` : e.name
  }
  return t('projectFtpPushTargetMulti', { count: list.length })
})

const uploadNames = computed(() => {
  if (mergeVdatahubActive.value) {
    return [vdatahubZipFileName(), ...otherEntries.value.map((e) => (e.is_dir ? `${e.name}.zip` : e.name))]
  }
  return activeEntries.value.map((e) => (e.is_dir ? `${e.name}.zip` : e.name))
})

const previewFileName = computed(() => uploadNames.value[0] || 'package')

const pushLayout = computed(() => resolvePushLayout(pushPath.value, previewFileName.value))

const defaultBackupPath = computed(() => {
  if (!pushPath.value.trim()) return ''
  return defaultBackupDir(pushPath.value, previewFileName.value)
})

const previewPathPairs = computed(() =>
  uploadNames.value.map((name) => {
    const layout = resolvePushLayout(pushPath.value, name)
    return {
      backupPath: backupFilePath(pushPath.value, name, backupPath.value),
      updatePath: pushPath.value.trim() ? layout.updatePath : name,
    }
  }),
)

const newManifestBlock = computed(() =>
  buildManifestPreviewPairs(
    new Date(),
    pusher.value,
    taskRef.value,
    previewPathPairs.value.length
      ? previewPathPairs.value
      : [
          {
            backupPath: backupFilePath(pushPath.value, previewFileName.value, backupPath.value),
            updatePath: pushPath.value.trim() ? pushLayout.value.updatePath : previewFileName.value,
          },
        ],
    detail.value,
  ),
)

const previewContent = computed(() =>
  mergeUpdateNotePreview(remoteNoteContent.value, newManifestBlock.value),
)

const previewRows = computed(() => (remoteNoteExists.value || isMulti.value ? 8 : 4))

const logComplete = computed(() => pushStatus.value !== 'running')
const logLoading = computed(() => saving.value && pushStatus.value === 'running' && !logText.value)

watch(open, (v) => {
  if (!v) {
    hideCtxMenu()
    return
  }
  resetForm()
  mergeVdatahub.value = canMergeVdatahub.value
  void loadFtpEnvs()
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

function scheduleRemoteNoteLoad(): void {
  if (remoteNoteTimer) clearTimeout(remoteNoteTimer)
  remoteNoteTimer = setTimeout(() => {
    void loadRemoteNote()
  }, 400)
}

async function loadRemoteNote(): Promise<void> {
  if (!props.projectRecId || !ftpEnvRecId.value || !pushPath.value.trim()) {
    remoteNoteContent.value = ''
    remoteNoteExists.value = false
    return
  }
  remoteNoteLoading.value = true
  try {
    const res = await fetchProjectWorkspaceFtpRemoteNote(props.projectRecId, {
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
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

function resetForm(): void {
  ftpEnvRecId.value = ''
  pushPath.value = ''
  backupPath.value = ''
  backupManual.value = false
  pusher.value = ''
  taskRef.value = ''
  detail.value = ''
  remoteNoteContent.value = ''
  remoteNoteExists.value = false
  logText.value = t('projectFtpPushLogReady')
  pushStatus.value = null
  successItems.value = []
  notifyOpen.value = false
  notifyInitial.value = null
  mergeVdatahub.value = false
  hideCtxMenu()
}

function stopPushPolling(): void {
  if (pushPollTimer) {
    clearTimeout(pushPollTimer)
    pushPollTimer = null
  }
}

function formatLogTime(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function appendLog(lines: string[]): void {
  const block = lines.join('\n')
  logText.value = logText.value ? `${logText.value}\n${block}` : block
}

function asEnvList(raw: EnvSetting[] | null | undefined): EnvSetting[] {
  return Array.isArray(raw) ? raw : []
}

function ftpEnvOptionLabel(item: EnvSetting): string {
  if (item.is_enabled === 1) return item.env_name
  return `${item.env_name} (${t('disabled')})`
}

function entryUploadName(entry: ProjectWorkspaceEntry): string {
  return entry.is_dir ? `${entry.name}.zip` : entry.name
}

function entryDisplayLabel(entry: ProjectWorkspaceEntry): string {
  return entry.is_dir ? `${entry.name}/` : entry.name
}

async function loadFtpEnvs(): Promise<void> {
  loadingEnvs.value = true
  try {
    const res = await fetchEnvSettings('ftp')
    ftpEnvs.value = asEnvList(res.items)
  } catch (e) {
    message.error((e as Error).message)
    ftpEnvs.value = []
  } finally {
    loadingEnvs.value = false
  }
}

function onEnvSelectVisible(visible: boolean): void {
  if (visible) void loadFtpEnvs()
}

function onEnvChange(recId: string): void {
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

function validate(): boolean {
  if (!props.projectRecId || !activeEntries.value.length) return false
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

async function waitForFtpPushTask(taskId: string): Promise<WorkspaceFtpPushResult> {
  for (;;) {
    const st = await fetchProjectWorkspaceFtpPushStatus(props.projectRecId, taskId)
    logText.value = st.log_content?.trimEnd() || logText.value
    if (st.status === 'running' || st.status === 'idle') {
      pushStatus.value = 'running'
      await new Promise<void>((resolve) => {
        pushPollTimer = setTimeout(() => {
          pushPollTimer = null
          resolve()
        }, 1000)
      })
      continue
    }
    if (st.status === 'success' && st.result) return st.result
    pushStatus.value = 'failed'
    throw new Error(st.message || t('projectFtpPushLogFailed'))
  }
}

async function pushMergedVdatahub(
  entries: ProjectWorkspaceEntry[],
  deferNote: boolean,
): Promise<PushSuccessItem | null> {
  const fileName = vdatahubZipFileName()
  const envName = ftpEnvs.value.find((e) => e.rec_id === ftpEnvRecId.value)?.env_name ?? ftpEnvRecId.value
  const noteHint: string[] = []
  if (deferNote) noteHint.push(`  ${t('projectBulkAutoFtpNoteDeferred')}`)
  else if (remoteNoteExists.value) noteHint.push(`  ${t('projectFtpPushLogNoteAppend')}`)

  appendLog([
    `[${formatLogTime()}] ${t('projectFtpPushLogStart')}`,
    `  ${t('projectFtpPushMergeVdatahubTarget', { count: entries.length })}`,
    `  ${t('projectFtpPushLogEnv', { name: envName })}`,
    `  ${t('projectFtpPushLogPushPath', { path: pushPath.value.trim() })}`,
    ...noteHint,
  ])
  try {
    const start = await pushProjectWorkspaceFtp(props.projectRecId, {
      path: entries[0].path,
      paths: entries.map((e) => e.path),
      merge_zip_root: VDATAHUB_STATIC_ROOT,
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      backup_path: backupPath.value.trim() || undefined,
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim(),
      detail: deferNote ? undefined : detail.value.trim() || undefined,
      skip_update_note: deferNote || undefined,
    })
    const result = await waitForFtpPushTask(start.task_id)
    const plannedBackup = backupFilePath(pushPath.value.trim(), fileName, backupPath.value)
    const successLines = [
      `[${formatLogTime()}] ${t('projectFtpPushLogSuccess')}`,
      `  ${t('projectFtpPushLogBackupPath', { path: result.backup_path || plannedBackup || '—' })}`,
      `  ${t('projectFtpPushLogUpdatePath', { path: result.update_path })}`,
    ]
    if (!deferNote && result.info_file_path) {
      successLines.push(`  ${t('projectFtpPushLogInfoPath', { path: result.info_file_path })}`)
    }
    appendLog(successLines)
    return {
      entry: {
        name: VDATAHUB_STATIC_ROOT,
        path: entries[0].path,
        is_dir: true,
        size: 0,
      },
      fileName,
      result: {
        ...result,
        backup_path: result.backup_path || plannedBackup,
      },
      pushPath: pushPath.value.trim(),
      taskRef: taskRef.value.trim(),
    }
  } catch (e) {
    const msg = (e as Error).message
    if (!logText.value.includes(msg)) {
      appendLog([`[${formatLogTime()}] ${t('projectFtpPushLogFailed')}`, `  ${msg}`])
    }
    return null
  }
}

async function pushOne(entry: ProjectWorkspaceEntry, deferNote: boolean): Promise<PushSuccessItem | null> {
  const fileName = entryUploadName(entry)
  const label = entryDisplayLabel(entry)
  const envName = ftpEnvs.value.find((e) => e.rec_id === ftpEnvRecId.value)?.env_name ?? ftpEnvRecId.value
  const noteHint: string[] = []
  if (deferNote) noteHint.push(`  ${t('projectBulkAutoFtpNoteDeferred')}`)
  else if (remoteNoteExists.value) noteHint.push(`  ${t('projectFtpPushLogNoteAppend')}`)

  appendLog([
    `[${formatLogTime()}] ${t('projectFtpPushLogStart')}`,
    `  ${t('projectFtpPushLogTarget', { name: label })}`,
    `  ${t('projectFtpPushLogEnv', { name: envName })}`,
    `  ${t('projectFtpPushLogPushPath', { path: pushPath.value.trim() })}`,
    ...noteHint,
  ])
  try {
    const start = await pushProjectWorkspaceFtp(props.projectRecId, {
      path: entry.path,
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      backup_path: backupPath.value.trim() || undefined,
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim(),
      detail: deferNote ? undefined : detail.value.trim() || undefined,
      skip_update_note: deferNote || undefined,
    })
    const result = await waitForFtpPushTask(start.task_id)
    const plannedBackup = backupFilePath(pushPath.value.trim(), fileName, backupPath.value)
    const successLines = [
      `[${formatLogTime()}] ${t('projectFtpPushLogSuccess')}`,
      `  ${t('projectFtpPushLogBackupPath', { path: result.backup_path || plannedBackup || '—' })}`,
      `  ${t('projectFtpPushLogUpdatePath', { path: result.update_path })}`,
    ]
    if (!deferNote && result.info_file_path) {
      successLines.push(`  ${t('projectFtpPushLogInfoPath', { path: result.info_file_path })}`)
    }
    appendLog(successLines)
    if (!deferNote && result.info_content?.trim()) {
      appendLog([`\n${t('projectFtpPushLogInfoContent')}`, result.info_content.trimEnd()])
    }
    return {
      entry,
      fileName,
      result: {
        ...result,
        backup_path: result.backup_path || plannedBackup,
      },
      pushPath: pushPath.value.trim(),
      taskRef: taskRef.value.trim(),
    }
  } catch (e) {
    const msg = (e as Error).message
    // Status polling already streams backend failure lines into the log.
    if (!logText.value.includes(msg)) {
      appendLog([`[${formatLogTime()}] ${t('projectFtpPushLogFailed')}`, `  ${msg}`])
    }
    return null
  }
}

async function writeMergedUpdateNote(items: PushSuccessItem[]): Promise<void> {
  if (!items.length) return
  appendLog([`[${formatLogTime()}] ${t('projectBulkAutoFtpNoteMergeStart', { count: items.length })}`])
  try {
    const res = await appendProjectWorkspaceFtpUpdateNote(props.projectRecId, {
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim(),
      detail: detail.value.trim() || undefined,
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
    if (res.info_content?.trim()) {
      appendLog([`\n${t('projectFtpPushLogInfoContent')}`, res.info_content.trimEnd()])
    }
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

  const list = activeEntries.value
  const mergeList = mergeVdatahubActive.value ? vdatahubEntries.value : []
  const rest = mergeVdatahubActive.value ? otherEntries.value : list
  const deferNote = rest.length + (mergeList.length ? 1 : 0) > 1
  logText.value = ''
  pushStatus.value = 'running'
  successItems.value = []
  hideCtxMenu()
  saving.value = true

  try {
    const ok: PushSuccessItem[] = []
    if (mergeList.length) {
      const item = await pushMergedVdatahub(mergeList, deferNote)
      if (item) ok.push(item)
    }
    for (const entry of rest) {
      const item = await pushOne(entry, deferNote)
      if (item) ok.push(item)
    }
    successItems.value = ok

    if (deferNote && ok.length) {
      try {
        await writeMergedUpdateNote(ok)
      } catch {
        // packages already uploaded; keep success list for notify
      }
    }

    const planned = (mergeList.length ? 1 : 0) + rest.length
    if (planned > 1 || list.length > 1) {
      appendLog([
        `[${formatLogTime()}] ${t('projectFtpPushMultiDone', {
          succeeded: ok.length,
          failed: planned - ok.length,
        })}`,
      ])
    }

    if (ok.length) {
      pushStatus.value = 'success'
      message.success(
        list.length > 1
          ? t('projectFtpPushMultiSuccess', { count: ok.length })
          : t('projectFtpPushSuccess'),
      )
      emit('success')
    } else {
      pushStatus.value = 'failed'
      message.error(list.length > 1 ? t('projectFtpPushMultiAllFailed') : t('projectFtpPushLogFailed'))
    }
  } catch (e) {
    const msg = (e as Error).message
    appendLog([`[${formatLogTime()}] ${t('projectFtpPushLogFailed')}`, `  ${msg}`])
    pushStatus.value = 'failed'
    if (msg.includes('build is running')) {
      message.error(t('projectFtpPushRunning'))
    } else {
      message.error(msg)
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
  void openNotify()
}

async function openNotify(): Promise<void> {
  const fields = buildMergedFtpNotifyFields(
    successItems.value.map((item) => ({
      projectName: props.projectName ?? '',
      projectVersion: props.projectVersion ?? '',
      projectType: props.projectType ?? '',
      fileName: item.fileName,
      path: item.entry.path,
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
    :title="$t('projectFtpPushTitle')"
    width="1000px"
    :show-footer="true"
    :confirm-text="$t('projectFtpPushExecute')"
    :confirm-loading="saving"
    :confirm-disabled="saving"
    @confirm="onSubmit"
  >
    <div class="project-ftp-push__layout">
      <div v-loading="loadingEnvs" class="project-ftp-push__main">
        <p v-if="activeEntries.length" class="project-ftp-push__target">
          {{ $t('projectFtpPushTarget', { name: entryLabel }) }}
        </p>
        <ul v-if="isMulti" class="project-ftp-push__entry-list">
          <li v-for="e in activeEntries" :key="e.path">
            {{ e.is_dir ? `${e.name}/` : e.name }}
          </li>
        </ul>
        <el-checkbox
          v-if="canMergeVdatahub"
          v-model="mergeVdatahub"
          class="project-ftp-push__merge"
          :disabled="saving"
        >
          {{ $t('projectFtpPushMergeVdatahub') }}
        </el-checkbox>
        <p v-if="canMergeVdatahub" class="project-ftp-push__hint">
          {{ $t('projectFtpPushMergeVdatahubHint') }}
        </p>
        <el-form label-width="108px" class="project-ftp-push__form" @submit.prevent="onSubmit">
          <el-form-item :label="$t('projectFtpPushEnv')" required>
            <el-select
              v-model="ftpEnvRecId"
              filterable
              teleported
              :loading="loadingEnvs"
              :popper-class="APP_DIALOG_POPPER_CLASS"
              class="project-ftp-push__full"
              :placeholder="$t('projectFtpPushEnvPlaceholder')"
              @change="onEnvChange"
              @visible-change="onEnvSelectVisible"
            >
              <el-option
                v-for="e in ftpEnvs"
                :key="e.rec_id"
                :label="ftpEnvOptionLabel(e)"
                :value="e.rec_id"
                :disabled="e.is_enabled !== 1"
              />
            </el-select>
            <p v-if="!loadingEnvs && !ftpEnvs.length" class="project-ftp-push__hint">
              {{ $t('envEmptyFilter') }}
            </p>
          </el-form-item>
          <el-form-item :label="$t('projectFtpPushPath')" required>
            <el-input v-model="pushPath" :placeholder="$t('projectFtpPushPathPlaceholder')" />
            <p class="project-ftp-push__hint">{{ $t('projectFtpPushPathHint') }}</p>
          </el-form-item>
          <el-form-item :label="$t('projectFtpPushBackupPath')">
            <el-input
              v-model="backupPath"
              :placeholder="defaultBackupPath || $t('projectFtpPushBackupPathPlaceholder')"
              @input="backupManual = true"
            />
            <p class="project-ftp-push__hint">{{ $t('projectFtpPushBackupPathHint') }}</p>
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
              :model-value="previewContent"
              type="textarea"
              :rows="previewRows"
              readonly
              class="project-ftp-push__preview"
            />
            <p v-if="remoteNoteExists" class="project-ftp-push__hint project-ftp-push__hint--append">
              {{ $t('projectFtpPushNoteAppendPreview') }}
            </p>
            <p class="project-ftp-push__hint">{{ $t('projectFtpPushInfoFormatHint') }}</p>
          </el-form-item>
        </el-form>
      </div>

      <aside class="project-ftp-push__log-panel">
        <ProjectBuildLogPanel
          class="project-ftp-push__log-console"
          :build-status="pushStatus"
          :content="logText"
          :loading="logLoading"
          :complete="logComplete"
        />
        <section
          v-if="successItems.length"
          class="project-ftp-push__success"
          @contextmenu="onSuccessContextMenu"
        >
          <header class="project-ftp-push__success-head">
            <div class="project-ftp-push__success-title">
              {{ $t('projectBulkAutoFtpSuccessList') }}
              <span class="project-ftp-push__success-tip">{{ $t('projectBulkAutoFtpSuccessTip') }}</span>
            </div>
            <el-button type="primary" link size="small" @click="openNotify()">
              {{ $t('projectFtpNotifyAction') }}
            </el-button>
          </header>
          <ul class="project-ftp-push__success-list">
            <li
              v-for="item in successItems"
              :key="item.entry.path + item.result.update_path"
              class="project-ftp-push__success-item"
            >
              <strong>{{ item.fileName || item.entry.name }}</strong>
              <span>{{ item.result.update_path }}</span>
            </li>
          </ul>
        </section>
      </aside>
    </div>
  </AppDialog>

  <Teleport to="body">
    <ul
      v-if="ctxMenu.visible"
      class="project-ftp-push__ctx"
      :style="{ left: `${ctxMenu.x}px`, top: `${ctxMenu.y}px` }"
      @click.stop
    >
      <li class="project-ftp-push__ctx-item" @click="openNotifyFromCtx">
        {{ $t('projectFtpNotifyAction') }}
      </li>
    </ul>
  </Teleport>

  <ProjectFtpNotifyEmailDialog v-model:open="notifyOpen" :initial="notifyInitial" />
</template>

<style scoped lang="scss">
.project-ftp-push {
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

  &__target {
    margin: 0 0 8px;
    font-size: 13px;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__merge {
    margin: 0 0 4px;
    font-size: 13px;
  }

  &__entry-list {
    margin: 0 0 12px;
    padding: 8px 10px;
    list-style: none;
    max-height: 96px;
    overflow-y: auto;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
    background: var(--el-fill-color-lighter);
    font-size: 12px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));

    li + li {
      margin-top: 2px;
    }
  }

  &__full {
    width: 100%;
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

  &__log-panel {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    background: var(--el-fill-color-blank);
    overflow: hidden;
    padding: 0;
  }

  &__log-console {
    flex: 1;
    min-height: 0;
  }

  &__success {
    flex-shrink: 0;
    border-top: 1px solid var(--el-border-color-lighter);
    padding: 10px 12px;
    background: var(--el-fill-color-blank);
  }

  &__success-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 8px;
  }

  &__success-title {
    font-size: 13px;
    font-weight: 600;
  }

  &__success-tip {
    margin-left: 8px;
    font-size: 12px;
    font-weight: 400;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__success-list {
    margin: 0;
    padding: 0;
    list-style: none;
    max-height: 120px;
    overflow-y: auto;
  }

  &__success-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 6px 0;
    font-size: 12px;
    border-top: 1px dashed var(--el-border-color-extra-light);

    &:first-child {
      border-top: 0;
      padding-top: 0;
    }

    strong {
      font-size: 13px;
    }

    span {
      color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
      word-break: break-all;
    }
  }
}
</style>

<style lang="scss">
.project-ftp-push__ctx {
  position: fixed;
  z-index: 4000;
  margin: 0;
  padding: 4px 0;
  list-style: none;
  min-width: 140px;
  background: var(--el-bg-color-overlay, #fff);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  box-shadow: var(--el-box-shadow-light);
}

.project-ftp-push__ctx-item {
  padding: 8px 14px;
  font-size: 13px;
  cursor: pointer;

  &:hover {
    background: var(--el-fill-color-light);
    color: var(--el-color-primary);
  }
}

.app-dialog--windowed.app-dialog--initial-height .app-dialog__body:has(.project-ftp-push__layout) {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.app-dialog--windowed.app-dialog--initial-height .app-dialog__content:has(.project-ftp-push__layout) {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.app-dialog--windowed.app-dialog--initial-height
  .app-dialog__content:has(.project-ftp-push__layout)
  .project-ftp-push__layout {
  flex: 1;
  min-height: 0;
}

.app-dialog.is-fullscreen .app-dialog__content:has(.project-ftp-push__layout),
.app-dialog--fullscreen .app-dialog__content:has(.project-ftp-push__layout) {
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
