<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import { fetchEnvSettings } from '@/request/api/env'
import {
  fetchProjectWorkspaceFtpPushStatus,
  fetchProjectWorkspaceFtpRemoteNote,
  pushProjectWorkspaceFtp,
} from '@/request/api/build'
import type { EnvSetting, PackBuildStatus, ProjectWorkspaceEntry } from '@/types'
import {
  backupFilePath,
  buildManifestPreview,
  defaultBackupDir,
  mergeUpdateNotePreview,
  resolvePushLayout,
} from '@/utils/workspace-ftp'
import { ProjectBuildLogPanel } from './panels'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  projectRecId: string
  entry: ProjectWorkspaceEntry | null
}>()

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()
const message = useMessage()

const ftpEnvs = ref<EnvSetting[]>([])
const loadingEnvs = ref(false)
const saving = ref(false)
const logText = ref('')
const pushStatus = ref<PackBuildStatus | null>(null)

const ftpEnvRecId = ref('')
const pushPath = ref('')
const backupPath = ref('')
const backupManual = ref(false)
const pusher = ref('')
const taskRef = ref('')
const detail = ref('')
const remoteNoteContent = ref('')
const remoteNoteExists = ref(false)
const remoteNoteLoading = ref(false)
let remoteNoteTimer: ReturnType<typeof setTimeout> | null = null
let pushPollTimer: ReturnType<typeof setTimeout> | null = null

const entryLabel = computed(() => {
  const e = props.entry
  if (!e) return ''
  return e.is_dir ? `${e.name}/` : e.name
})

const uploadName = computed(() => {
  const e = props.entry
  if (!e) return 'package'
  return e.is_dir ? `${e.name}.zip` : e.name
})

const pushLayout = computed(() => resolvePushLayout(pushPath.value, uploadName.value))

const defaultBackupPath = computed(() => {
  if (!pushPath.value.trim()) return ''
  return defaultBackupDir(pushPath.value, uploadName.value)
})

const backupFilePathPreview = computed(() => {
  if (!pushPath.value.trim()) return ''
  return backupFilePath(pushPath.value, uploadName.value, backupPath.value)
})

const newManifestBlock = computed(() =>
  buildManifestPreview(
    new Date(),
    pusher.value,
    taskRef.value,
    backupFilePathPreview.value,
    pushPath.value.trim() ? pushLayout.value.updatePath : uploadName.value,
    detail.value,
  ),
)

const previewContent = computed(() =>
  mergeUpdateNotePreview(remoteNoteContent.value, newManifestBlock.value),
)

const previewRows = computed(() => (remoteNoteExists.value ? 8 : 4))

const logComplete = computed(() => pushStatus.value !== 'running')
const logLoading = computed(() => saving.value && pushStatus.value === 'running' && !logText.value)

watch(open, (v) => {
  if (!v) return
  resetForm()
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

watch([ftpEnvRecId, pushPath, uploadName], () => {
  scheduleRemoteNoteLoad()
})

onUnmounted(() => {
  if (remoteNoteTimer) clearTimeout(remoteNoteTimer)
  stopPushPolling()
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
      file_name: uploadName.value,
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

async function onSubmit(): Promise<void> {
  if (!props.projectRecId || !props.entry) return
  if (!ftpEnvRecId.value) {
    message.warning(t('projectFtpPushEnvRequired'))
    return
  }
  if (!pushPath.value.trim()) {
    message.warning(t('projectFtpPushPathRequired'))
    return
  }
  if (!pusher.value.trim()) {
    message.warning(t('projectFtpPushPusherRequired'))
    return
  }
  if (!taskRef.value.trim()) {
    message.warning(t('projectFtpPushTaskRefRequired'))
    return
  }

  const envName = ftpEnvs.value.find((e) => e.rec_id === ftpEnvRecId.value)?.env_name ?? ftpEnvRecId.value
  logText.value = ''
  pushStatus.value = 'running'
  appendLog([
    `[${formatLogTime()}] ${t('projectFtpPushLogStart')}`,
    `  ${t('projectFtpPushLogTarget', { name: entryLabel.value })}`,
    `  ${t('projectFtpPushLogEnv', { name: envName })}`,
    `  ${t('projectFtpPushLogPushPath', { path: pushPath.value.trim() })}`,
    ...(remoteNoteExists.value ? [`  ${t('projectFtpPushLogNoteAppend')}`] : []),
  ])

  saving.value = true
  try {
    const start = await pushProjectWorkspaceFtp(props.projectRecId, {
      path: props.entry.path,
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      backup_path: backupPath.value.trim(),
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim(),
      detail: detail.value.trim(),
    })
    await pollPushTask(start.task_id)
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
  }
}

async function pollPushTask(taskId: string): Promise<void> {
  stopPushPolling()
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
    if (st.status === 'success' && st.result) {
      pushStatus.value = 'success'
      if (st.result.info_content?.trim()) {
        logText.value = `${(st.log_content || '').trimEnd()}\n\n${t('projectFtpPushLogInfoContent')}\n${st.result.info_content.trimEnd()}`.trim()
      }
      message.success(t('projectFtpPushSuccess'))
      emit('success')
      return
    }
    pushStatus.value = 'failed'
    throw new Error(st.message || t('projectFtpPushLogFailed'))
  }
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectFtpPushTitle')"
    width="1000px"
    :show-footer="true"
    :confirm-text="$t('projectFtpPushExecute')"
    :confirm-loading="saving"
    @confirm="onSubmit"
  >
    <div class="project-ftp-push__layout">
      <div v-loading="loadingEnvs" class="project-ftp-push__main">
        <p v-if="entry" class="project-ftp-push__target">
          {{ $t('projectFtpPushTarget', { name: entryLabel }) }}
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
      </aside>
    </div>
  </AppDialog>
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
    margin: 0 0 12px;
    font-size: 13px;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
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
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    background: var(--el-fill-color-blank);
    overflow: hidden;
  }

  &__log-console {
    flex: 1;
    min-height: 0;
  }
}
</style>

<style lang="scss">
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
