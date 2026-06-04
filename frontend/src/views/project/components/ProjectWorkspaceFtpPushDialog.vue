<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import { fetchEnvSettings } from '@/request/api/env'
import { pushProjectWorkspaceFtp } from '@/request/api/build'
import type { EnvSetting, PackBuildStatus, ProjectWorkspaceEntry } from '@/types'
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

function normalizeRemoteDir(p: string): string {
  let s = p.trim().replaceAll('\\', '/').replace(/\/+$/, '')
  if (!s) return '/'
  if (!s.startsWith('/')) s = `/${s}`
  return s
}

function resolvePushLayout(pushPathRaw: string, fileName: string): { pushDir: string; updatePath: string } {
  const trimmed = pushPathRaw.trim().replaceAll('\\', '/').replace(/\/+$/, '') || '/'
  const normalized = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  const base = normalized.split('/').pop() ?? ''
  if (fileName && base === fileName) {
    const parent = normalized.slice(0, normalized.length - base.length).replace(/\/+$/, '') || '/'
    return { pushDir: normalizeRemoteDir(parent), updatePath: normalized }
  }
  const pushDir = normalizeRemoteDir(pushPathRaw)
  return { pushDir, updatePath: `${pushDir}/${fileName}` }
}

const pushLayout = computed(() => resolvePushLayout(pushPath.value, uploadName.value))

const defaultBackupPath = computed(() => {
  if (!pushPath.value.trim()) return ''
  const stamp = formatBackupStamp(new Date())
  return `${pushLayout.value.pushDir}/BAK/${stamp}`
})

const backupFilePath = computed(() => {
  const backupDir = backupPath.value.trim() || defaultBackupPath.value
  if (!backupDir) return ''
  return `${normalizeRemoteDir(backupDir)}/${uploadName.value}`
})

const previewContent = computed(() =>
  buildManifestPreview(
    new Date(),
    pusher.value,
    taskRef.value,
    backupFilePath.value,
    pushPath.value.trim() ? pushLayout.value.updatePath : uploadName.value,
    detail.value,
  ),
)

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

function resetForm(): void {
  ftpEnvRecId.value = ''
  pushPath.value = ''
  backupPath.value = ''
  backupManual.value = false
  pusher.value = ''
  taskRef.value = ''
  detail.value = ''
  logText.value = t('projectFtpPushLogReady')
  pushStatus.value = null
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

function formatBackupStamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}`
}

function buildManifestPreview(
  pushTime: Date,
  pusherName: string,
  taskRefText: string,
  backupPathText: string,
  updatePathText: string,
  detailText: string,
): string {
  const dateYmd = formatBackupStamp(pushTime)
  const lines = [
    `${dateYmd} ${pusherName.trim()} ${taskRefText.trim()}`,
    `备份包路径:${backupPathText.trim()}`,
    `更新包路径:${updatePathText.trim()}`,
  ]
  const detail = detailText.replace(/\r\n/g, '\n').trimEnd()
  if (detail.trim()) {
    lines.push(detail)
  }
  return `${lines.join('\n')}\n`
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
  ])

  saving.value = true
  try {
    appendLog([`[${formatLogTime()}] ${t('projectFtpPushLogUploading')}`])
    const res = await pushProjectWorkspaceFtp(props.projectRecId, {
      path: props.entry.path,
      ftp_env_rec_id: ftpEnvRecId.value,
      push_path: pushPath.value.trim(),
      backup_path: backupPath.value.trim(),
      pusher: pusher.value.trim(),
      task_ref: taskRef.value.trim(),
      detail: detail.value.trim(),
    })
    appendLog([
      `[${formatLogTime()}] ${t('projectFtpPushLogSuccess')}`,
      `  ${t('projectFtpPushLogBackupPath', { path: res.backup_path || '—' })}`,
      `  ${t('projectFtpPushLogUpdatePath', { path: res.update_path })}`,
      `  ${t('projectFtpPushLogInfoPath', { path: res.info_file_path })}`,
      '',
      t('projectFtpPushLogInfoContent'),
      res.info_content.trimEnd(),
    ])
    pushStatus.value = 'success'
    message.success(t('projectFtpPushSuccess'))
    emit('success')
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
          <el-form-item :label="$t('projectFtpPushInfoPreview')">
            <el-input
              :model-value="previewContent"
              type="textarea"
              :rows="4"
              readonly
              class="project-ftp-push__preview"
            />
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
