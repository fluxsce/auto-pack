<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsIcon, RsLog, RsTag, useRsToast } from '@/ui'
import { useAuthStore } from '@/stores/auth'
import { useWorkspaceStore } from '@/stores/workspace'
import {
  applyJobLog,
  errText,
  fetchRemoteNote,
  getJob,
  getJobLog,
  getWorkspaceEnv,
  listWorkspaceEnvs,
  submitJob,
  type ProjectFileEntry,
} from '../../api'
import { parseRecipe, type ProjectRecipe } from '../../lib/project-defaults'
import {
  backupFilePath,
  defaultBackupDir,
  mergeScePushEntries,
  resolvePushLayout,
  uploadFileName,
} from '../../lib/workspace-ftp'
import { buildMergedFtpNotifyFields, type FtpNotifyEmailFields } from '../../lib/ftp-notify-email'
import { statusVariant } from '../../composables/use-project-list'
import MailPanel from './MailPanel.vue'
import PushPanel from './PushPanel.vue'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{
  workspaceId: string
  projectId: string
  projectName?: string
  projectType?: string
  recipe?: ProjectRecipe
  entries: ProjectFileEntry[]
}>()
const emit = defineEmits<{ success: [jobId: string] }>()

type SuccessItem = {
  path: string
  fileName: string
  updatePath: string
  backupPath: string
}

const { t } = useI18n()
const toast = useRsToast()
const auth = useAuthStore()
const ws = useWorkspaceStore()
const envOptions = ref<{ value: string; label: string }[]>([])
const smtpOptions = ref<{ value: string; label: string }[]>([])
const logText = ref('')
const jobStatus = ref('')
const running = ref(false)
const backupManual = ref(false)
const successItems = ref<SuccessItem[]>([])
const notifyOpen = ref(false)
const notifyInitial = ref<FtpNotifyEmailFields | null>(null)
const form = ref(emptyForm())
const remoteNote = ref('')
const remoteNoteState = ref<'idle' | 'loading' | 'exists' | 'missing' | 'error'>('idle')
let followGen = 0
let noteGen = 0
let noteTimer: ReturnType<typeof setTimeout> | undefined

const workspace = computed(() => ws.list.find((item) => item.rec_id === props.workspaceId))
const paths = computed(() => props.entries.map((item) => item.path).filter(Boolean))
const previewEntries = computed(() => mergeScePushEntries(props.entries))
const previewFileName = computed(() => {
  const first = previewEntries.value[0]
  return first ? uploadFileName(first.name, first.is_dir) : 'package'
})
const canSubmit = computed(() => {
  if (running.value) return false
  return Boolean(
    form.value.envRecId
    && form.value.pushPath.trim()
    && form.value.pusher.trim()
    && form.value.taskRef.trim()
    && paths.value.length,
  )
})

function emptyForm() {
  return { envRecId: '', pushPath: '', backupPath: '', pusher: '', taskRef: '', detail: '' }
}

function defaultPusher(): string {
  return (auth.me?.display_name || auth.me?.login_name || '').trim()
}

function statusLabel(code?: string): string {
  if (!code) return ''
  const key = `jobs.statuses.${code}`
  const label = t(key)
  return label === key ? code : label
}

function notes(): string {
  return [
    `${t('workspaces.remote.pusher')}:${form.value.pusher.trim()}`,
    `${t('workspaces.remote.backup')}:${form.value.backupPath.trim()}`,
    form.value.taskRef.trim(),
    form.value.detail.trim(),
  ].filter(Boolean).join(' | ')
}

function syncBackupDefault(): void {
  if (backupManual.value || !form.value.pushPath.trim()) return
  form.value.backupPath = defaultBackupDir(form.value.pushPath, previewFileName.value)
}

function collectSuccess(): SuccessItem[] {
  return previewEntries.value.map((item) => {
    const fileName = uploadFileName(item.name, item.is_dir)
    return {
      path: item.path,
      fileName,
      updatePath: resolvePushLayout(form.value.pushPath, fileName).updatePath,
      backupPath: backupFilePath(form.value.pushPath, fileName, form.value.backupPath),
    }
  })
}

function prefill(): void {
  const recipe = props.recipe ?? parseRecipe('')
  const upload = (recipe.post_actions ?? []).find((act) => ['ftp', 'sftp', 'ssh'].includes(act.kind) && act.env_rec_id)
  form.value.envRecId = upload?.env_rec_id || recipe.ftp_env_rec_id || ''
  form.value.pushPath = (upload?.remote_path || recipe.remote_path || '').trim()
  form.value.pusher = defaultPusher()
  form.value.taskRef = ''
  form.value.detail = ''
  backupManual.value = false
  syncBackupDefault()
}

async function onEnvChange(id: string): Promise<void> {
  if (!id || form.value.pushPath.trim()) return
  try {
    const row = await getWorkspaceEnv(id)
    const cfg = row.config ?? {}
    const base = String(cfg.remote_dir ?? cfg.remote_path ?? cfg.base_path ?? '').trim() || '/'
    form.value.pushPath = base === '/' ? '/update' : base
    syncBackupDefault()
  } catch {
    /* keep path */
  }
}

async function onOpen(): Promise<void> {
  const keepSession = running.value || notifyOpen.value || successItems.value.length > 0
  if (!keepSession) {
    logText.value = ''
    jobStatus.value = ''
    successItems.value = []
    form.value = emptyForm()
    backupManual.value = false
  }
  if (!props.workspaceId) return
  try {
    const envs = await listWorkspaceEnvs(props.workspaceId)
    envOptions.value = envs
      .filter((e) => e.is_enabled !== 0 && (e.env_kind === 'ftp' || e.env_kind === 'sftp' || e.env_kind === 'ssh'))
      .map((e) => {
        const kindLabel = t(`envs.kinds.${e.env_kind}`)
        return { value: e.rec_id, label: `${e.env_name} (${kindLabel})` }
      })
    smtpOptions.value = envs
      .filter((e) => e.is_enabled !== 0 && e.env_kind === 'smtp')
      .map((e) => ({ value: e.rec_id, label: e.env_name }))
    if (!keepSession) {
      prefill()
      if (form.value.envRecId) void onEnvChange(form.value.envRecId)
    }
  } catch (e) {
    toast.error(errText(e))
  }
}

function mergeFollowLog(header: string, view: { log: string; size: number; offset: number }, size: number): number {
  if (view.offset === 0) {
    logText.value = view.log ? `${header}\n${view.log}` : header
    return view.size
  }
  const next = applyJobLog(logText.value, view)
  logText.value = next.text
  return next.size || size
}

function finishFollow(status: string, errorText?: string): void {
  if (status === 'succeeded') {
    successItems.value = collectSuccess()
    toast.success(t('workspaces.remote.doneOk'))
    void openNotify()
    return
  }
  toast.error(t('workspaces.remote.doneFail', { msg: errorText || status }))
}

async function followJob(id: string, header: string): Promise<void> {
  const gen = followGen
  let size = 0
  const terminal = new Set(['succeeded', 'failed', 'canceled', 'cancelled'])
  for (;;) {
    if (gen !== followGen) return
    const [view, info] = await Promise.all([
      getJobLog(id, size).catch(() => null),
      getJob(id),
    ])
    if (gen !== followGen) return
    jobStatus.value = info.job.job_status
    if (view) size = mergeFollowLog(header, view, size)
    if (terminal.has(info.job.job_status)) {
      finishFollow(info.job.job_status, info.job.error_text)
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 1500))
  }
}

async function submit(): Promise<void> {
  if (!canSubmit.value) return
  followGen += 1
  running.value = true
  successItems.value = []
  notifyOpen.value = false
  const header = `[${new Date().toLocaleString()}] ${t('workspaces.remote.start', { n: paths.value.length })}`
  logText.value = header
  jobStatus.value = 'queued'
  try {
    const job = await submitJob(props.workspaceId, props.projectId, 'ftp_push', {
      env_rec_id: form.value.envRecId,
      remote_path: form.value.pushPath.trim(),
      backup_path: form.value.backupPath.trim(),
      pusher: form.value.pusher.trim(),
      task_ref: form.value.taskRef.trim(),
      detail: form.value.detail.trim(),
      artifacts: paths.value,
      notes: notes(),
    })
    toast.success(t('workspaces.remote.queued'))
    emit('success', job.rec_id)
    await followJob(job.rec_id, header)
  } catch (e) {
    jobStatus.value = 'failed'
    logText.value = `${header}\n${errText(e)}`
    toast.error(errText(e))
  } finally {
    running.value = false
  }
}

async function openNotify(): Promise<void> {
  const fields = buildMergedFtpNotifyFields(
    successItems.value.map((item) => ({
      projectName: props.projectName ?? '',
      projectType: props.projectType ?? '',
      fileName: item.fileName,
      path: item.path,
      pushPath: form.value.pushPath,
      taskRef: form.value.taskRef,
      backupPath: item.backupPath,
    })),
    {
      workspaceCode: workspace.value?.workspace_code,
      workspaceName: workspace.value?.workspace_name,
    },
  )
  if (!fields) {
    toast.warning(t('workspaces.remote.notify.empty'))
    return
  }
  notifyInitial.value = fields
  await nextTick()
  notifyOpen.value = true
}

function scheduleRemoteNote(): void {
  if (noteTimer) clearTimeout(noteTimer)
  const env = form.value.envRecId
  const path = form.value.pushPath.trim()
  if (!open.value || !props.projectId || !env || !path) {
    remoteNote.value = ''
    remoteNoteState.value = 'idle'
    return
  }
  remoteNote.value = ''
  remoteNoteState.value = 'loading'
  noteTimer = setTimeout(() => {
    void loadRemoteNote()
  }, 400)
}

async function loadRemoteNote(): Promise<void> {
  const gen = ++noteGen
  remoteNoteState.value = 'loading'
  try {
    const view = await fetchRemoteNote(props.projectId, {
      env_rec_id: form.value.envRecId,
      push_path: form.value.pushPath.trim(),
      file_name: previewFileName.value,
    })
    if (gen !== noteGen) return
    remoteNote.value = view.exists ? view.content : ''
    remoteNoteState.value = view.exists ? 'exists' : 'missing'
  } catch {
    if (gen !== noteGen) return
    remoteNote.value = ''
    remoteNoteState.value = 'error'
  }
}

watch(open, (visible) => {
  if (visible) void onOpen()
})

onUnmounted(() => {
  followGen += 1
  noteGen += 1
  if (noteTimer) clearTimeout(noteTimer)
})

watch(
  () => [form.value.pushPath, previewFileName.value],
  () => syncBackupDefault(),
)

watch(
  () => [open.value, form.value.envRecId, form.value.pushPath, previewFileName.value],
  () => scheduleRemoteNote(),
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
    :title="t('workspaces.remote.title')"
    :description="t('workspaces.remote.hint', { n: paths.length })"
  >
    <template #body>
      <div class="remote-split">
        <div class="form-col">
          <PushPanel
            v-model:form="form"
            :entries="previewEntries"
            :env-options="envOptions"
            :disabled="running"
            :remote-note="remoteNote"
            :remote-note-state="remoteNoteState"
            @backup-edit="backupManual = true"
            @env-change="onEnvChange"
          />
        </div>
        <div class="log-col">
          <header class="sec-head log-head">
            <h3>{{ t('workspaces.remote.logTitle') }}</h3>
            <RsTag v-if="jobStatus" :variant="statusVariant(jobStatus)" round>{{ statusLabel(jobStatus) }}</RsTag>
          </header>
          <RsLog
            v-model:lines="logText"
            class="log"
            height="100%"
            :follow="running"
            :empty-text="t('workspaces.remote.logEmpty')"
            :aria-label="t('workspaces.remote.logTitle')"
          />
          <section v-if="successItems.length" class="success">
            <header class="success-head">
              <div>
                <strong>{{ t('workspaces.remote.successTitle') }}</strong>
                <p class="hint">{{ t('workspaces.remote.successTip') }}</p>
              </div>
              <RsButton size="sm" @click="openNotify">
                <RsIcon name="mail" :size="14" />
                {{ t('workspaces.remote.notify.action') }}
              </RsButton>
            </header>
            <ul class="file-list">
              <li v-for="item in successItems" :key="item.path" class="row">{{ item.fileName }}</li>
            </ul>
          </section>
        </div>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" :disabled="running" @click="open = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="running" :disabled="!canSubmit" @click="submit">{{ t('workspaces.remote.execute') }}</RsButton>
    </template>
  </RsDialog>
  <MailPanel v-model:open="notifyOpen" :initial="notifyInitial" :smtp-options="smtpOptions" />
</template>

<style scoped src="./dialog.css"></style>
<style scoped>
.remote-split {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(22rem, 28rem) minmax(0, 1fr);
  gap: 1rem;
}

.log-head {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.log-head h3 {
  margin: 0;
}

.log {
  flex: 1;
  min-height: 12rem;
  height: 0;
}

.success {
  flex-shrink: 0;
  padding-top: 0.65rem;
  border-top: 1px solid var(--shell-border);
}

.success-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}

.success-head .hint {
  margin: 0.15rem 0 0;
}

@media (max-width: 900px) {
  .remote-split {
    grid-template-columns: 1fr;
  }
}
</style>
