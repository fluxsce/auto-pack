<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  RsAlert,
  RsButton,
  RsDialog,
  RsFormItem,
  RsInput,
  RsSelect,
  RsSwitch,
  RsTable,
  RsTextarea,
  useRsToast,
  type RsSelectModelValue,
  type RsTableColumn,
} from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{
  recId: string
  envName: string
}>()

const { t } = useI18n()
const toast = useRsToast()

type MqConsumeMsg = {
  message_id: string
  body: string
  receipt_handle?: string
  deleted?: boolean
}

const mqBusy = ref(false)
const mqQueue = ref<'request' | 'result'>('request')
const mqDeleteAfter = ref(false)
const sampleBody = JSON.stringify(
  {
    taskId: 'TSKIDS-V900P09-001',
    taskNo: 'V900P09-001',
    packTaskId: 'PACK-V900P09-001',
    subsystemType: 'DATAHUB',
    packagingMode: 'war',
    deploymentStrategy: 'war',
    ossProjectRoot: '/开发/V9.0.0-P09/',
    customerCode: 'LSYM',
    customerShortName: '零食有鸣',
    resourceFiles: 'app.war,config.zip',
    mailTo: 'shangjian',
    version: 'V9.0.0',
    customerBranch: 'release/V9.0.0-P09',
    baseRef: 'V9.0.0-P09',
    baseRefType: 'tag',
    SYSVAL: 'V10DATAHUB',
  },
  null,
  2,
)
const mqBody = ref(sampleBody)
const mqProduceResult = ref<{ ok: boolean; message: string; message_id?: string; queue?: string; queue_name?: string } | null>(null)
const mqConsumeResult = ref<{
  ok: boolean
  message: string
  queue?: string
  queue_name?: string
  empty?: boolean
  messages?: MqConsumeMsg[]
} | null>(null)

const title = computed(() => {
  const name = props.envName.trim()
  return name ? `${t('envs.sectionMqTest')} · ${name}` : t('envs.sectionMqTest')
})

const mqQueueOptions = computed(() => [
  { value: 'request', label: t('envs.mqQueueRequest') },
  { value: 'result', label: t('envs.mqQueueResult') },
])

watch(
  () => [open.value, props.recId] as const,
  ([shown], prev) => {
    if (!shown) return
    if (prev && prev[1] === props.recId) return
    mqProduceResult.value = null
    mqConsumeResult.value = null
    mqFilterText.value = ''
    mqFilterDirection.value = ''
    mqFilterQueue.value = ''
    mqFilterSource.value = ''
    mqQueue.value = 'request'
    mqDeleteAfter.value = false
    mqBody.value = sampleBody
  },
)

function asString(value: RsSelectModelValue): string {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw == null || raw === '') return ''
  return String(typeof raw === 'object' ? raw.value : raw)
}

function mqPayload() {
  return {
    rec_id: props.recId,
    env_kind: 'mq',
    queue: mqQueue.value,
  }
}

async function mqProduce(): Promise<void> {
  const body = mqBody.value.trim()
  if (!body) {
    toast.error(t('envs.mqBodyRequired'))
    return
  }
  mqBusy.value = true
  mqProduceResult.value = null
  try {
    const res = await request<{ ok: boolean; message: string; message_id?: string; queue?: string; queue_name?: string }>({
      method: 'POST',
      url: apiPath('envs', 'mq-produce'),
      data: { ...mqPayload(), body },
    })
    mqProduceResult.value = res
    if (res.ok) {
      toast.success(t('envs.mqProduceOk'))
      await loadHistory()
    }
    else toast.error(res.message || t('envs.mqProduceFail'))
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    mqBusy.value = false
  }
}

async function mqConsume(): Promise<void> {
  mqBusy.value = true
  mqConsumeResult.value = null
  try {
    const res = await request<{
      ok: boolean
      message: string
      queue?: string
      queue_name?: string
      empty?: boolean
      messages?: MqConsumeMsg[]
    }>({
      method: 'POST',
      url: apiPath('envs', 'mq-consume'),
      data: { ...mqPayload(), delete: mqDeleteAfter.value },
    })
    mqConsumeResult.value = res
    if (res.ok) {
      toast.success(res.empty ? t('envs.mqConsumeEmpty') : t('envs.mqConsumeOk'))
    } else {
      toast.error(res.message || t('envs.mqConsumeFail'))
    }
    if ((res.messages?.length ?? 0) > 0) await loadHistory()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    mqBusy.value = false
  }
}

function formatMqBody(raw: string): string {
  const text = raw.trim()
  if (!text) return ''
  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return raw
  }
}

type MqConsumeRow = {
  key: string
  message_id: string
  taskId: string
  branch: string
  result: string
  body: string
}

function mqText(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function mqConsumeRows(messages: MqConsumeMsg[] | undefined): MqConsumeRow[] {
  return (messages ?? []).map((item, index) => {
    let taskId = ''
    let branch = ''
    let result = ''
    const raw = item.body || ''
    try {
      const parsed = JSON.parse(raw) as Record<string, unknown>
      taskId = mqText(parsed.taskId)
      branch = mqText(parsed.customerBranch)
      result = mqText(parsed.resultStatus)
    } catch {
      taskId = ''
    }
    return {
      key: item.message_id || `mq-${index}`,
      message_id: item.message_id,
      taskId,
      branch,
      result,
      body: formatMqBody(raw),
    }
  })
}

const mqConsumeTable = computed(() => mqConsumeRows(mqConsumeResult.value?.messages))

type MqLogRow = {
  rec_id: string
  direction: string
  queue_role: string
  queue_name?: string
  message_id?: string
  task_id?: string
  body?: string
  source: string
  deleted: number
  created_at: string
}

const mqFilterText = ref('')
const mqFilterDirection = ref('')
const mqFilterQueue = ref('')
const mqFilterSource = ref('')

const directionOptions = computed(() => [
  { value: '', label: t('envs.mqFilterAll') },
  { value: 'produce', label: t('envs.mqDirProduce') },
  { value: 'consume', label: t('envs.mqDirConsume') },
  { value: 'peek', label: t('envs.mqDirPeek') },
])

const queueFilterOptions = computed(() => [
  { value: '', label: t('envs.mqFilterAll') },
  { value: 'request', label: t('envs.mqQueueRequest') },
  { value: 'result', label: t('envs.mqQueueResult') },
])

const sourceOptions = computed(() => [
  { value: '', label: t('envs.mqFilterAll') },
  { value: 'manual', label: t('envs.mqSourceManual') },
  { value: 'intake', label: t('envs.mqSourceIntake') },
])

const filteredHistory = computed(() => {
  const keyword = mqFilterText.value.trim().toLowerCase()
  return mqHistory.value.filter((row) => {
    if (mqFilterDirection.value && row.direction !== mqFilterDirection.value) return false
    if (mqFilterQueue.value && row.queue_role !== mqFilterQueue.value) return false
    if (mqFilterSource.value && row.source !== mqFilterSource.value) return false
    if (!keyword) return true
    const blob = [row.task_id, row.message_id, row.queue_name, row.body, row.created_at].join('\n').toLowerCase()
    return blob.includes(keyword)
  })
})

function dash(value: unknown): string {
  const text = String(value ?? '').trim()
  return text || t('common.dash')
}

function oneLine(value: unknown): string {
  const text = String(value ?? '').replace(/\s+/g, ' ').trim()
  return text || t('common.dash')
}

function tableHeight(count: number): number {
  return Math.min(360, 42 + Math.max(count, 1) * 41)
}

const consumeColumns = computed<RsTableColumn<MqConsumeRow>[]>(() => [
  { key: 'taskId', title: 'taskId', minWidth: 160, ellipsis: true, formatter: (value) => dash(value) },
  { key: 'branch', title: t('envs.mqColBranch'), minWidth: 160, ellipsis: true, formatter: (value) => dash(value) },
  { key: 'result', title: t('envs.mqColResult'), width: 110, ellipsis: true, formatter: (value) => dash(value) },
  { key: 'message_id', title: t('envs.mqColMessage'), minWidth: 200, ellipsis: true, formatter: (value) => dash(value) },
  {
    key: 'body',
    title: t('envs.mqColBody'),
    minWidth: 280,
    ellipsis: true,
    formatter: (value) => oneLine(value),
    tooltip: (row) => row.body || undefined,
  },
])

const historyColumns = computed<RsTableColumn<MqLogRow>[]>(() => [
  { key: 'created_at', title: t('envs.mqColTime'), width: 188 },
  { key: 'direction', title: t('envs.mqColDirection'), width: 88, formatter: (value) => directionLabel(String(value ?? '')) },
  { key: 'queue_role', title: t('envs.mqQueue'), width: 110, formatter: (value) => queueLabel(String(value ?? '')) },
  { key: 'queue_name', title: t('envs.mqColQueueName'), minWidth: 180, ellipsis: true, formatter: (value) => dash(value) },
  { key: 'task_id', title: 'taskId', minWidth: 160, ellipsis: true, formatter: (value) => dash(value) },
  { key: 'source', title: t('envs.mqColSource'), width: 100, formatter: (value) => sourceLabel(String(value ?? '')) },
  { key: 'message_id', title: t('envs.mqColMessage'), minWidth: 200, ellipsis: true, formatter: (value) => dash(value) },
  {
    key: 'body',
    title: t('envs.mqColBody'),
    minWidth: 280,
    ellipsis: true,
    formatter: (value) => oneLine(value),
    tooltip: (row) => formatMqBody(row.body || '') || undefined,
  },
])

const mqHistory = ref<MqLogRow[]>([])
const mqHistoryError = ref('')

function directionLabel(direction: string): string {
  if (direction === 'produce') return t('envs.mqDirProduce')
  if (direction === 'consume') return t('envs.mqDirConsume')
  if (direction === 'peek') return t('envs.mqDirPeek')
  return direction
}

function sourceLabel(source: string): string {
  if (source === 'intake') return t('envs.mqSourceIntake')
  if (source === 'manual') return t('envs.mqSourceManual')
  return source
}

function queueLabel(role: string): string {
  return role === 'result' ? t('envs.mqQueueResult') : t('envs.mqQueueRequest')
}

async function loadHistory(): Promise<void> {
  if (!props.recId) {
    mqHistory.value = []
    return
  }
  mqHistoryError.value = ''
  try {
    mqHistory.value = (await request<MqLogRow[]>({
      method: 'GET',
      url: apiPath('envs', props.recId, 'mq-messages'),
    })) ?? []
  } catch (e) {
    mqHistoryError.value = e instanceof Error ? e.message : String(e)
  }
}

watch(open, (shown) => {
  if (shown) void loadHistory()
})
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="window"
    width="80%"
    height="75%"
    draggable
    resizable
    fullscreenable
    teleport-to="body"
    :title="title"
  >
    <template #body>
      <div class="mq-test">
        <p class="mq-test__desc">{{ t('envs.mqTestHint') }}</p>
        <RsFormItem :label="t('envs.mqQueue')">
          <RsSelect
            class="mq-test__full"
            :model-value="mqQueue"
            :options="mqQueueOptions"
            :disabled="mqBusy"
            @update:model-value="(v: RsSelectModelValue) => (mqQueue = asString(v) === 'result' ? 'result' : 'request')"
          />
        </RsFormItem>
        <RsFormItem :label="t('envs.mqBody')">
          <RsTextarea
            v-model="mqBody"
            :rows="8"
            :placeholder="t('envs.mqBodyPlaceholder')"
            :disabled="mqBusy"
          />
        </RsFormItem>
        <div class="mq-test__actions">
          <RsButton variant="ghost" :disabled="mqBusy" @click="mqProduce">{{ t('envs.mqProduce') }}</RsButton>
          <RsSwitch :model-value="mqDeleteAfter" :disabled="mqBusy" @update:model-value="mqDeleteAfter = !!$event">
            {{ t('envs.mqDeleteAfter') }}
          </RsSwitch>
          <RsButton variant="ghost" :disabled="mqBusy" @click="mqConsume">{{ t('envs.mqConsume') }}</RsButton>
        </div>

        <RsAlert
          v-if="mqProduceResult"
          :type="mqProduceResult.ok ? 'success' : 'danger'"
          :title="mqProduceResult.ok ? t('envs.mqProduceOk') : t('envs.mqProduceFail')"
          closable
          @close="mqProduceResult = null"
        >
          <dl class="mq-test__meta">
              <div v-if="mqProduceResult.queue_name">
                <dt>{{ t('envs.mqColQueueName') }}</dt>
                <dd>{{ mqProduceResult.queue_name }}</dd>
              </div>
              <div v-if="mqProduceResult.queue">
                <dt>{{ t('envs.mqQueue') }}</dt>
                <dd>{{ queueLabel(mqProduceResult.queue) }}</dd>
              </div>
            <div v-if="mqProduceResult.message_id">
              <dt>{{ t('envs.mqColMessage') }}</dt>
              <dd>{{ mqProduceResult.message_id }}</dd>
            </div>
              <div v-if="!mqProduceResult.ok">
                <dt>{{ t('envs.mqProduceFail') }}</dt>
                <dd>{{ mqProduceResult.message }}</dd>
              </div>
            </dl>
            <p v-if="mqProduceResult.ok" class="mq-test__cloud">{{ t('envs.mqProduceCloudHint') }}</p>
          </RsAlert>

        <RsAlert
          v-if="mqConsumeResult"
          :type="mqConsumeResult.ok ? 'success' : 'danger'"
          :title="mqConsumeResult.ok ? (mqConsumeResult.empty ? t('envs.mqConsumeEmpty') : t('envs.mqConsumeOk')) : t('envs.mqConsumeFail')"
          closable
          @close="mqConsumeResult = null"
        >
          {{ mqConsumeResult.message }}
          <span v-if="mqConsumeResult.queue_name"> · {{ mqConsumeResult.queue_name }}</span>
        </RsAlert>

        <RsTable
          v-if="mqConsumeTable.length"
          virtual
          :height="tableHeight(mqConsumeTable.length)"
          :columns="consumeColumns"
          :data="mqConsumeTable"
          row-key="key"
          :bordered="false"
          cell-focus="none"
          :cell-tooltip="true"
        />

        <section class="mq-test__history">
          <h3>{{ t('envs.mqLogTitle') }}</h3>
          <p>{{ t('envs.mqLogHint') }}</p>
          <div class="mq-test__filters">
            <RsInput v-model="mqFilterText" clearable :placeholder="t('envs.mqFilterPlaceholder')" />
            <RsSelect
              :model-value="mqFilterDirection"
              :options="directionOptions"
              @update:model-value="(v: RsSelectModelValue) => (mqFilterDirection = asString(v))"
            />
            <RsSelect
              :model-value="mqFilterQueue"
              :options="queueFilterOptions"
              @update:model-value="(v: RsSelectModelValue) => (mqFilterQueue = asString(v))"
            />
            <RsSelect
              :model-value="mqFilterSource"
              :options="sourceOptions"
              @update:model-value="(v: RsSelectModelValue) => (mqFilterSource = asString(v))"
            />
          </div>
          <p v-if="mqHistoryError" class="mq-test__error">{{ mqHistoryError }}</p>
          <p v-else-if="!mqHistory.length" class="mq-test__empty">{{ t('envs.mqLogEmpty') }}</p>
          <p v-else-if="!filteredHistory.length" class="mq-test__empty">{{ t('envs.mqFilterNoMatch') }}</p>
          <RsTable
            v-else
            virtual
            :height="tableHeight(filteredHistory.length)"
            :columns="historyColumns"
            :data="filteredHistory"
            row-key="rec_id"
            :bordered="false"
            cell-focus="none"
            :cell-tooltip="true"
          />
        </section>
      </div>
    </template>
  </RsDialog>
</template>

<style scoped>
.mq-test {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  min-width: 0;
}

.mq-test__desc {
  margin: 0;
  font-size: 0.8rem;
  line-height: 1.55;
  color: var(--shell-muted);
}

.mq-test__full {
  width: 100%;
}

.mq-test__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem 1rem;
}

.mq-test__meta {
  display: grid;
  gap: 0.35rem;
  margin: 0;
}

.mq-test__meta div {
  display: grid;
  grid-template-columns: 5.5rem minmax(0, 1fr);
  gap: 0.5rem;
}

.mq-test__meta dt {
  margin: 0;
  color: var(--shell-muted);
}

.mq-test__meta dd {
  margin: 0;
  word-break: break-all;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.mq-test__filters {
  display: grid;
  grid-template-columns: minmax(12rem, 1.6fr) repeat(3, minmax(7rem, 1fr));
  gap: 0.5rem;
  margin-bottom: 0.65rem;
}

@media (max-width: 860px) {
  .mq-test__filters {
    grid-template-columns: 1fr;
  }
}

.mq-test__cloud {
  margin: 0.45rem 0 0;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--shell-muted);
}

.mq-test__history h3 {
  margin: 0.4rem 0 0.35rem;
  font-size: 0.88rem;
  font-weight: 650;
}

.mq-test__history p,
.mq-test__empty,
.mq-test__error {
  margin: 0 0 0.55rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--shell-muted);
}

.mq-test__error {
  color: var(--rs-danger, #dc2626);
}
</style>
