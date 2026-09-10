<script setup lang="ts">
import { computed, nextTick, onUnmounted, watch } from 'vue'
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
  fetchRemoteNote,
  getWorkspaceEnv,
  listProjects,
  listWorkspaceEnvs,
  saveAutoReleasePostOptions,
  startAutoRelease,
  RELEASE_OK,
  type AutoReleaseStatus,
  type Project,
  type ProjectFileEntry,
  type ReleasePostOptions,
  type WorkspaceReleaseRecord,
} from '../api'
import { typeLabel } from '../composables/use-project-list'
import { isGitBranch } from '../lib/path-slug'
import { defaultBuild, parseRecipe, projectTypes } from '../lib/project-defaults'
import { defaultBackupDir } from '../lib/workspace-ftp'
import { setAutoReleaseBusy, useBatchActivity } from '../composables/use-batch-activity'
import { autoReleaseBag, isAutoReleaseLive, stopAutoReleasePolls, type AutoReleaseBag, type ReleaseOutcomeKind } from './runtime'
import PushPanel from '../components/remote/PushPanel.vue'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ workspace?: Workspace | null }>()
const emit = defineEmits<{ success: [] }>()

const { t } = useI18n()
const toast = useRsToast()
const auth = useAuthStore()
const wsId = computed(() => props.workspace?.rec_id || '')

function slot(id = wsId.value): AutoReleaseBag {
  return autoReleaseBag(id)
}

function touch(id = wsId.value): void {
  if (id) setAutoReleaseBusy(id, isAutoReleaseLive(id))
}

function bind<K extends keyof AutoReleaseBag>(key: K) {
  return computed({
    get: () => slot()[key],
    set: (v: AutoReleaseBag[K]) => {
      slot()[key] = v
    },
  })
}

const releaseBranch = bind('releaseBranch')
const tagBranch = bind('tagBranch')
const releaseNote = bind('releaseNote')
const releaseDate = bind('releaseDate')
const updatePomVersion = bind('updatePomVersion')
const createTargetBranch = bind('createTargetBranch')
const targetBranch = bind('targetBranch')
const autoBuildAfterRelease = bind('autoBuildAfterRelease')
const autoFtpPushAfterBuild = bind('autoFtpPushAfterBuild')
const ftpPushProjectIds = bind('ftpPushProjectIds')
const pushForm = bind('pushForm')
const backupManual = bind('backupManual')
const remoteNote = bind('remoteNote')
const remoteNoteState = bind('remoteNoteState')
const envOptions = bind('envOptions')
const projects = bind('projects')
const logText = bind('logText')
const logOutcome = bind('logOutcome')
const releasing = bind('releasing')
const postProcessing = bind('postProcessing')
const saving = bind('saving')
const history = bind('history')
const selectedHistory = bind('selectedHistory')
let noteGen = 0
let noteTimer: ReturnType<typeof setTimeout> | undefined

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

function applyPost(id: string, opts?: ReleasePostOptions): void {
  if (!opts) return
  const cur = slot(id)
  cur.applyingPost = true
  cur.autoBuildAfterRelease = opts.auto_build_after_release
  cur.autoFtpPushAfterBuild = opts.auto_ftp_push_after_build
  cur.ftpPushProjectIds = [...(opts.ftp_push_project_ids ?? [])]
  cur.pushForm.envRecId = opts.ftp_env_rec_id ?? ''
  cur.pushForm.pushPath = opts.push_path ?? ''
  cur.pushForm.backupPath = opts.backup_path ?? ''
  cur.pushForm.pusher = opts.pusher ?? ''
  cur.pushForm.taskRef = opts.task_ref ?? ''
  cur.pushForm.detail = opts.detail ?? ''
  cur.backupManual = Boolean(opts.backup_manual)
  if (opts.release_date?.trim()) cur.releaseDate = opts.release_date.trim()
  if (opts.update_pom_version != null) cur.updatePomVersion = opts.update_pom_version
  if (opts.create_target_branch != null) {
    cur.createTargetBranch = opts.create_target_branch
    cur.targetBranch = opts.target_branch ?? ''
  }
  void nextTick(() => {
    cur.applyingPost = false
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

function applyOutcome(id: string, kind: ReleaseOutcomeKind, counts: { released?: number; skipped?: number; failed?: number; tag?: string; done?: number; total?: number; project?: string; foreignName?: string }): string {
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
  slot(id).logOutcome = { kind, text }
  const tag = outcomeBracket(kind)
  return tag && text ? `${tag} ${text}` : text || tag
}

function applyOutcomeFromStatus(id: string, st: AutoReleaseStatus): string {
  const cur = slot(id)
  if (st.foreign) return applyOutcome(id, 'idle', {})
  const runningNow = st.status === 'running'
  const kind = outcomeKind(st.released ?? 0, st.failed ?? 0, runningNow)
  return applyOutcome(id, kind, {
    released: st.released,
    skipped: st.skipped,
    failed: st.failed,
    tag: st.tag || cur.tagBranch,
    done: st.done_count,
    total: st.total_count,
    project: st.current_project,
  })
}

function applyOutcomeFromRecord(id: string, item: WorkspaceReleaseRecord): string {
  let kind = outcomeKind(item.released ?? 0, item.failed ?? 0, item.status === 'running')
  if (item.status === 'failed') kind = 'failed'
  else if (item.status === 'partial') kind = 'partial'
  else if (item.status === 'success') kind = 'success'
  return applyOutcome(id, kind, {
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

function formatStatus(id: string, st: AutoReleaseStatus): string {
  const headline = applyOutcomeFromStatus(id, st)
  if (st.foreign) return headline
  const lines: string[] = [headline]
  for (const item of st.results ?? []) {
    lines.push(`[${projectStatusLabel(item.status)}] ${item.project_name}`)
    for (const step of item.steps ?? []) lines.push(`  · ${step}`)
    if (item.message) lines.push(`  → ${item.message}`)
  }
  return lines.join('\n')
}

function stopPoll(id: string): void {
  const cur = slot(id)
  if (cur.pollTimer) {
    clearInterval(cur.pollTimer)
    cur.pollTimer = null
  }
  touch(id)
}

async function pullLog(id: string, recId: string): Promise<void> {
  if (!id || !recId) return
  const view = await autoReleaseLog(id, recId)
  slot(id).logText = view.content
}

async function pollOnce(id: string, showToast: boolean): Promise<boolean> {
  if (!id) return true
  const cur = slot(id)
  const st = await autoReleaseStatus(id)
  if (st.foreign) {
    stopPoll(id)
    cur.releasing = false
    touch(id)
    return true
  }
  if (st.status === 'running') {
    cur.releasing = true
    applyOutcomeFromStatus(id, st)
    if (st.release_rec_id) await pullLog(id, st.release_rec_id)
    else cur.logText = formatStatus(id, st)
    if (st.post_options) applyPost(id, st.post_options)
    touch(id)
    return false
  }
  stopPoll(id)
  cur.releasing = false
  cur.postProcessing = false
  if (st.release_rec_id) await pullLog(id, st.release_rec_id)
  else cur.logText = formatStatus(id, st)
  touch(id)
  void loadHistory(id)
  if (st.status === 'completed') {
    const released = st.released ?? 0
    const failed = st.failed ?? 0
    const tag = st.tag || cur.tagBranch
    if (showToast) {
      if (failed > 0) toast.warning(t('workspaces.autoReleasePartial', { released, skipped: st.skipped ?? 0, failed }))
      else toast.success(t('workspaces.autoReleaseSuccess', { released, skipped: st.skipped ?? 0, tag }))
    }
    if (released > 0) emit('success')
  }
  return true
}

function startPoll(id: string, showToast: boolean): void {
  stopPoll(id)
  void pollOnce(id, showToast)
  const cur = slot(id)
  cur.pollTimer = setInterval(() => {
    void pollOnce(id, showToast)
  }, 2000)
  touch(id)
}

async function loadHistory(id: string): Promise<void> {
  if (!id) return
  const cur = slot(id)
  try {
    cur.history = await autoReleaseHistory(id)
  } catch {
    cur.history = []
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
  const id = wsId.value
  if (!id) return
  const cur = slot(id)
  cur.selectedHistory = recId
  try {
    const view = await autoReleaseLog(id, recId)
    const row = cur.history.find((item) => item.rec_id === recId)
    const headline = row ? applyOutcomeFromRecord(id, row) : ''
    cur.logText = withOutcomePrefix(view.content, headline)
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
  const id = wsId.value
  if (!id) return
  const cur = slot(id)
  void refreshBatch()
  if (cur.releasing || cur.postProcessing || cur.pollTimer || cur.postTask) {
    touch(id)
    if (cur.releasing && !cur.pollTimer) startPoll(id, false)
    return
  }
  cur.handledPostFor = ''
  cur.selectedHistory = ''
  cur.logText = ''
  cur.logOutcome = { kind: 'idle', text: '' }
  cur.releaseDate = todayISO()
  try {
    cur.projects = await listProjects(id)
    const envs = await listWorkspaceEnvs(id)
    cur.envOptions = envs
      .filter((e) => e.is_enabled !== 0 && (e.env_kind === 'ftp' || e.env_kind === 'sftp' || e.env_kind === 'ssh'))
      .map((e) => ({ value: e.rec_id, label: `${e.env_name} (${t(`envs.kinds.${e.env_kind}`)})` }))
    const defaults = await autoReleaseDefaults(id)
    cur.releaseBranch = defaults.release_branch
    cur.tagBranch = defaults.tag_branch
    cur.releaseNote = noteFromTag(defaults.tag_branch)
    if (!cur.pushForm.pusher.trim()) cur.pushForm.pusher = (auth.me?.display_name || auth.me?.login_name || '').trim()
    if (!cur.pushForm.taskRef.trim()) cur.pushForm.taskRef = cur.releaseNote
    await loadHistory(id)
    const st = await autoReleaseStatus(id)
    if (st.status === 'running' && !st.foreign) {
      if (st.release_branch) cur.releaseBranch = st.release_branch
      if (st.tag) {
        cur.tagBranch = st.tag
        cur.releaseNote = noteFromTag(st.tag)
      }
      applyPost(id, st.post_options)
      startPoll(id, false)
    }
  } catch (e) {
    toast.error(errText(e))
  }
}

async function submit(): Promise<void> {
  const id = wsId.value
  if (!id || !canSubmit.value) return
  const cur = slot(id)
  cur.saving = true
  try {
    await startAutoRelease({
      workspace_rec_id: id,
      release_branch: cur.releaseBranch.trim(),
      tag_branch: cur.tagBranch.trim(),
      release_note: cur.releaseNote.trim(),
      release_date: cur.releaseDate.trim() || todayISO(),
      allow_repeat: true,
      update_pom_version: cur.updatePomVersion,
      create_target_branch: cur.createTargetBranch,
      target_branch: cur.createTargetBranch ? cur.targetBranch.trim() : undefined,
      post_options: collectPost(),
    })
    const startLine = applyOutcome(id, 'running', { tag: cur.tagBranch })
    cur.logText = `${startLine}\n${t('workspaces.autoReleaseStart')}`
    startPoll(id, true)
  } catch (e) {
    toast.error(errText(e))
  } finally {
    cur.saving = false
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
})
watch(wsId, (id, prev) => {
  if (prev) setAutoReleaseBusy(prev, isAutoReleaseLive(prev))
  if (id) setAutoReleaseBusy(id, isAutoReleaseLive(id))
  if (open.value && id && id !== prev) void onOpen()
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
    const id = wsId.value
    const cur = slot(id)
    if (!open.value || cur.applyingPost || (!cur.releasing && !cur.postProcessing)) return
    void saveAutoReleasePostOptions(id, collectPost()).catch(() => undefined)
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
watch([releasing, postProcessing], () => {
  if (wsId.value) touch(wsId.value)
})
onUnmounted(() => {
  stopAutoReleasePolls()
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
