<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  RsButton,
  RsCard,
  RsDialog,
  RsEmpty,
  RsFormItem,
  RsIcon,
  RsLog,
  RsSelect,
  RsTag,
  useRsToast,
  type RsLogLevel,
  type RsLogLineInput,
} from '@/ui'
import { useAuthStore } from '@/stores/auth'
import type { Workspace } from '@/stores/workspace'
import {
  errText,
  fetchRemoteNote,
  getJob,
  getJobLog,
  getWorkspaceEnv,
  asSelectStrings,
  listProjects,
  listWorkspaceEnvs,
  submitJob,
  type Project,
  type ProjectFileEntry,
} from '../api'
import { statusVariant, typeLabel, typeVariant } from '../composables/use-project-list'
import {
  backupFilePath,
  defaultBackupDir,
  resolvePushLayout,
  uploadFileName,
} from '../lib/workspace-ftp'
import { buildMergedFtpNotifyFields, type FtpNotifyEmailFields } from '../lib/ftp-notify-email'
import { parseRecipe } from '../lib/project-defaults'
import MailPanel from '../components/remote/MailPanel.vue'
import PushPanel from '../components/remote/PushPanel.vue'
import ProjectPushCard from './ProjectPushCard.vue'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ workspace?: Workspace | null }>()
const emit = defineEmits<{ success: [] }>()

type SuccessItem = {
  path: string
  fileName: string
  updatePath: string
  backupPath: string
  projectName: string
  projectType: string
}

const { t } = useI18n()
const toast = useRsToast()
const auth = useAuthStore()
const projects = ref<Project[]>([])
const selectedIds = ref<string[]>([])
const projectPaths = ref<Record<string, string[]>>({})
const envOptions = ref<{ value: string; label: string }[]>([])
const smtpOptions = ref<{ value: string; label: string }[]>([])
const form = ref(emptyForm())
const backupManual = ref(false)
const remoteNote = ref('')
const remoteNoteState = ref<'idle' | 'loading' | 'exists' | 'missing' | 'error'>('idle')
const logLines = ref<RsLogLineInput[]>([])
const jobStatus = ref('')
const running = ref(false)
const successItems = ref<SuccessItem[]>([])
const notifyOpen = ref(false)
const notifyInitial = ref<FtpNotifyEmailFields | null>(null)
let cancelled = false
let followGen = 0
let noteGen = 0
let noteTimer: ReturnType<typeof setTimeout> | undefined

const projectOptions = computed(() =>
  projects.value
    .filter((p) => p.is_enabled !== 0)
    .map((p) => ({ value: String(p.rec_id), label: p.project_name })),
)
const selectedIdList = computed(() => asSelectStrings(selectedIds.value))
const queue = computed(() => {
  const picked = new Set(selectedIdList.value)
  return projects.value.filter((p) => p.is_enabled !== 0 && picked.has(String(p.rec_id)))
})
const previewEntries = computed<ProjectFileEntry[]>(() => {
  const out: ProjectFileEntry[] = []
  const seen = new Set<string>()
  for (const row of queue.value) {
    for (const rel of projectPaths.value[String(row.rec_id)] ?? []) {
      const entry = pathToEntry(rel)
      const key = `${entry.path}\0${entry.is_dir ? 'd' : 'f'}`
      if (seen.has(key)) continue
      seen.add(key)
      out.push(entry)
    }
  }
  return out
})
const previewFileName = computed(() => {
  const first = previewEntries.value[0]
  return first ? uploadFileName(first.name, first.is_dir) : 'package'
})
const noteProjectId = computed(() => (queue.value[0] ? String(queue.value[0].rec_id) : ''))
const canSubmit = computed(() => {
  if (running.value) return false
  if (!form.value.envRecId || !form.value.pushPath.trim() || !form.value.pusher.trim() || !form.value.taskRef.trim()) {
    return false
  }
  return queue.value.length > 0 && queue.value.every((row) => (projectPaths.value[String(row.rec_id)] ?? []).length > 0)
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

function pathToEntry(raw: string): ProjectFileEntry {
  let path = raw.trim().replaceAll('\\', '/')
  while (path.endsWith('/')) path = path.slice(0, -1)
  const parts = path.split('/').filter(Boolean)
  const name = parts.length ? parts[parts.length - 1] : path
  const glob = name.includes('*') || name.includes('?') || name.includes('[')
  const isDir = !glob && !name.includes('.')
  return { name, path, is_dir: isDir }
}

function setProjectPaths(id: string, rels: string[]): void {
  projectPaths.value = { ...projectPaths.value, [String(id)]: rels }
}

function removeProject(id: string): void {
  const drop = String(id)
  selectedIds.value = selectedIdList.value.filter((item) => item !== drop)
}

function recipeUpload(row: Project): { envId: string; remote: string } {
  const recipe = parseRecipe(row.recipe_json, row.project_type)
  const upload = (recipe.post_actions ?? []).find((act) => ['ftp', 'sftp', 'ssh'].includes(act.kind) && act.env_rec_id)
  return {
    envId: upload?.env_rec_id || recipe.ftp_env_rec_id || '',
    remote: (upload?.remote_path || recipe.remote_path || '').trim(),
  }
}

function syncBackupDefault(): void {
  if (backupManual.value || !form.value.pushPath.trim()) return
  form.value.backupPath = defaultBackupDir(form.value.pushPath, previewFileName.value)
}

function collectSuccess(row: Project, rels: string[]): SuccessItem[] {
  return rels.map((rel) => {
    const entry = pathToEntry(rel)
    const fileName = uploadFileName(entry.name, entry.is_dir)
    return {
      path: `${row.rec_id}:${entry.path}`,
      fileName,
      updatePath: resolvePushLayout(form.value.pushPath, fileName).updatePath,
      backupPath: backupFilePath(form.value.pushPath, fileName, form.value.backupPath),
      projectName: row.project_name,
      projectType: row.project_type,
    }
  })
}

function syncSelectedProjects(): void {
  const paths: Record<string, string[]> = {}
  for (const id of selectedIdList.value) {
    paths[id] = projectPaths.value[id] ?? []
  }
  projectPaths.value = paths
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

function prefillShared(rows: Project[]): void {
  form.value.pusher = defaultPusher()
  form.value.taskRef = t('workspaces.autoPushDefaultNote')
  form.value.detail = ''
  backupManual.value = false
  for (const row of rows) {
    const { envId, remote } = recipeUpload(row)
    if (!form.value.envRecId && envId) form.value.envRecId = envId
    if (!form.value.pushPath.trim() && remote) form.value.pushPath = remote
    if (form.value.envRecId && form.value.pushPath.trim()) break
  }
  syncBackupDefault()
}

async function onOpen(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id) return
  cancelled = false
  const keepSession = running.value || notifyOpen.value || successItems.value.length > 0
  if (!keepSession) {
    logLines.value = []
    jobStatus.value = ''
    successItems.value = []
    form.value = emptyForm()
    selectedIds.value = []
    projectPaths.value = {}
    backupManual.value = false
  }
  try {
    projects.value = await listProjects(id)
    const enabled = projects.value.filter((p) => p.is_enabled !== 0)
    const envs = await listWorkspaceEnvs(id)
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
      prefillShared(enabled)
      if (form.value.envRecId) void onEnvChange(form.value.envRecId)
    }
  } catch (e) {
    toast.error(errText(e))
  }
}

function mergeFollowLog(view: { log: string; size: number; offset: number }, size: number): number {
  if (view.offset === 0) {
    if (size === 0 && view.log) append(view.log)
    return view.size
  }
  if (view.log) append(view.log)
  return view.size || size
}

async function followJob(id: string): Promise<string> {
  const gen = followGen
  let size = 0
  const terminal = new Set(['succeeded', 'failed', 'canceled', 'cancelled'])
  for (;;) {
    if (cancelled || gen !== followGen) return 'canceled'
    const [view, info] = await Promise.all([
      getJobLog(id, size).catch(() => null),
      getJob(id),
    ])
    if (cancelled || gen !== followGen) return 'canceled'
    jobStatus.value = info.job.job_status
    if (view) size = mergeFollowLog(view, size)
    if (terminal.has(info.job.job_status)) return info.job.job_status
    await new Promise((resolve) => setTimeout(resolve, 1500))
  }
}

function append(text: string, level?: RsLogLevel): void {
  const parts = text.replaceAll('\r\n', '\n').replaceAll('\r', '\n').split('\n')
  if (parts.length > 1 && parts.at(-1) === '') parts.pop()
  logLines.value = [...logLines.value, ...parts.map((line) => (level ? { text: line, level } : line))]
}

function pushDoneLevel(ok: number, failed: number): RsLogLevel {
  if (failed <= 0) return 'success'
  if (ok > 0) return 'warn'
  return 'error'
}

async function openNotify(): Promise<void> {
  const fields = buildMergedFtpNotifyFields(
    successItems.value.map((item) => ({
      projectName: item.projectName,
      projectType: item.projectType,
      fileName: item.fileName,
      path: item.path,
      pushPath: form.value.pushPath,
      taskRef: form.value.taskRef,
      backupPath: item.backupPath,
    })),
    {
      workspaceCode: props.workspace?.workspace_code,
      workspaceName: props.workspace?.workspace_name,
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
  if (!open.value || !noteProjectId.value || !env || !path) {
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
    const view = await fetchRemoteNote(noteProjectId.value, {
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

async function submit(): Promise<void> {
  const wsId = props.workspace?.rec_id
  if (!wsId || !canSubmit.value) return
  followGen += 1
  running.value = true
  cancelled = false
  successItems.value = []
  notifyOpen.value = false
  let ok = 0
  let failed = 0
  logLines.value = []
  append(t('workspaces.autoPushStart', { count: queue.value.length }), 'info')
  jobStatus.value = 'queued'
  try {
    for (const [i, p] of queue.value.entries()) {
      if (cancelled) {
        append(t('workspaces.autoPushCanceled'), 'warn')
        break
      }
      const rels = projectPaths.value[String(p.rec_id)] ?? []
      const header = `[${new Date().toLocaleString()}] ${i + 1}/${queue.value.length} ${t('workspaces.autoReleaseFtpProject', { name: p.project_name })}`
      append(header, 'info')
      try {
        const job = await submitJob(wsId, p.rec_id, 'ftp_push', {
          env_rec_id: form.value.envRecId,
          remote_path: form.value.pushPath.trim(),
          backup_path: form.value.backupPath.trim(),
          pusher: form.value.pusher.trim(),
          task_ref: form.value.taskRef.trim(),
          detail: form.value.detail.trim(),
          artifacts: [...rels],
          notes: notes(),
        })
        toast.success(t('workspaces.remote.queued'))
        const status = await followJob(job.rec_id)
        if (status === 'succeeded') {
          ok += 1
          successItems.value = [...successItems.value, ...collectSuccess(p, rels)]
          append(t('workspaces.autoReleaseFtpOk', { name: p.project_name }), 'success')
        } else {
          failed += 1
          append(t('workspaces.autoReleaseFtpFailed', { name: p.project_name, msg: status }), 'error')
        }
      } catch (e) {
        failed += 1
        append(t('workspaces.autoReleaseFtpFailed', { name: p.project_name, msg: errText(e) }), 'error')
      }
    }
    if (!cancelled) {
      append(t('workspaces.autoPushDone', { succeeded: ok, failed }), pushDoneLevel(ok, failed))
      if (failed > 0) toast.warning(t('workspaces.autoPushPartial', { succeeded: ok, failed }))
      else toast.success(t('workspaces.autoPushSuccess', { count: ok }))
      if (ok > 0) void openNotify()
      emit('success')
    }
  } finally {
    running.value = false
  }
}

watch(open, (visible) => {
  if (visible) void onOpen()
  else cancelled = true
})

watch(selectedIds, () => {
  if (open.value) syncSelectedProjects()
}, { deep: true })

watch(
  () => [form.value.pushPath, previewFileName.value],
  () => syncBackupDefault(),
)

watch(
  () => [open.value, form.value.envRecId, form.value.pushPath, previewFileName.value, noteProjectId.value],
  () => scheduleRemoteNote(),
)

onUnmounted(() => {
  followGen += 1
  noteGen += 1
  cancelled = true
  if (noteTimer) clearTimeout(noteTimer)
})
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="window"
    width="80%"
    height="80%"
    draggable
    resizable
    fullscreenable
    teleport-to="body"
    :title="t('workspaces.autoPushTitle')"
    :description="t('workspaces.autoPushHint', { name: workspace?.workspace_name || '', count: workspace?.project_count ?? projects.length })"
  >
    <template #body>
      <div class="remote-split">
        <div class="form-col">
          <RsFormItem :label="t('workspaces.autoPushProjects')" required>
            <RsSelect
              v-model="selectedIds"
              multiple
              searchable
              :options="projectOptions"
              :disabled="running"
              :placeholder="t('workspaces.autoPushProjectsPh')"
            />
          </RsFormItem>
          <section class="cards" :aria-label="t('workspaces.autoPushPicked', { n: queue.length })">
            <header v-if="queue.length" class="cards-head">{{ t('workspaces.autoPushPicked', { n: queue.length }) }}</header>
            <RsEmpty v-if="!queue.length" :description="t('workspaces.autoPushEmptyQueue')" />
            <RsCard
              v-for="row in queue"
              :key="row.rec_id"
              class="proj-card"
              size="sm"
              variant="outlined"
              radius="sm"
              :title="row.project_name"
            >
              <template #actions>
                <RsTag :variant="typeVariant(row.project_type)" round>{{ typeLabel(row.project_type) }}</RsTag>
                <RsButton size="sm" variant="text" tone="danger" :disabled="running" @click="removeProject(String(row.rec_id))">
                  {{ t('workspaces.autoPushRemove') }}
                </RsButton>
              </template>
              <ProjectPushCard
                :model-value="projectPaths[String(row.rec_id)] ?? []"
                :project-id="String(row.rec_id)"
                :disabled="running"
                @update:model-value="setProjectPaths(String(row.rec_id), $event)"
              />
            </RsCard>
          </section>
          <PushPanel
            v-model:form="form"
            :entries="previewEntries"
            :env-options="envOptions"
            :disabled="running"
            :remote-note="remoteNote"
            :remote-note-state="remoteNoteState"
            :show-targets="false"
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
            v-model:lines="logLines"
            class="log"
            height="100%"
            :follow="running"
            :empty-text="t('workspaces.autoPushLogEmpty')"
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
      <RsButton :loading="running" :disabled="!canSubmit" @click="submit">{{ t('workspaces.autoPushExecute') }}</RsButton>
    </template>
  </RsDialog>
  <MailPanel v-model:open="notifyOpen" :initial="notifyInitial" :smtp-options="smtpOptions" />
</template>

<style scoped src="../components/remote/dialog.css"></style>
<style scoped>
.remote-split {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(24rem, 34rem) minmax(0, 1fr);
  gap: 1rem;
}

.cards {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  min-height: 12rem;
  max-height: min(28rem, 52vh);
  overflow: auto;
}

.cards-head {
  font-size: 0.82rem;
  font-weight: 650;
  color: var(--shell-title);
}

.proj-card {
  min-width: 0;
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
