<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsAlert, RsButton, RsCheckbox, RsDialog, RsFieldset, RsFormItem, RsInput, RsLog, RsTag, useRsToast } from '@/ui'
import { useAuthStore } from '@/stores/auth'
import type { Workspace } from '@/stores/workspace'
import {
  autoReleaseDefaults,
  autoReleaseHistory,
  autoReleaseLog,
  autoReleaseStatus,
  errText,
  fetchMavenOrder,
  fetchRemoteNote,
  getWorkspaceEnv,
  listProjects,
  listWorkspaceEnvs,
  saveAutoReleasePostOptions,
  startAutoRelease,
  submitJob,
  waitForJob,
  RELEASE_OK,
  type AutoReleaseStatus,
  type Project,
  type ProjectFileEntry,
  type ReleasePostOptions,
  type WorkspaceReleaseRecord,
} from '../api'
import { typeLabel } from '../composables/use-project-list'
import { isGitBranch } from '../lib/path-slug'
import { orderProjectsForBuild } from '../lib/ordered-build'
import { defaultBuild, parseRecipe, projectTypes } from '../lib/project-defaults'
import { defaultBackupDir } from '../lib/workspace-ftp'
import { syncSourcesWithLog } from '../lib/sync-log'
import { setAutoReleaseBusy, useBatchActivity } from '../composables/use-batch-activity'
import PushPanel from '../components/remote/PushPanel.vue'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ workspace?: Workspace | null }>()
const emit = defineEmits<{ success: [] }>()

const { t } = useI18n()
const toast = useRsToast()
const auth = useAuthStore()
const releaseBranch = ref('')
const tagBranch = ref('')
const releaseNote = ref('')
const releaseDate = ref('')
const updatePomVersion = ref(true)
const createTargetBranch = ref(false)
const targetBranch = ref('')
const autoBuildAfterRelease = ref(true)
const autoFtpPushAfterBuild = ref(true)
const ftpPushProjectIds = ref<string[]>([])
const pushForm = ref(emptyPushForm())
const backupManual = ref(false)
const remoteNote = ref('')
const remoteNoteState = ref<'idle' | 'loading' | 'exists' | 'missing' | 'error'>('idle')
const envOptions = ref<{ value: string; label: string }[]>([])
const projects = ref<Project[]>([])
const logText = ref('')
const logOutcome = ref<{ kind: ReleaseOutcomeKind; text: string }>({ kind: 'idle', text: '' })
const releasing = ref(false)
const postProcessing = ref(false)
const saving = ref(false)
const history = ref<WorkspaceReleaseRecord[]>([])
const selectedHistory = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null
let handledPostFor = ''
let applyingPost = false
let noteGen = 0
let noteTimer: ReturnType<typeof setTimeout> | undefined

type ReleaseOutcomeKind = 'idle' | 'running' | 'success' | 'partial' | 'failed' | 'foreign'

const logInferMarkers = {
  success: [/^\s*\[成功\]/],
  warn: [/^\s*\[部分失败\]/, /^\s*\[跳过\]/],
  error: [/^\s*\[失败\]/],
  info: [/^\s*\[进行中\]/],
}

const ftpEnabled = computed(() => autoBuildAfterRelease.value && autoFtpPushAfterBuild.value)
const outcomeTag = computed(() => {
  const kind = logOutcome.value.kind
  if (kind === 'idle') return ''
  return t(`workspaces.autoReleaseOutcomeTags.${kind}`)
})
const outcomeVariant = computed(() => {
  const kind = logOutcome.value.kind
  if (kind === 'success') return 'success'
  if (kind === 'partial') return 'warning'
  if (kind === 'failed' || kind === 'foreign') return 'danger'
  if (kind === 'running') return 'info'
  return 'default'
})
const enabledProjects = computed(() => projects.value.filter((p) => p.is_enabled !== 0))
const pickedSet = computed(() => new Set(ftpPushProjectIds.value))
const typeGroups = computed(() => {
  const by = new Map<string, Project[]>()
  for (const p of enabledProjects.value) {
    const key = p.project_type || 'custom'
    const list = by.get(key) ?? []
    list.push(p)
    by.set(key, list)
  }
  const known = projectTypes.map((item) => item.value)
  const keys = [...known.filter((key) => by.has(key)), ...[...by.keys()].filter((key) => !known.includes(key as (typeof known)[number]))]
  return keys.map((type) => {
    const rows = [...(by.get(type) ?? [])].sort((a, b) => a.project_name.localeCompare(b.project_name))
    const picked = rows.filter((p) => pickedSet.value.has(p.rec_id)).length
    return { type, label: typeLabel(type), rows, picked }
  })
})
const previewEntries = computed<ProjectFileEntry[]>(() =>
  enabledProjects.value.filter((p) => pickedSet.value.has(p.rec_id)).map(artifactEntry),
)
const previewFileName = computed(() => previewEntries.value[0]?.name || 'package')
const noteProjectId = computed(() => ftpPushProjectIds.value[0] || '')
const { buildBusy, releaseBusy, refresh: refreshBatch } = useBatchActivity(() => props.workspace?.rec_id || '')
const occupyTip = computed(() => {
  if (buildBusy.value) return t('workspaces.autoBatchBusyBuild')
  if (releaseBusy.value || releasing.value || postProcessing.value) return t('workspaces.autoBatchBusyRelease')
  return ''
})
const startBlocked = computed(() => Boolean(occupyTip.value) && !releasing.value && !postProcessing.value)
const canSubmit = computed(() => {
  if (startBlocked.value || releasing.value || postProcessing.value || saving.value) return false
  if (!isGitBranch(releaseBranch.value) || !tagBranch.value.trim()) return false
  if (createTargetBranch.value && !isGitBranch(targetBranch.value)) return false
  if (ftpEnabled.value && (!pushForm.value.envRecId || !pushForm.value.pushPath.trim() || !pushForm.value.pusher.trim() || !pushForm.value.taskRef.trim() || !ftpPushProjectIds.value.length)) {
    return false
  }
  return true
})

function emptyPushForm() {
  return { envRecId: '', pushPath: '', backupPath: '', pusher: '', taskRef: '', detail: '' }
}

function todayISO(): string {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function noteFromTag(tag: string): string {
  const v = tag.trim()
  return v ? `${v}自动发版说明` : ''
}

function artifactEntry(row: Project): ProjectFileEntry {
  const recipe = parseRecipe(row.recipe_json, row.project_type)
  const built = defaultBuild(row.project_type)
  const dir = (recipe.output_dir || built.output_dir || 'dist').trim()
  const pattern = (recipe.artifact_pattern || built.artifact_pattern || '*').trim()
  const name = pattern.includes('*') ? (row.project_type === 'maven' ? 'app.jar' : dir) : pattern
  return { name, path: `${dir}/${name}`, is_dir: pattern === '*' || !name.includes('.') }
}

function collectPost(): ReleasePostOptions {
  return {
    auto_build_after_release: autoBuildAfterRelease.value,
    auto_ftp_push_after_build: autoFtpPushAfterBuild.value,
    ftp_push_project_ids: [...ftpPushProjectIds.value],
    ftp_env_rec_id: pushForm.value.envRecId,
    push_path: pushForm.value.pushPath,
    backup_path: pushForm.value.backupPath,
    backup_manual: backupManual.value,
    pusher: pushForm.value.pusher,
    task_ref: pushForm.value.taskRef,
    detail: pushForm.value.detail,
    release_date: releaseDate.value,
    update_pom_version: updatePomVersion.value,
    create_target_branch: createTargetBranch.value,
    target_branch: createTargetBranch.value ? targetBranch.value.trim() : undefined,
  }
}

function applyPost(opts?: ReleasePostOptions): void {
  if (!opts) return
  applyingPost = true
  autoBuildAfterRelease.value = opts.auto_build_after_release
  autoFtpPushAfterBuild.value = opts.auto_ftp_push_after_build
  ftpPushProjectIds.value = [...(opts.ftp_push_project_ids ?? [])]
  pushForm.value.envRecId = opts.ftp_env_rec_id ?? ''
  pushForm.value.pushPath = opts.push_path ?? ''
  pushForm.value.backupPath = opts.backup_path ?? ''
  pushForm.value.pusher = opts.pusher ?? ''
  pushForm.value.taskRef = opts.task_ref ?? ''
  pushForm.value.detail = opts.detail ?? ''
  backupManual.value = Boolean(opts.backup_manual)
  if (opts.release_date?.trim()) releaseDate.value = opts.release_date.trim()
  if (opts.update_pom_version != null) updatePomVersion.value = opts.update_pom_version
  if (opts.create_target_branch != null) {
    createTargetBranch.value = opts.create_target_branch
    targetBranch.value = opts.target_branch ?? ''
  }
  void nextTick(() => {
    applyingPost = false
  })
}

function toggleProject(id: string, on: boolean): void {
  if (on) {
    if (!pickedSet.value.has(id)) ftpPushProjectIds.value = [...ftpPushProjectIds.value, id]
    return
  }
  ftpPushProjectIds.value = ftpPushProjectIds.value.filter((item) => item !== id)
}

function setTypePicked(type: string, on: boolean): void {
  const ids = enabledProjects.value.filter((p) => (p.project_type || 'custom') === type).map((p) => p.rec_id)
  if (on) {
    ftpPushProjectIds.value = [...new Set([...ftpPushProjectIds.value, ...ids])]
    return
  }
  const drop = new Set(ids)
  ftpPushProjectIds.value = ftpPushProjectIds.value.filter((id) => !drop.has(id))
}

function outcomeBracket(kind: ReleaseOutcomeKind): string {
  if (kind === 'running') return '[进行中]'
  if (kind === 'success') return '[成功]'
  if (kind === 'partial') return '[部分失败]'
  if (kind === 'failed') return '[失败]'
  return ''
}

function outcomeKind(released: number, failed: number, running: boolean): ReleaseOutcomeKind {
  if (running) return 'running'
  if (failed === 0) return 'success'
  return released === 0 ? 'failed' : 'partial'
}

function outcomeText(kind: ReleaseOutcomeKind, counts: { released: number; skipped: number; failed: number; tag?: string; done?: number; total?: number; project?: string; foreignName?: string }): string {
  if (kind === 'foreign') return t('workspaces.autoReleaseForeign', { name: counts.foreignName || '—' })
  if (kind === 'running') {
    return t('workspaces.autoReleaseOutcomeRunning', {
      released: counts.released,
      skipped: counts.skipped,
      failed: counts.failed,
      done: counts.done ?? 0,
      total: counts.total ?? 0,
      project: counts.project || '—',
    })
  }
  if (kind === 'failed') return t('workspaces.autoReleaseFailed', { released: counts.released, skipped: counts.skipped, failed: counts.failed })
  if (kind === 'partial') return t('workspaces.autoReleasePartial', { released: counts.released, skipped: counts.skipped, failed: counts.failed })
  if (kind === 'success') return t('workspaces.autoReleaseSuccess', { released: counts.released, skipped: counts.skipped, tag: counts.tag || '—' })
  return ''
}

function applyOutcome(kind: ReleaseOutcomeKind, counts: { released?: number; skipped?: number; failed?: number; tag?: string; done?: number; total?: number; project?: string; foreignName?: string }): string {
  const next = {
    released: counts.released ?? 0,
    skipped: counts.skipped ?? 0,
    failed: counts.failed ?? 0,
    tag: counts.tag,
    done: counts.done,
    total: counts.total,
    project: counts.project,
    foreignName: counts.foreignName,
  }
  const text = outcomeText(kind, next)
  logOutcome.value = { kind, text }
  const tag = outcomeBracket(kind)
  return tag && text ? `${tag} ${text}` : text || tag
}

function applyOutcomeFromStatus(st: AutoReleaseStatus): string {
  if (st.foreign) return applyOutcome('foreign', { foreignName: st.active_workspace_name })
  const running = st.status === 'running'
  const kind = outcomeKind(st.released ?? 0, st.failed ?? 0, running)
  return applyOutcome(kind, {
    released: st.released,
    skipped: st.skipped,
    failed: st.failed,
    tag: st.tag || tagBranch.value,
    done: st.done_count,
    total: st.total_count,
    project: st.current_project,
  })
}

function applyOutcomeFromRecord(item: WorkspaceReleaseRecord): string {
  let kind = outcomeKind(item.released ?? 0, item.failed ?? 0, item.status === 'running')
  if (item.status === 'failed') kind = 'failed'
  else if (item.status === 'partial') kind = 'partial'
  else if (item.status === 'success') kind = 'success'
  return applyOutcome(kind, {
    released: item.released,
    skipped: item.skipped,
    failed: item.failed,
    tag: item.tag_branch,
  })
}

function withOutcomePrefix(content: string, headline: string): string {
  const head = headline.trim()
  if (!head) return content
  const first = (content.split('\n', 1)[0] ?? '').trim()
  if (!first) return head
  if (first === head || /^\[(成功|失败|部分失败|进行中)\]/.test(first)) return content
  return `${head}\n${content}`
}

function projectStatusLabel(status: string): string {
  if (status === RELEASE_OK) return '成功'
  if (status === 'skipped') return '跳过'
  return '失败'
}

function formatStatus(st: AutoReleaseStatus): string {
  const headline = applyOutcomeFromStatus(st)
  if (st.foreign) return headline
  const lines: string[] = [headline]
  for (const item of st.results ?? []) {
    lines.push(`[${projectStatusLabel(item.status)}] ${item.project_name}`)
    for (const step of item.steps ?? []) lines.push(`  · ${step}`)
    if (item.message) lines.push(`  → ${item.message}`)
  }
  return lines.join('\n')
}

function stopPoll(): void {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function runPost(st: AutoReleaseStatus, tag: string): Promise<void> {
  const recId = st.release_rec_id || ''
  if (!recId || handledPostFor === recId || !autoBuildAfterRelease.value || (st.released ?? 0) <= 0) return
  handledPostFor = recId
  if (ftpEnabled.value && (!pushForm.value.envRecId || !pushForm.value.pushPath.trim() || !pushForm.value.pusher.trim() || !pushForm.value.taskRef.trim() || !ftpPushProjectIds.value.length)) {
    append(`[${new Date().toLocaleString()}] ${t('workspaces.autoReleaseFtpConfigRequired')}`)
    return
  }
  postProcessing.value = true
  try {
    append(`\n[${new Date().toLocaleString()}] ${t('workspaces.autoReleaseBuildStart')}`)
    const wsId = props.workspace?.rec_id || ''
    if (!projects.value.length) projects.value = await listProjects(wsId)
    const byName = new Map(projects.value.map((p) => [p.project_name, p]))
    const releasedIds = (st.results ?? [])
      .filter((r) => r.status === RELEASE_OK)
      .map((r) => byName.get(r.project_name)?.rec_id)
      .filter((id): id is string => !!id)
    const releasedSet = new Set(releasedIds)
    const released = releasedIds.map((id) => projects.value.find((p) => p.rec_id === id)).filter((p): p is Project => !!p)
    const pushSet = new Set(ftpPushProjectIds.value)
    let queue = released
    try {
      await syncSourcesWithLog(wsId, t, append)
      const analysis = await fetchMavenOrder(wsId)
      const ordered = orderProjectsForBuild(projects.value, analysis.full_build_order ?? [], {
        targetRecIds: releasedIds,
        edges: analysis.internal_edges,
      })
      if (ordered.length) {
        queue = ordered
        append(t('workspaces.autoBuildPlanLine', { count: ordered.length }))
        ordered.forEach((p, i) => append(`  ${i + 1}. ${p.project_name}`))
      }
    } catch (e) {
      append(errText(e))
    }
    for (let i = 0; i < queue.length; i++) {
      const p = queue[i]
      append(`[${new Date().toLocaleString()}] ${t('workspaces.autoReleaseBuildProject', { name: p.project_name, num: i + 1, total: queue.length })}`)
      let built = false
      try {
        const job = await submitJob(wsId, p.rec_id, 'build')
        const done = await waitForJob(job.rec_id)
        if (done.job_status === 'succeeded') {
          built = true
          append(t('workspaces.autoReleaseBuildOk', { name: p.project_name, num: 0 }))
        } else {
          append(t('workspaces.autoReleaseBuildFailed', { name: p.project_name, msg: done.error_text || done.job_status }))
        }
      } catch (e) {
        append(t('workspaces.autoReleaseBuildFailed', { name: p.project_name, msg: errText(e) }))
      }
      if (!ftpEnabled.value || !built || !releasedSet.has(p.rec_id)) continue
      if (!pushSet.has(p.rec_id)) {
        append(t('workspaces.autoReleaseFtpSkipped', { name: p.project_name }))
        continue
      }
      append(t('workspaces.autoReleaseFtpProject', { name: p.project_name }))
      try {
        const job = await submitJob(wsId, p.rec_id, 'ftp_push', {
          env_rec_id: pushForm.value.envRecId,
          remote_path: pushForm.value.pushPath.trim(),
          backup_path: pushForm.value.backupPath.trim(),
          pusher: pushForm.value.pusher.trim(),
          task_ref: pushForm.value.taskRef.trim(),
          detail: pushForm.value.detail.trim(),
          notes: [pushForm.value.taskRef.trim() || noteFromTag(tag), pushForm.value.detail.trim()].filter(Boolean).join(' | '),
        })
        const done = await waitForJob(job.rec_id)
        append(done.job_status === 'succeeded' ? t('workspaces.autoReleaseFtpOk', { name: p.project_name }) : t('workspaces.autoReleaseFtpFailed', { name: p.project_name, msg: done.error_text || done.job_status }))
      } catch (e) {
        append(t('workspaces.autoReleaseFtpFailed', { name: p.project_name, msg: errText(e) }))
      }
    }
    append(t('workspaces.autoReleasePostDone'))
    emit('success')
  } finally {
    postProcessing.value = false
  }
}

function append(line: string): void {
  logText.value = logText.value ? `${logText.value}\n${line}` : line
}

async function pollOnce(showToast: boolean): Promise<boolean> {
  const id = props.workspace?.rec_id
  if (!id) return true
  const st = await autoReleaseStatus(id)
  if (st.status === 'running') {
    releasing.value = !st.foreign
    if (!st.foreign) {
      logText.value = formatStatus(st)
      if (st.post_options) applyPost(st.post_options)
    } else {
      logText.value = formatStatus(st)
    }
    return false
  }
  stopPoll()
  releasing.value = false
  if (!st.foreign) logText.value = formatStatus(st)
  void loadHistory()
  if (st.status === 'completed') {
    const released = st.released ?? 0
    const failed = st.failed ?? 0
    const tag = st.tag || tagBranch.value
    if (showToast) {
      if (failed > 0) toast.warning(t('workspaces.autoReleasePartial', { released, skipped: st.skipped ?? 0, failed }))
      else toast.success(t('workspaces.autoReleaseSuccess', { released, skipped: st.skipped ?? 0, tag }))
    }
    if (released > 0 && autoBuildAfterRelease.value) {
      void runPost(st, tag)
    } else if (released > 0) {
      emit('success')
    }
  }
  return true
}

function startPoll(showToast: boolean): void {
  stopPoll()
  void pollOnce(showToast)
  pollTimer = setInterval(() => {
    void pollOnce(showToast)
  }, 2000)
}

async function loadHistory(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id) return
  try {
    history.value = await autoReleaseHistory(id)
  } catch {
    history.value = []
  }
}

function historyTone(status?: string): 'ok' | 'fail' | 'run' | 'warn' | 'muted' {
  if (status === 'success') return 'ok'
  if (status === 'failed') return 'fail'
  if (status === 'partial') return 'warn'
  if (status === 'running') return 'run'
  return 'muted'
}

function historyTime(row: WorkspaceReleaseRecord): string {
  const value = row.started_at || row.finished_at
  if (!value) return ''
  const iso = value.includes('T') ? value : value.replace(' ', 'T')
  const zoned = /(?:[zZ]|[+-]\d{2}:?\d{2})$/.test(iso)
  const d = new Date(zoned ? iso : `${iso}Z`)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function loadHistoryLog(recId: string): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id) return
  selectedHistory.value = recId
  try {
    const view = await autoReleaseLog(id, recId)
    const row = history.value.find((item) => item.rec_id === recId)
    const headline = row ? applyOutcomeFromRecord(row) : ''
    logText.value = withOutcomePrefix(view.content, headline)
  } catch (e) {
    toast.error(errText(e))
  }
}

function syncBackupDefault(): void {
  if (backupManual.value || !pushForm.value.pushPath.trim()) return
  pushForm.value.backupPath = defaultBackupDir(pushForm.value.pushPath, previewFileName.value)
}

async function onEnvChange(id: string): Promise<void> {
  if (!id || pushForm.value.pushPath.trim()) return
  try {
    const row = await getWorkspaceEnv(id)
    const cfg = row.config ?? {}
    const base = String(cfg.remote_dir ?? cfg.remote_path ?? cfg.base_path ?? '').trim() || '/'
    pushForm.value.pushPath = base === '/' ? '/update' : base
    syncBackupDefault()
  } catch {
    /* keep path */
  }
}

function scheduleRemoteNote(): void {
  if (noteTimer) clearTimeout(noteTimer)
  const env = pushForm.value.envRecId
  const path = pushForm.value.pushPath.trim()
  if (!open.value || !ftpEnabled.value || !noteProjectId.value || !env || !path) {
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
      env_rec_id: pushForm.value.envRecId,
      push_path: pushForm.value.pushPath.trim(),
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

async function onOpen(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id) return
  void refreshBatch()
  handledPostFor = ''
  selectedHistory.value = ''
  logText.value = ''
  logOutcome.value = { kind: 'idle', text: '' }
  releaseDate.value = todayISO()
  try {
    projects.value = await listProjects(id)
    const envs = await listWorkspaceEnvs(id)
    envOptions.value = envs
      .filter((e) => e.is_enabled !== 0 && (e.env_kind === 'ftp' || e.env_kind === 'sftp' || e.env_kind === 'ssh'))
      .map((e) => ({ value: e.rec_id, label: `${e.env_name} (${t(`envs.kinds.${e.env_kind}`)})` }))
    const defaults = await autoReleaseDefaults(id)
    releaseBranch.value = defaults.release_branch
    tagBranch.value = defaults.tag_branch
    releaseNote.value = noteFromTag(defaults.tag_branch)
    if (!pushForm.value.pusher.trim()) pushForm.value.pusher = (auth.me?.display_name || auth.me?.login_name || '').trim()
    if (!pushForm.value.taskRef.trim()) pushForm.value.taskRef = releaseNote.value
    await loadHistory()
    const st = await autoReleaseStatus(id)
    if (st.status === 'running') {
      if (st.release_branch) releaseBranch.value = st.release_branch
      if (st.tag) {
        tagBranch.value = st.tag
        releaseNote.value = noteFromTag(st.tag)
      }
      applyPost(st.post_options)
      startPoll(false)
    }
  } catch (e) {
    toast.error(errText(e))
  }
}

async function submit(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id || !canSubmit.value) return
  saving.value = true
  try {
    await startAutoRelease({
      workspace_rec_id: id,
      release_branch: releaseBranch.value.trim(),
      tag_branch: tagBranch.value.trim(),
      release_note: releaseNote.value.trim(),
      release_date: releaseDate.value.trim() || todayISO(),
      allow_repeat: true,
      update_pom_version: updatePomVersion.value,
      create_target_branch: createTargetBranch.value,
      target_branch: createTargetBranch.value ? targetBranch.value.trim() : undefined,
      post_options: collectPost(),
    })
    const startLine = applyOutcome('running', { tag: tagBranch.value })
    logText.value = `${startLine}\n${t('workspaces.autoReleaseStart')}`
    startPoll(true)
  } catch (e) {
    toast.error(errText(e))
  } finally {
    saving.value = false
  }
}

watch(tagBranch, (tag) => {
  releaseNote.value = noteFromTag(tag)
  if (!pushForm.value.taskRef.trim() && tag.trim()) pushForm.value.taskRef = noteFromTag(tag)
})
watch(autoBuildAfterRelease, (on) => {
  if (!on) autoFtpPushAfterBuild.value = false
})
watch(open, (v) => {
  if (v) void onOpen()
  else stopPoll()
})
watch(
  () => [
    autoBuildAfterRelease.value,
    autoFtpPushAfterBuild.value,
    pushForm.value.envRecId,
    pushForm.value.pushPath,
    pushForm.value.backupPath,
    pushForm.value.pusher,
    pushForm.value.taskRef,
    pushForm.value.detail,
    ftpPushProjectIds.value,
  ],
  () => {
    if (!open.value || applyingPost || (!releasing.value && !postProcessing.value)) return
    void saveAutoReleasePostOptions(props.workspace?.rec_id || '', collectPost()).catch(() => undefined)
  },
)
watch(
  () => [open.value, ftpEnabled.value, pushForm.value.envRecId, pushForm.value.pushPath, previewFileName.value, noteProjectId.value],
  () => scheduleRemoteNote(),
)
watch(
  () => [pushForm.value.pushPath, previewFileName.value],
  () => syncBackupDefault(),
)
watch(
  [releasing, postProcessing, () => props.workspace?.rec_id],
  ([run, post, id], prev) => {
    const prevId = String(prev?.[2] ?? '')
    if (prevId && prevId !== id) setAutoReleaseBusy(prevId, false)
    if (id) setAutoReleaseBusy(id, Boolean(run || post))
  },
)
onUnmounted(() => {
  stopPoll()
  if (noteTimer) clearTimeout(noteTimer)
})
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
    :title="t('workspaces.autoReleaseTitle')"
    :description="t('workspaces.autoReleaseHint', { name: workspace?.workspace_name || '', count: workspace?.project_count ?? projects.length })"
  >
    <template #body>
      <RsAlert v-if="occupyTip" type="warning" class="occupy-alert">{{ occupyTip }}</RsAlert>
      <div class="auto-split">
        <div class="form-col">
          <RsFieldset
            :legend="t('workspaces.autoReleaseSectionGit')"
            :tooltip="t('workspaces.autoReleaseSectionGitDesc')"
            :disabled="releasing"
            border-style="dashed"
            title-tone="muted"
            size="sm"
          >
            <div class="pair">
              <RsFormItem :label="t('workspaces.autoReleaseBranch')" :tooltip="t('workspaces.autoReleaseBranchTip')" required>
                <RsInput v-model="releaseBranch" :placeholder="t('workspaces.autoReleaseBranchPh')" />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.autoReleaseTag')" :tooltip="t('workspaces.autoReleaseTagTip')" required>
                <RsInput v-model="tagBranch" :placeholder="t('workspaces.autoReleaseTagPh')" />
              </RsFormItem>
            </div>
            <div class="pair">
              <RsFormItem :label="t('workspaces.autoReleaseNote')" :tooltip="t('workspaces.autoReleaseNoteTip')">
                <RsInput v-model="releaseNote" readonly />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.autoReleaseDate')" :tooltip="t('workspaces.autoReleaseDateTip')">
                <RsInput v-model="releaseDate" />
              </RsFormItem>
            </div>
          </RsFieldset>
          <RsFieldset
            :legend="t('workspaces.autoReleaseSectionOpts')"
            :tooltip="t('workspaces.autoReleaseSectionOptsDesc')"
            :disabled="releasing"
            border-style="dashed"
            title-tone="muted"
            size="sm"
          >
            <RsCheckbox v-model="updatePomVersion">{{ t('workspaces.autoReleaseUpdatePom') }}</RsCheckbox>
            <p class="hint">{{ t('workspaces.autoReleaseUpdatePomTip') }}</p>
            <RsCheckbox v-model="createTargetBranch">{{ t('workspaces.autoReleaseCreateTarget') }}</RsCheckbox>
            <p class="hint">{{ t('workspaces.autoReleaseCreateTargetTip') }}</p>
            <RsFormItem
              v-if="createTargetBranch"
              :label="t('workspaces.autoReleaseTarget')"
              :tooltip="t('workspaces.autoReleaseTargetTip')"
              required
            >
              <RsInput v-model="targetBranch" :placeholder="t('workspaces.newBranchPh')" />
            </RsFormItem>
          </RsFieldset>
          <RsFieldset
            :legend="t('workspaces.autoReleaseSectionPost')"
            :tooltip="t('workspaces.autoReleaseSectionPostDesc')"
            border-style="dashed"
            title-tone="muted"
            size="sm"
          >
            <RsCheckbox v-model="autoBuildAfterRelease" :disabled="releasing && !postProcessing">{{ t('workspaces.autoReleaseAutoBuild') }}</RsCheckbox>
            <p class="hint">{{ t('workspaces.autoReleaseAutoBuildTip') }}</p>
            <RsCheckbox v-model="autoFtpPushAfterBuild" :disabled="!autoBuildAfterRelease">{{ t('workspaces.autoReleaseAutoFtp') }}</RsCheckbox>
            <p class="hint">{{ t('workspaces.autoReleaseAutoFtpTip') }}</p>
          </RsFieldset>
          <template v-if="ftpEnabled">
            <RsFieldset
              v-for="group in typeGroups"
              :key="group.type"
              :legend="group.label"
              :tooltip="t('workspaces.autoReleaseFtpByTypeTip')"
              :disabled="releasing && !postProcessing"
              border-style="dashed"
              title-tone="muted"
              size="sm"
            >
              <template #extra>
                <span>{{ group.picked }}/{{ group.rows.length }}</span>
                <RsButton size="sm" variant="text" @click="setTypePicked(group.type, group.picked < group.rows.length)">
                  {{ group.picked < group.rows.length ? t('workspaces.autoReleaseFtpSelectAll') : t('workspaces.autoReleaseFtpClear') }}
                </RsButton>
              </template>
              <label v-for="row in group.rows" :key="row.rec_id" class="pick">
                <RsCheckbox
                  :model-value="pickedSet.has(row.rec_id)"
                  @update:model-value="toggleProject(row.rec_id, Boolean($event))"
                />
                <span>{{ row.project_name }}</span>
              </label>
            </RsFieldset>
            <PushPanel
              v-model:form="pushForm"
              :entries="previewEntries"
              :env-options="envOptions"
              :disabled="releasing && !postProcessing"
              :remote-note="remoteNote"
              :remote-note-state="remoteNoteState"
              :show-targets="false"
              @backup-edit="backupManual = true"
              @env-change="onEnvChange"
            />
          </template>
          <RsFieldset
            v-if="history.length"
            :legend="t('workspaces.autoReleaseSectionHist')"
            :tooltip="t('workspaces.autoReleaseSectionHistDesc')"
            border-style="dashed"
            title-tone="muted"
            size="sm"
          >
            <ol class="history">
              <li v-for="item in history" :key="item.rec_id">
                <button
                  type="button"
                  class="history__row"
                  :class="{ 'is-on': selectedHistory === item.rec_id }"
                  @click="loadHistoryLog(item.rec_id)"
                >
                  <span class="history__no" :class="`is-${historyTone(item.status)}`">#{{ item.release_num }}</span>
                  <span class="history__time">{{ historyTime(item) }}</span>
                </button>
              </li>
            </ol>
          </RsFieldset>
        </div>
        <div class="log-col">
          <header class="sec-head log-head">
            <h3>{{ t('workspaces.autoReleaseSectionLog') }}</h3>
            <RsTag v-if="logOutcome.kind !== 'idle'" :variant="outcomeVariant" round>{{ outcomeTag }}</RsTag>
          </header>
          <p class="log-outcome" :data-kind="logOutcome.kind">{{ logOutcome.text || t('workspaces.autoReleaseLogEmpty') }}</p>
          <RsLog
            v-model:lines="logText"
            class="log"
            wrap
            height="100%"
            :infer-markers="logInferMarkers"
            :empty-text="t('workspaces.autoReleaseLogEmpty')"
            :aria-label="t('workspaces.autoReleaseSectionLog')"
          />
        </div>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" :disabled="saving || startBlocked" @click="open = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="saving || releasing" :disabled="!canSubmit" @click="submit">{{ t('workspaces.autoReleaseExecute') }}</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped src="./dialog.css"></style>
<style scoped>
.occupy-alert {
  flex: 0 0 auto;
  margin-bottom: 0.75rem;
}
.auto-split {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(22rem, 38rem) minmax(0, 1fr);
  gap: 1rem;
}
.form-col,
.log-col {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}
.form-col {
  overflow: auto;
  padding-right: 0.15rem;
}
.pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.7rem;
}
.pick {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
  font-size: 0.82rem;
}
.sec-head h3 {
  margin: 0 0 0.25rem;
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--shell-title);
}
.log-head {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}
.log-head h3 {
  margin: 0;
}
.log-outcome {
  margin: 0;
  font-size: 0.82rem;
  line-height: 1.45;
  font-weight: 650;
  color: var(--shell-title);
}
.log-outcome[data-kind='success'] {
  color: var(--rs-success, #15803d);
}
.log-outcome[data-kind='partial'] {
  color: var(--rs-warning, #b45309);
}
.log-outcome[data-kind='failed'],
.log-outcome[data-kind='foreign'] {
  color: var(--rs-danger, #b91c1c);
}
.log-outcome[data-kind='running'] {
  color: var(--rs-info, #0369a1);
}
.log-outcome[data-kind='idle'] {
  font-weight: 400;
  color: var(--shell-muted);
}
.log {
  flex: 1;
  min-height: 16rem;
  height: 0;
}
.history {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.22rem;
  max-height: 16rem;
  overflow: auto;
}
.history__row {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  padding: 0.28rem 0.4rem;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: transparent;
  text-align: left;
  cursor: pointer;
  outline: none;
  box-shadow: none;
}
.history__row:hover {
  background: color-mix(in srgb, #0ea5e9 7%, transparent);
}
.history__row:focus-visible {
  box-shadow: 0 0 0 2px color-mix(in srgb, #0ea5e9 22%, transparent);
}
.history__row.is-on {
  border-color: color-mix(in srgb, #0ea5e9 38%, transparent);
  background: color-mix(in srgb, #0ea5e9 10%, #fff);
}
.history__no {
  flex-shrink: 0;
  min-width: 2.35rem;
  padding: 0.08rem 0.42rem;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--rs-success, #34c759) 32%, transparent);
  background: var(--rs-success-container, #e4f8e9);
  color: var(--rs-on-success-container, #1a7f37);
  font-size: 0.78rem;
  font-weight: 650;
  text-align: center;
}
.history__no.is-fail {
  border-color: color-mix(in srgb, var(--rs-danger, #ff3b30) 28%, transparent);
  background: var(--rs-danger-container, #ffe5e3);
  color: var(--rs-on-danger-container, #c41e16);
}
.history__no.is-run {
  border-color: color-mix(in srgb, #0ea5e9 32%, transparent);
  background: #e0f2fe;
  color: #0369a1;
}
.history__no.is-warn {
  border-color: color-mix(in srgb, var(--rs-warning, #ff9500) 28%, transparent);
  background: var(--rs-warning-container, #fff3e0);
  color: var(--rs-on-warning-container, #b26500);
}
.history__no.is-muted {
  border-color: var(--shell-border);
  background: var(--shell-inset-bg);
  color: var(--shell-muted);
}
.history__time {
  color: #94a3b8;
  font-size: 0.82rem;
}
@media (max-width: 900px) {
  .auto-split,
  .pair {
    grid-template-columns: 1fr;
  }
}
</style>
