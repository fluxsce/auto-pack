<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  RsButton,
  RsCard,
  RsDialog,
  RsInput,
  RsLog,
  RsPagination,
  RsSelect,
  RsTable,
  RsTag,
  useRsToast,
  type RsSelectModelValue,
  type RsTableColumn,
} from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import { applyJobLog, getJobLog, listJobsPage, type JobRow, type JobStats } from '@/pages/workspaces/api'
import { useWorkspaceStore } from '@/stores/workspace'
import PageFrame from '@/components/PageFrame.vue'

type Job = JobRow

type JobStep = {
  rec_id: string
  step_key: string
  step_name: string
  step_status: string
  started_at?: string
  finished_at?: string
  error_text?: string
}

type Project = { rec_id: string; project_name: string }

type JobPayload = {
  tag?: string
  notes?: string
  command?: string
  pom_version?: string
}

const POLL_MS = 4000
const LIVE = new Set(['queued', 'running'])

const { t } = useI18n()
const toast = useRsToast()
const route = useRoute()
const router = useRouter()
const ws = useWorkspaceStore()
const emptyStats = (): JobStats => ({ running: 0, queued: 0, succeeded: 0, failed: 0 })

const jobs = ref<Job[]>([])
const liveRows = ref<Job[]>([])
const projects = ref<Project[]>([])
const stepsByJob = ref<Record<string, JobStep[]>>({})
const nowMs = ref(Date.now())
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const stats = ref<JobStats>(emptyStats())
const open = ref(false)
const logOpen = ref(false)
const logText = ref('')
const logSize = ref(0)
const logJobId = ref('')
const busyId = ref('')
const form = ref({
  job_type: 'build',
  project_rec_id: '',
  tag: '',
  notes: '',
  command: '',
  pom_version: '',
  update_pom: false,
})

let tickId = 0
let pollId = 0
let loadSeq = 0
let pageAlive = true

const types = computed(() => [
  { value: 'build', label: t('jobs.types.build') },
  { value: 'release', label: t('jobs.types.release') },
  { value: 'ftp_push', label: t('jobs.types.ftp_push') },
  { value: 'pom_version', label: t('jobs.types.pom_version') },
  { value: 'branch_clone', label: t('jobs.types.branch_clone') },
  { value: 'sync_sources', label: t('jobs.types.sync_sources') },
])

const projectFilter = computed(() => String(route.query.project ?? ''))

const visibleJobs = computed(() => jobs.value)

const liveJobs = computed(() => liveRows.value.filter((row) => LIVE.has(row.job_status)))

const columns = computed<RsTableColumn<Job>[]>(() => [
  { key: 'job_type', title: t('jobs.type'), width: 110 },
  { key: 'project', title: t('jobs.project'), minWidth: 140 },
  { key: 'job_status', title: t('jobs.status'), width: 120 },
  { key: 'duration', title: t('jobs.duration'), width: 130 },
  { key: 'started_at', title: t('jobs.started'), minWidth: 168 },
  { key: 'error_text', title: t('jobs.error'), minWidth: 160 },
  { key: 'actions', title: t('common.actions'), width: 168, align: 'right' },
])

function asString(value: RsSelectModelValue): string {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw == null || raw === '') return ''
  return String(typeof raw === 'object' ? raw.value : raw)
}

function typeLabel(code: string): string {
  const key = `jobs.types.${code}`
  const label = t(key)
  return label === key ? code : label
}

function statusLabel(code: string): string {
  const key = `jobs.statuses.${code}`
  const label = t(key)
  return label === key ? code : label
}

function statusVariant(code: string): 'default' | 'info' | 'success' | 'danger' | 'warning' {
  if (code === 'running') return 'info'
  if (code === 'succeeded') return 'success'
  if (code === 'failed') return 'danger'
  if (code === 'canceled' || code === 'cancelled') return 'warning'
  return 'default'
}

function parseStamp(value?: string): number | null {
  if (!value) return null
  const iso = value.includes('T') ? value : value.replace(' ', 'T')
  const zoned = /(?:[zZ]|[+-]\d{2}:?\d{2})$/.test(iso)
  const ms = new Date(zoned ? iso : `${iso}Z`).getTime()
  return Number.isNaN(ms) ? null : ms
}

function formatTime(value?: string): string {
  const ms = parseStamp(value)
  if (ms == null) return value || t('common.dash')
  return new Date(ms).toLocaleString()
}

function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return t('common.dash')
  const total = Math.floor(ms / 1000)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  if (h > 0) return t('jobs.durationHms', { h, m, s })
  if (m > 0) return t('jobs.durationMs', { m, s })
  return t('jobs.durationS', { s })
}

function elapsedMs(row: Job): number | null {
  const start = parseStamp(row.started_at) ?? (row.job_status === 'queued' ? parseStamp(row.created_at) : null)
  if (start == null) return null
  if (LIVE.has(row.job_status)) return nowMs.value - start
  const end = parseStamp(row.finished_at)
  return end == null ? null : end - start
}

function durationText(row: Job): string {
  const ms = elapsedMs(row)
  if (ms == null) return t('common.dash')
  if (row.job_status === 'queued') return t('jobs.queuedFor', { duration: formatDuration(ms) })
  if (row.job_status === 'running') return t('jobs.runningFor', { duration: formatDuration(ms) })
  return formatDuration(ms)
}

function projectName(id?: string): string {
  if (!id) return t('common.dash')
  return projects.value.find((item) => item.rec_id === id)?.project_name || id
}

function payloadOf(row: Job): JobPayload {
  if (!row.payload_json) return {}
  try {
    return JSON.parse(row.payload_json) as JobPayload
  } catch {
    return {}
  }
}

function payloadHint(row: Job): string {
  const p = payloadOf(row)
  return p.tag || p.pom_version || p.command || ''
}

function currentStep(id: string): JobStep | undefined {
  const steps = stepsByJob.value[id] ?? []
  return (
    steps.find((item) => item.step_status === 'running') ??
    steps.find((item) => item.step_status === 'pending') ??
    steps[steps.length - 1]
  )
}

function stepProgress(id: string): string {
  const steps = stepsByJob.value[id] ?? []
  if (!steps.length) return ''
  const done = steps.filter((item) => item.step_status === 'succeeded' || item.step_status === 'skipped').length
  return t('jobs.stepProgress', { done, total: steps.length })
}

function errText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

async function loadSteps(ids: string[]): Promise<void> {
  if (!ids.length) {
    stepsByJob.value = {}
    return
  }
  const entries = await Promise.all(
    ids.map(async (id) => {
      try {
        const res = await request<{ steps?: JobStep[] }>({ method: 'GET', url: apiPath('jobs', id) })
        return [id, res.steps ?? []] as const
      } catch {
        return [id, []] as const
      }
    }),
  )
  stepsByJob.value = Object.fromEntries(entries)
}

function stopPoll(): void {
  if (pollId) {
    window.clearInterval(pollId)
    pollId = 0
  }
}

function stopTick(): void {
  if (tickId) {
    window.clearInterval(tickId)
    tickId = 0
  }
}

function syncTick(): void {
  const live = liveJobs.value.length > 0
  if (live && !tickId) {
    nowMs.value = Date.now()
    tickId = window.setInterval(() => {
      nowMs.value = Date.now()
    }, 1000)
  }
  if (!live) stopTick()
}

function syncPoll(): void {
  const live = liveJobs.value.length > 0
  if (live && !pollId) {
    pollId = window.setInterval(() => {
      void load(true)
    }, POLL_MS)
  }
  if (!live) stopPoll()
  syncTick()
}

async function loadProjects(): Promise<void> {
  if (!ws.currentId) {
    projects.value = []
    return
  }
  projects.value =
    (await request<Project[]>({
      method: 'GET',
      url: apiPath('projects'),
      params: { workspace_rec_id: ws.currentId },
    })) ?? []
}

async function refreshOpenLog(): Promise<void> {
  const id = logJobId.value
  if (!logOpen.value || !id) return
  const job = jobs.value.find((item) => item.rec_id === id) ?? liveRows.value.find((item) => item.rec_id === id)
  if (!job || !LIVE.has(job.job_status)) return
  const view = await getJobLog(id, logSize.value).catch(() => null)
  if (!view) return
  const next = applyJobLog(logText.value, view)
  logText.value = next.text || logText.value
  logSize.value = next.size
}

async function load(silent = false): Promise<void> {
  if (!ws.currentId || !pageAlive) return
  const seq = ++loadSeq
  if (!silent) loading.value = true
  try {
    let out = await fetchPage(page.value, pageSize.value)
    if (!pageAlive || seq !== loadSeq) return
    const size = out.page_size || pageSize.value
    const last = Math.max(1, Math.ceil((out.total || 0) / size))
    if (out.total > 0 && page.value > last) {
      page.value = last
      out = await fetchPage(last, size)
      if (!pageAlive || seq !== loadSeq) return
    }
    applyList(out)
    if (!silent || !projects.value.length) {
      await loadProjects()
      if (!pageAlive || seq !== loadSeq) return
    }
    const liveIds = liveRows.value.filter((row) => LIVE.has(row.job_status)).map((row) => row.rec_id)
    await loadSteps(liveIds)
    await refreshOpenLog()
  } catch (e) {
    if (!silent && pageAlive && seq === loadSeq) toast.error(errText(e))
  } finally {
    if (!pageAlive || seq !== loadSeq) return
    if (!silent) loading.value = false
    syncPoll()
  }
}

function fetchPage(targetPage: number, size: number) {
  return listJobsPage({
    workspaceId: ws.currentId,
    projectId: projectFilter.value || undefined,
    page: targetPage,
    pageSize: size,
  })
}

function applyList(out: { items: Job[]; live: Job[]; stats: JobStats; total: number; page: number; page_size: number }): void {
  jobs.value = out.items
  liveRows.value = out.live
  stats.value = out.stats
  total.value = out.total
  if (out.page) page.value = out.page
  if (out.page_size) pageSize.value = out.page_size
}

function consumeSubmitQuery(): void {
  const projectId = projectFilter.value
  if (projectId) {
    form.value.project_rec_id = projectId
  }
  if (String(route.query.submit ?? '') !== '1') return
  open.value = true
  void router.replace({ name: 'jobs', query: projectId ? { project: projectId } : {} })
}

onMounted(() => {
  pageAlive = true
  void load()
  consumeSubmitQuery()
})

onUnmounted(() => {
  pageAlive = false
  stopTick()
  stopPoll()
})

watch(() => ws.currentId, () => {
  page.value = 1
  void load()
})
watch(projectFilter, () => {
  page.value = 1
  void load()
})
watch(() => [route.query.submit, route.query.project], consumeSubmitQuery)

function onPage(value: number): void {
  page.value = value
  void load()
}

function onPageSize(value: number): void {
  pageSize.value = value
  page.value = 1
  void load()
}
watch(logOpen, (shown) => {
  if (!shown) {
    logText.value = ''
    logSize.value = 0
    logJobId.value = ''
  }
})

async function submit(): Promise<void> {
  try {
    await request({
      method: 'POST',
      url: apiPath('jobs'),
      data: {
        workspace_rec_id: ws.currentId,
        project_rec_id: form.value.project_rec_id,
        job_type: form.value.job_type,
        payload: {
          tag: form.value.tag,
          notes: form.value.notes,
          command: form.value.command,
          pom_version: form.value.pom_version,
          update_pom: form.value.update_pom,
        },
      },
    })
    open.value = false
    page.value = 1
    await load()
  } catch (e) {
    toast.error(errText(e))
  }
}

async function showLog(id: string): Promise<void> {
  try {
    const view = await getJobLog(id, 0)
    const next = applyJobLog('', view)
    logJobId.value = id
    logText.value = next.text
    logSize.value = next.size
    logOpen.value = true
  } catch (e) {
    toast.error(errText(e))
  }
}

async function cancel(id: string): Promise<void> {
  busyId.value = id
  try {
    await request({ method: 'POST', url: apiPath('jobs', id, 'cancel') })
    await load()
  } catch (e) {
    toast.error(errText(e))
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <PageFrame class="jobs-page" wide :title="t('jobs.title')" :desc="t('jobs.desc')">
    <template #actions>
      <RsButton size="sm" variant="ghost" :loading="loading" @click="load()">{{ t('common.refresh') }}</RsButton>
      <RsButton size="sm" @click="open = true">{{ t('jobs.submit') }}</RsButton>
    </template>

    <div class="jobs">
      <section v-if="liveJobs.length" class="jobs-live">
        <header class="jobs-live__head">
          <h2>{{ t('jobs.liveTitle') }}</h2>
          <p>{{ t('jobs.liveDesc') }}</p>
        </header>
        <div class="live-grid">
          <article v-for="row in liveJobs" :key="row.rec_id" class="live-card" :class="{ 'is-running': row.job_status === 'running' }">
            <div class="live-card__top">
              <RsTag variant="primary" round>{{ typeLabel(row.job_type) }}</RsTag>
              <RsTag :variant="statusVariant(row.job_status)" round>{{ statusLabel(row.job_status) }}</RsTag>
            </div>
            <h3 class="live-card__name">{{ projectName(row.project_rec_id) }}</h3>
            <p v-if="payloadHint(row)" class="live-card__hint">{{ payloadHint(row) }}</p>
            <p class="live-card__clock">
              <span class="live-card__dot" aria-hidden="true" />
              <strong>{{ durationText(row) }}</strong>
            </p>
            <p v-if="currentStep(row.rec_id)" class="live-card__step">
              {{ t('jobs.currentStep') }}：{{ currentStep(row.rec_id)?.step_name }}
              <span v-if="stepProgress(row.rec_id)"> {{ stepProgress(row.rec_id) }}</span>
            </p>
            <p class="live-card__meta">{{ t('jobs.started') }} {{ formatTime(row.started_at || row.created_at) }}</p>
            <div class="live-card__actions">
              <RsButton size="sm" variant="text" tone="primary" @click="showLog(row.rec_id)">{{ t('jobs.log') }}</RsButton>
              <RsButton size="sm" variant="text" tone="danger" :loading="busyId === row.rec_id" @click="cancel(row.rec_id)">
                {{ t('common.cancel') }}
              </RsButton>
            </div>
          </article>
        </div>
      </section>

      <RsCard class="jobs__list" fill variant="outlined" elevated radius="lg" :title="t('jobs.listTitle')" :padding="false">
        <div class="jobs-stats">
          <span class="jobs-chip is-run">{{ t('jobs.statsRunning') }} <b>{{ stats.running }}</b></span>
          <span class="jobs-chip">{{ t('jobs.statsQueued') }} <b>{{ stats.queued }}</b></span>
          <span class="jobs-chip is-ok">{{ t('jobs.statsSucceeded') }} <b>{{ stats.succeeded }}</b></span>
          <span class="jobs-chip is-fail">{{ t('jobs.statsFailed') }} <b>{{ stats.failed }}</b></span>
        </div>
        <RsTable
          v-if="visibleJobs.length || loading"
          fill
          cell-focus="none"
          :columns="columns"
          :data="visibleJobs"
          :loading="loading"
          row-key="rec_id"
          :bordered="false"
          :cell-tooltip="true"
        >
          <template #job_type="{ row }">
            <RsTag variant="primary" round>{{ typeLabel(row.job_type) }}</RsTag>
          </template>
          <template #project="{ row }">
            <span class="jobs-proj">{{ projectName(row.project_rec_id) }}</span>
          </template>
          <template #job_status="{ row }">
            <RsTag :variant="statusVariant(row.job_status)" round>{{ statusLabel(row.job_status) }}</RsTag>
          </template>
          <template #duration="{ row }">
            <span class="jobs-time">{{ durationText(row) }}</span>
          </template>
          <template #started_at="{ row }">{{ formatTime(row.started_at || (row.job_status === 'queued' ? row.created_at : '')) }}</template>
          <template #error_text="{ row }">
            <span :title="row.error_text" class="jobs-error">{{ row.error_text || t('common.dash') }}</span>
          </template>
          <template #actions="{ row }">
            <RsButton size="sm" variant="text" tone="primary" @click="showLog(row.rec_id)">{{ t('jobs.log') }}</RsButton>
            <RsButton
              v-if="LIVE.has(row.job_status)"
              size="sm"
              variant="text"
              tone="danger"
              :loading="busyId === row.rec_id"
              @click="cancel(row.rec_id)"
            >
              {{ t('common.cancel') }}
            </RsButton>
          </template>
        </RsTable>
        <p v-else class="page__quiet">{{ t('jobs.empty') }}</p>
        <div v-if="total > 0" class="page__pager">
          <RsPagination
            :page="page"
            :page-size="pageSize"
            :total="total"
            show-page-size
            @update:page="onPage"
            @update:page-size="onPageSize"
          />
        </div>
      </RsCard>
    </div>

    <RsDialog v-model:open="open" layout="form" width="sm" :title="t('jobs.submit')">
      <template #body>
        <div class="form-grid">
          <RsSelect
            :model-value="form.job_type"
            :options="types"
            @update:model-value="(v: RsSelectModelValue) => (form.job_type = asString(v))"
          />
          <RsSelect
            :model-value="form.project_rec_id"
            :options="projects.map((p) => ({ value: p.rec_id, label: p.project_name }))"
            :placeholder="t('jobs.projectOptional')"
            @update:model-value="(v: RsSelectModelValue) => (form.project_rec_id = asString(v))"
          />
          <RsInput v-if="form.job_type === 'build'" v-model="form.command" :placeholder="t('jobs.commandOverride')" />
          <RsInput
            v-if="form.job_type === 'release' || form.job_type === 'pom_version'"
            v-model="form.tag"
            :placeholder="t('jobs.tag')"
          />
          <RsInput v-if="form.job_type === 'release'" v-model="form.notes" :placeholder="t('jobs.notes')" />
        </div>
      </template>
      <template #footer>
        <RsButton variant="ghost" @click="open = false">{{ t('common.cancel') }}</RsButton>
        <RsButton @click="submit">{{ t('common.submit') }}</RsButton>
      </template>
    </RsDialog>

    <RsDialog v-model:open="logOpen" class="jobs-log-dlg" layout="form" width="lg" :title="t('jobs.logTitle')">
      <template #body>
        <RsLog
          v-model:lines="logText"
          class="jobs-log"
          wrap
          height="min(32rem, 56vh)"
          :empty-text="t('common.empty')"
          :aria-label="t('jobs.logTitle')"
        />
      </template>
    </RsDialog>
  </PageFrame>
</template>

<style scoped>
.jobs-page {
  height: calc(100dvh - 8.5rem);
  max-height: calc(100dvh - 8.5rem);
  min-height: 0;
  overflow: hidden;
}

.jobs-page :deep(.page__stack) {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
}

.jobs {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  gap: 1rem;
}

.jobs-live {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.jobs-live__head h2 {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: var(--shell-title);
}

.jobs-live__head p {
  margin: 0.25rem 0 0;
  font-size: 0.8rem;
  line-height: 1.5;
  color: var(--shell-muted);
}

.jobs__list {
  flex: 1;
  min-height: 0;
}

.jobs__list :deep(.rs-card__body) {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.jobs__list :deep(.rs-table-shell--fill) {
  flex: 1 1 auto;
  min-height: 0;
}

.live-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 0.75rem;
}

.live-card {
  display: grid;
  gap: 0.35rem;
  padding: 0.85rem 0.95rem;
  border-radius: 12px;
  border: 1px solid var(--shell-card-border);
  background: var(--shell-inset-bg);
}

.live-card.is-running {
  border-color: rgba(14, 165, 233, 0.45);
  box-shadow: inset 0 0 0 1px rgba(14, 165, 233, 0.12);
}

.live-card__top {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.live-card__name {
  margin: 0.1rem 0 0;
  font-size: 0.98rem;
  font-weight: 700;
  color: var(--shell-title);
}

.live-card__hint,
.live-card__step,
.live-card__meta {
  margin: 0;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--shell-muted);
}

.live-card__clock {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  margin: 0.1rem 0;
  font-size: 0.9rem;
  color: var(--shell-title);
}

.live-card__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #0ea5e9;
  animation: jobs-pulse 1.4s ease-out infinite;
}

.live-card__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.2rem;
  margin-top: 0.15rem;
}

.jobs-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
  padding: 0.75rem 1rem 0.45rem;
}

.jobs-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.2rem 0.55rem;
  border-radius: 999px;
  background: var(--shell-inset-bg);
  color: var(--shell-muted);
  font-size: 0.75rem;
  font-weight: 650;
}

.jobs-chip b {
  font-variant-numeric: tabular-nums;
  color: var(--shell-title);
}

.jobs-chip.is-run {
  color: #0284c7;
}

.jobs-chip.is-ok {
  color: #15803d;
}

.jobs-chip.is-fail {
  color: #b91c1c;
}

.jobs-proj {
  font-weight: 650;
}

.jobs-time {
  font-variant-numeric: tabular-nums;
}

.jobs-error {
  display: inline-block;
  max-width: 22em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jobs-log-dlg :deep(.rs-dialog__body) {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.jobs-log {
  flex: 1 1 auto;
  min-height: 16rem;
  width: 100%;
}

@keyframes jobs-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(14, 165, 233, 0.5);
  }
  100% {
    box-shadow: 0 0 0 8px rgba(14, 165, 233, 0);
  }
}
</style>
