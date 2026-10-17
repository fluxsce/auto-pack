<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsDialog, RsInput, RsLog, RsSelect, RsTag, type RsSelectModelValue } from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import { formatStamp } from '../composables/use-project-list'
import { autoBuildHistory, autoBuildLog, autoBuildStatus, getJobLog } from '../api'

type AuditEntry = {
  rec_id: string
  summary: string
  success_flag: number
  created_at?: string
  target_id?: string
}

type AuditPage = {
  items?: AuditEntry[]
}

type MqItem = {
  rec_id: string
  title: string
  branch: string
  message: string
  jobId: string
  buildId: string
  ok: boolean
  time: string
}

const props = defineProps<{
  workspaceId: string
}>()

const { t } = useI18n()
const items = ref<MqItem[]>([])
const selectedId = ref('')
const detail = ref('')
const loading = ref(false)
const logLoading = ref(false)
const open = ref(false)
const latest = computed(() => items.value[0])
const selected = computed(() => items.value.find((item) => item.rec_id === selectedId.value))
const listQuery = ref('')
const listPhase = ref('')
const phaseOptions = computed(() => [
  { value: '', label: t('workspaces.mqLog.filterAll') },
  { value: 'run', label: t('workspaces.mqLog.running') },
  { value: 'ok', label: t('audit.ok') },
  { value: 'fail', label: t('audit.fail') },
])
const visibleItems = computed(() => {
  const keyword = listQuery.value.trim().toLowerCase()
  return items.value.filter((item) => {
    if (listPhase.value && phaseOf(item) !== listPhase.value) return false
    if (!keyword) return true
    return `${item.title}\n${item.branch}\n${item.message}`.toLowerCase().includes(keyword)
  })
})

function asString(value: RsSelectModelValue): string {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw == null) return ''
  return String(typeof raw === 'object' ? raw.value : raw)
}
let listSeq = 0
let logSeq = 0
let pollTimer: ReturnType<typeof setInterval> | null = null

function stopPoll(): void {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function parseSummary(summary: string): { taskId: string; branch: string; message: string; jobId: string; buildId: string } {
  let rest = summary.trim()
  let jobId = ''
  let buildId = ''
  const build = rest.match(/\sbuild=(\S+)\s*$/)
  if (build && build.index != null) {
    buildId = build[1]
    rest = rest.slice(0, build.index).trim()
  }
  const job = rest.match(/\sjob=(\S+)\s*$/)
  if (job && job.index != null) {
    jobId = job[1]
    rest = rest.slice(0, job.index).trim()
  }
  const head = '消息队列'
  if (rest.startsWith(head)) rest = rest.slice(head.length).trim()
  const colon = rest.indexOf('：')
  let title = rest
  let message = ''
  if (colon >= 0) {
    title = rest.slice(0, colon).trim()
    message = rest.slice(colon + 1).trim()
  }
  const parts = title.split(/\s+/).filter(Boolean)
  return {
    taskId: parts[0] || '',
    branch: parts.slice(1).join(' '),
    message: message || summary.trim(),
    jobId,
    buildId,
  }
}

function isProgressMessage(message: string): boolean {
  return message.startsWith('已接收') || message.includes('正在构建')
}

function phaseOf(item: MqItem): 'run' | 'ok' | 'fail' {
  if (isProgressMessage(item.message)) return 'run'
  return item.ok ? 'ok' : 'fail'
}

/** 同一任务已经有成功或失败记录时，不再把更早的「已接收 / 正在构建」显示为进行中。 */
function hideSettledProgress(rows: MqItem[]): MqItem[] {
  const settled = new Set<string>()
  for (const item of rows) {
    if (item.title && !isProgressMessage(item.message)) settled.add(item.title)
  }
  if (!settled.size) return rows
  return rows.filter((item) => !item.title || !settled.has(item.title) || !isProgressMessage(item.message))
}

function phaseVariant(item: MqItem): 'warning' | 'success' | 'danger' {
  const phase = phaseOf(item)
  if (phase === 'run') return 'warning'
  return phase === 'ok' ? 'success' : 'danger'
}

function phaseLabel(item: MqItem): string {
  const phase = phaseOf(item)
  if (phase === 'run') return t('workspaces.mqLog.running')
  return phase === 'ok' ? t('audit.ok') : t('audit.fail')
}

function toItem(row: AuditEntry): MqItem {
  const parsed = parseSummary(row.summary || '')
  return {
    rec_id: row.rec_id,
    title: parsed.taskId || row.target_id || t('workspaces.mqLog.title'),
    branch: parsed.branch,
    message: parsed.message,
    jobId: parsed.jobId,
    buildId: parsed.buildId,
    ok: row.success_flag === 1,
    time: formatStamp(row.created_at, t('common.dash')),
  }
}

function parseLocalStamp(value: string): number {
  const matched = value.match(/(\d{4})\/(\d{1,2})\/(\d{1,2})\s+(\d{1,2}):(\d{2}):(\d{2})/)
  if (!matched) return Number.NaN
  return new Date(
    Number(matched[1]),
    Number(matched[2]) - 1,
    Number(matched[3]),
    Number(matched[4]),
    Number(matched[5]),
    Number(matched[6]),
  ).getTime()
}

function parseServerStamp(value?: string): number {
  if (!value) return Number.NaN
  const iso = value.includes('T') ? value : value.replace(' ', 'T')
  const zoned = /(?:[zZ]|[+-]\d{2}:?\d{2})$/.test(iso)
  return new Date(zoned ? iso : `${iso}Z`).getTime()
}

async function resolveBuildId(item: MqItem): Promise<string> {
  if (item.buildId) return item.buildId
  if (item.jobId || phaseOf(item) !== 'run') return ''
  try {
    const status = await autoBuildStatus(props.workspaceId)
    if (status.status === 'running' && status.build_rec_id) {
      item.buildId = status.build_rec_id
      return status.build_rec_id
    }
  } catch {
    // 状态读不到时再按开始时间就近找这次构建。
  }
  try {
    const when = parseLocalStamp(item.time)
    const rows = await autoBuildHistory(props.workspaceId)
    let best = ''
    let gap = Number.POSITIVE_INFINITY
    for (const row of rows) {
      const started = parseServerStamp(row.started_at)
      if (Number.isNaN(when) || Number.isNaN(started)) continue
      const delta = Math.abs(started - when)
      if (delta < gap) {
        gap = delta
        best = row.rec_id
      }
    }
    if (best && gap <= 3 * 60 * 1000) {
      item.buildId = best
      return best
    }
  } catch {
    return ''
  }
  return ''
}

async function fillLog(item: MqItem, seq: number, first: boolean): Promise<void> {
  if (first) {
    logLoading.value = true
    detail.value = ''
  }
  try {
    const head = item.message ? `${item.message}\n\n` : ''
    const buildId = await resolveBuildId(item)
    if (seq !== logSeq) return
    if (buildId) {
      const view = await autoBuildLog(props.workspaceId, buildId)
      if (seq !== logSeq) return
      const log = view.content.trim()
      detail.value = (log.includes('—— 回执 ——') ? '' : head) + (log || t('workspaces.mqLog.noJobLog'))
      return
    }
    if (item.jobId) {
      const view = await getJobLog(item.jobId)
      if (seq !== logSeq) return
      detail.value = head + (view.log.trim() || t('workspaces.mqLog.noJobLog'))
      return
    }
    detail.value = item.message || t('workspaces.mqLog.noJobLog')
  } catch (e) {
    if (seq !== logSeq) return
    const msg = e instanceof Error ? e.message : String(e)
    detail.value = `${item.message}\n\n${t('workspaces.mqLog.logFail')}：${msg}`
  } finally {
    if (seq === logSeq) logLoading.value = false
  }
}

async function show(item: MqItem): Promise<void> {
  selectedId.value = item.rec_id
  const seq = ++logSeq
  stopPoll()
  await fillLog(item, seq, true)
  if (seq !== logSeq || phaseOf(item) !== 'run') return
  pollTimer = setInterval(() => {
    if (!open.value || selectedId.value !== item.rec_id) {
      stopPoll()
      return
    }
    void fillLog(item, seq, false)
  }, 2000)
}

async function reload(): Promise<void> {
  const workspaceId = props.workspaceId
  const seq = ++listSeq
  logSeq++
  if (!workspaceId) {
    items.value = []
    selectedId.value = ''
    detail.value = ''
    loading.value = false
    return
  }
  loading.value = true
  try {
    const out = await request<AuditPage>({
      method: 'GET',
      url: apiPath('audit'),
      params: {
        workspace_rec_id: workspaceId,
        action_code: 'intake.mq',
        page: 1,
        page_size: 30,
      },
    })
    if (seq !== listSeq) return
    items.value = hideSettledProgress((out.items ?? []).map(toItem))
    const current = items.value.find((item) => item.rec_id === selectedId.value) ?? items.value[0]
    if (!current) {
      selectedId.value = ''
      detail.value = ''
    } else if (open.value) {
      await show(current)
    } else {
      selectedId.value = current.rec_id
    }
  } catch (e) {
    if (seq !== listSeq) return
    items.value = []
    selectedId.value = ''
    detail.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === listSeq) loading.value = false
  }
}

watch(
  () => props.workspaceId,
  () => {
    stopPoll()
    listQuery.value = ''
    listPhase.value = ''
    void reload()
  },
  { immediate: true },
)

watch(open, (value) => {
  if (!value) stopPoll()
})

onBeforeUnmount(stopPoll)

function openDialog(): void {
  if (!items.value.length) return
  open.value = true
  const current = items.value.find((item) => item.rec_id === selectedId.value) ?? items.value[0]
  if (current) void show(current)
}

defineExpose({ reload })
</script>

<template>
  <div v-if="workspaceId" class="mq-banner-wrap">
    <button
      v-if="items.length"
      type="button"
      class="mq-banner"
      :aria-label="t('workspaces.mqLog.title')"
      @click="openDialog"
    >
      <span class="mq-banner__label">{{ t('workspaces.mqLog.title') }}</span>
      <span class="mq-banner__title">{{ latest?.title }}</span>
      <span v-if="latest?.branch" class="mq-banner__branch">{{ latest.branch }}</span>
      <RsTag v-if="latest" :variant="phaseVariant(latest)" round>
        {{ phaseLabel(latest) }}
      </RsTag>
      <span class="mq-banner__time">{{ latest?.time }}</span>
    </button>
    <p v-else class="mq-banner mq-banner--empty">
      {{ loading ? t('workspaces.mqLog.loading') : t('workspaces.mqLog.empty') }}
    </p>

    <RsDialog
      v-model:open="open"
      layout="window"
      width="86%"
      height="78%"
      draggable
      resizable
      fullscreenable
      teleport-to="body"
      :title="t('workspaces.mqLog.title')"
    >
      <template #body>
        <div class="mq-dialog">
          <aside class="mq-dialog__side">
            <div class="mq-filters">
              <RsInput
                v-model="listQuery"
                clearable
                :placeholder="t('workspaces.mqLog.filterPlaceholder')"
                :aria-label="t('workspaces.mqLog.filterPlaceholder')"
              />
              <RsSelect
                :model-value="listPhase"
                :options="phaseOptions"
                :aria-label="t('workspaces.mqLog.filterResult')"
                @update:model-value="(v: RsSelectModelValue) => (listPhase = asString(v))"
              />
            </div>
            <p v-if="items.length && !visibleItems.length" class="mq-filter-empty">
              {{ t('workspaces.mqLog.filterNoMatch') }}
            </p>
            <ul v-else class="mq__list">
              <li v-for="item in visibleItems" :key="item.rec_id">
                <button
                  type="button"
                  class="mq__item"
                  :class="{ 'is-on': item.rec_id === selectedId }"
                  @click="show(item)"
                >
                  <span class="mq__title-row">
                    <span class="mq__title">{{ item.title }}</span>
                    <RsTag :variant="phaseVariant(item)" round>
                      {{ phaseLabel(item) }}
                    </RsTag>
                  </span>
                  <span v-if="item.branch" class="mq__branch">{{ item.branch }}</span>
                  <span class="mq__time">{{ item.time }}</span>
                </button>
              </li>
            </ul>
            <section v-if="selected" class="mq-detail">
              <h4>{{ t('workspaces.mqLog.detail') }}</h4>
              <dl>
                <div>
                  <dt>{{ t('workspaces.mqLog.task') }}</dt>
                  <dd>{{ selected.title }}</dd>
                </div>
                <div>
                  <dt>{{ t('workspaces.mqLog.branch') }}</dt>
                  <dd>{{ selected.branch || t('common.dash') }}</dd>
                </div>
                <div>
                  <dt>{{ t('workspaces.mqLog.result') }}</dt>
                  <dd>{{ phaseLabel(selected) }}</dd>
                </div>
                <div>
                  <dt>{{ t('workspaces.mqLog.time') }}</dt>
                  <dd>{{ selected.time }}</dd>
                </div>
                <div>
                  <dt>{{ t('workspaces.mqLog.note') }}</dt>
                  <dd>{{ selected.message }}</dd>
                </div>
              </dl>
            </section>
          </aside>
          <RsLog
            class="mq-dialog__log"
            :lines="detail"
            wrap
            height="100%"
            :empty-text="logLoading ? t('workspaces.mqLog.loading') : t('workspaces.mqLog.pick')"
            :aria-label="t('workspaces.mqLog.title')"
          />
        </div>
      </template>
    </RsDialog>
  </div>
</template>

<style scoped>
.mq-banner-wrap {
  flex-shrink: 0;
  padding: 0.65rem 0.85rem 0;
}

.mq-banner {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  margin: 0;
  padding: 0.45rem 0.7rem;
  border: 1px solid color-mix(in srgb, #0284c7 28%, var(--shell-border));
  border-radius: 0.65rem;
  background: color-mix(in srgb, #0284c7 8%, transparent);
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.mq-banner--empty {
  cursor: default;
  border-color: var(--shell-border);
  background: var(--shell-inset-bg);
  font-size: 0.8rem;
  color: var(--shell-muted);
}

.mq-banner__label {
  flex-shrink: 0;
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--shell-title);
}

.mq-banner__title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.82rem;
  font-weight: 700;
}

.mq-banner__branch,
.mq-banner__time {
  flex-shrink: 0;
  font-size: 0.75rem;
  color: var(--shell-muted);
}

.mq-banner__branch {
  font-family: var(--rs-font-mono);
  color: #0369a1;
}

.mq-banner__time {
  margin-left: auto;
}

.mq-dialog {
  display: grid;
  grid-template-columns: minmax(16rem, 22rem) minmax(0, 1fr);
  gap: 0.75rem;
  flex: 1;
  height: 100%;
  min-height: 0;
}

.mq-dialog__side {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  min-width: 0;
  min-height: 0;
}

.mq-filters {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  flex-shrink: 0;
}

.mq-filter-empty {
  flex: 1;
  margin: 0;
  font-size: 0.75rem;
  line-height: 1.5;
  color: var(--shell-muted);
}

.mq-detail {
  flex-shrink: 0;
  padding: 0.7rem 0.75rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.65rem;
  background: var(--shell-inset-bg);
}

.mq-detail h4 {
  margin: 0 0 0.5rem;
  font-size: 0.78rem;
  font-weight: 700;
}

.mq-detail dl {
  display: grid;
  gap: 0.4rem;
  margin: 0;
}

.mq-detail div {
  display: grid;
  grid-template-columns: 3.2rem minmax(0, 1fr);
  gap: 0.45rem;
}

.mq-detail dt {
  margin: 0;
  font-size: 0.75rem;
  color: var(--shell-muted);
}

.mq-detail dd {
  margin: 0;
  font-size: 0.78rem;
  word-break: break-word;
  white-space: pre-line;
}

.mq__list {
  list-style: none;
  flex: 1;
  margin: 0;
  padding: 0;
  min-height: 0;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.mq__item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.15rem;
  width: 100%;
  margin: 0;
  padding: 0.45rem 0.55rem;
  border: 1px solid transparent;
  border-radius: 0.55rem;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.mq__item:hover {
  background: color-mix(in srgb, #0284c7 8%, transparent);
}

.mq__item.is-on {
  border-color: color-mix(in srgb, #0284c7 35%, transparent);
  background: color-mix(in srgb, #0284c7 12%, transparent);
}

.mq__title-row {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  width: 100%;
  min-width: 0;
}

.mq__title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.8rem;
  font-weight: 700;
}

.mq__branch,
.mq__time {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.72rem;
  color: var(--shell-muted);
}

.mq__branch {
  font-family: var(--rs-font-mono);
  color: #0369a1;
}

.mq-dialog__log {
  min-width: 0;
  min-height: 0;
  height: 100%;
}

.mq-dialog__log :deep(.rs-log) {
  height: 100%;
  min-height: 0;
}

@media (max-width: 860px) {
  .mq-dialog {
    grid-template-columns: 1fr;
    grid-template-rows: minmax(12rem, 16rem) minmax(12rem, 1fr);
  }
}
</style>
