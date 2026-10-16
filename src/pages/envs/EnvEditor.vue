<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  RsAlert,
  RsButton,
  RsCard,
  RsForm,
  RsFormItem,
  RsInput,
  RsSelect,
  RsSwitch,
  RsTextarea,
  useRsToast,
  type RsSelectModelValue,
} from '@/ui'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'
import {
  canonicalKind,
  defaultsOf,
  fieldVisible,
  findKind,
  kindTagClass,
  type EnvKindSpec,
} from './kinds'
import './env-panel.css'

const props = defineProps<{
  kinds: EnvKindSpec[]
  recId: string
  envKind: string
  envName: string
  isEnabled: number
  config: Record<string, unknown>
  secretsSet: string[]
  saving: boolean
  probing: boolean
  kindLocked: boolean
  probeResult: { ok: boolean; message: string } | null
}>()

const emit = defineEmits<{
  'update:envKind': [value: string]
  'update:envName': [value: string]
  'update:isEnabled': [value: number]
  'update:config': [value: Record<string, unknown>]
  save: []
  cancel: []
  probe: [testTo: string]
  'clear-probe': []
}>()

const { t } = useI18n()
const toast = useRsToast()
const testTo = ref(props.envKind === 'smtp' ? String(props.config.to ?? '') : '')
const isMq = computed(() => canonicalKind(props.envKind) === 'mq')

type MqConsumeMsg = {
  message_id: string
  body: string
  receipt_handle?: string
  deleted?: boolean
}

const mqBusy = ref(false)
/** 往返自测默认结果队列：请求队列会被本服务 intake 抢走。 */
const mqQueue = ref<'request' | 'result'>('result')
const mqDeleteAfter = ref(false)
const mqBody = ref(
  JSON.stringify(
    {
      taskId: 'TSKIDS-TEST-001',
      taskNo: 'TEST-001',
      packTaskId: 'PACK-TEST-001',
      subsystemType: 'DATAHUB',
      packagingMode: 'war',
      deploymentStrategy: 'war',
      ossProjectRoot: '/开发/S01/',
      customerCode: 'DEMO',
      version: 'V9.0.0',
      customerBranch: 'release/V9.0.0_DEMO',
      baseRef: 'V9.0.0_P01',
      baseRefType: 'tag',
      SYSVAL: 'V10DATAHUB',
    },
    null,
    2,
  ),
)
const mqProduceResult = ref<{ ok: boolean; message: string; message_id?: string; queue?: string } | null>(null)
const mqConsumeResult = ref<{
  ok: boolean
  message: string
  queue?: string
  empty?: boolean
  messages?: MqConsumeMsg[]
} | null>(null)

const mqQueueOptions = computed(() => [
  { value: 'request', label: t('envs.mqQueueRequest') },
  { value: 'result', label: t('envs.mqQueueResult') },
])

watch(
  () => props.envKind,
  (kind) => {
    if (canonicalKind(kind) !== 'mq') {
      mqProduceResult.value = null
      mqConsumeResult.value = null
    }
  },
)

function mqPayload() {
  return {
    rec_id: props.recId,
    env_kind: props.envKind,
    config: props.config,
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
    const res = await request<{ ok: boolean; message: string; message_id?: string; queue?: string }>({
      method: 'POST',
      url: apiPath('envs', 'mq-produce'),
      data: { ...mqPayload(), body },
    })
    mqProduceResult.value = res
    if (res.ok) toast.success(t('envs.mqProduceOk'))
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
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  } finally {
    mqBusy.value = false
  }
}

function formatMqProduce(res: { ok: boolean; message: string; message_id?: string; queue?: string }): string {
  const lines = [res.message]
  if (res.queue) lines.push(`queue: ${res.queue}`)
  if (res.message_id) lines.push(`message_id: ${res.message_id}`)
  return lines.filter(Boolean).join('\n')
}

function formatMqConsume(res: {
  ok: boolean
  message: string
  queue?: string
  empty?: boolean
  messages?: MqConsumeMsg[]
}): string {
  const lines = [res.message]
  if (res.queue) lines.push(`queue: ${res.queue}`)
  if (res.empty) return lines.join('\n')
  const msgs = res.messages ?? []
  if (!msgs.length) return lines.join('\n')
  lines.push(`count: ${msgs.length}`)
  for (let i = 0; i < msgs.length; i++) {
    const m = msgs[i]
    lines.push('')
    lines.push(`--- #${i + 1} ---`)
    if (m.message_id) lines.push(`message_id: ${m.message_id}`)
    if (m.deleted != null) lines.push(`deleted: ${m.deleted}`)
    lines.push('body:')
    lines.push(m.body || '')
  }
  return lines.join('\n')
}

const spec = computed(() => findKind(props.kinds, props.envKind))

const kindOptions = computed(() =>
  props.kinds.map((item) => ({
    value: item.kind,
    label: kindLabel(item.kind),
  })),
)

const visibleFields = computed(() => (spec.value?.fields ?? []).filter((field) => fieldVisible(field, props.config)))

const categoryLabel = computed(() => {
  const key = spec.value?.category ? `envs.categories.${spec.value.category}` : ''
  if (!key) return ''
  const label = t(key)
  return label === key ? spec.value?.category ?? '' : label
})

function kindLabel(code: string): string {
  const key = `envs.kinds.${code}`
  const label = t(key)
  return label === key ? code : label
}

function fieldLabel(key: string): string {
  const i18nKey = `envs.fields.${key}`
  const label = t(i18nKey)
  return label === i18nKey ? key : label
}

function fieldPlaceholder(key: string, secret: boolean | undefined): string {
  if (secret && props.secretsSet.includes(key)) return t('envs.secretKeep')
  const kind = canonicalKind(props.envKind)
  const byKind = `envs.placeholders.${kind}.${key}`
  const kindLabelText = t(byKind)
  if (kindLabelText !== byKind) return kindLabelText
  const i18nKey = `envs.placeholders.${key}`
  const label = t(i18nKey)
  if (label !== i18nKey) return label
  return fieldLabel(key)
}

const configHint = computed(() => {
  if (canonicalKind(props.envKind) === 'mq') return t('envs.mqHint')
  return t('envs.editorHint')
})

function groupLabel(group: string): string {
  const key = `envs.groups.${group}`
  const label = t(key)
  return label === key ? group : label
}

function optionLabel(key: string, value: string): string {
  const i18nKey = `envs.options.${key}.${value}`
  const label = t(i18nKey)
  return label === i18nKey ? value : label
}

function asString(value: RsSelectModelValue): string {
  const raw = Array.isArray(value) ? value[0] : value
  if (raw == null || raw === '') return ''
  return String(typeof raw === 'object' ? raw.value : raw)
}

function textValue(key: string): string {
  const v = props.config[key]
  if (v == null) return ''
  return String(v)
}

function boolValue(key: string): boolean {
  const v = props.config[key]
  return v !== false && v !== 0 && v !== 'false' && v !== '0'
}

function setConfig(key: string, value: unknown): void {
  emit('update:config', { ...props.config, [key]: value })
}

function numberOrEmpty(raw: string | number): number | '' {
  if (raw === '' || raw == null) return ''
  const n = typeof raw === 'number' ? raw : Number(raw)
  return Number.isFinite(n) ? n : ''
}

function onKind(value: RsSelectModelValue): void {
  const next = asString(value)
  const found = findKind(props.kinds, next)
  emit('update:envKind', next)
  if (found) emit('update:config', defaultsOf(found))
  testTo.value = next === 'smtp' ? String(props.config.to ?? '') : ''
}

function onProbe(): void {
  emit('probe', props.envKind === 'smtp' ? testTo.value.trim() : '')
}

function showGroup(index: number): string {
  const field = visibleFields.value[index]
  if (!field?.group) return ''
  const prev = visibleFields.value[index - 1]
  if (prev?.group === field.group) return ''
  return field.group
}
</script>

<template>
  <RsCard variant="outlined" elevated radius="lg" :padding="false" class="env-editor">
    <template #header>
      <div class="env-editor__head">
        <h2 class="env-editor__title">{{ recId ? t('envs.editorEdit') : t('envs.editorCreate') }}</h2>
        <span :class="kindTagClass(envKind, kinds)">{{ kindLabel(envKind) }}</span>
      </div>
    </template>

    <RsAlert
      v-if="probeResult"
      class="env-editor__probe"
      :type="probeResult.ok ? 'success' : 'danger'"
      :title="probeResult.ok ? t('envs.verifyOk') : t('envs.verifyFail')"
      closable
      @close="emit('clear-probe')"
    >
      {{ probeResult.message }}
    </RsAlert>

    <RsForm
      class="env-editor__form"
      label-position="top"
      gap="lg"
      @submit.prevent="emit('save')"
    >
      <section class="env-editor__section">
        <h3 class="env-editor__section-title">{{ t('envs.sectionBasic') }}</h3>
        <div class="env-editor__grid">
        <RsFormItem :label="t('envs.name')" required>
          <RsInput
            :model-value="envName"
            :placeholder="t('envs.name')"
            @update:model-value="emit('update:envName', $event)"
          />
        </RsFormItem>
        <RsFormItem :label="t('envs.kind')" required>
          <RsSelect
            class="env-editor__full"
            :model-value="envKind"
            :options="kindOptions"
            :disabled="kindLocked"
            :placeholder="t('envs.kindPlaceholder')"
            @update:model-value="onKind"
          />
        </RsFormItem>
        </div>
      </section>

      <section class="env-editor__section">
        <h3 class="env-editor__section-title">{{ t('envs.sectionConfig') }}</h3>
        <div class="env-editor__config">
          <div class="env-editor__config-head">
            <span>{{ categoryLabel }}</span>
            <p>{{ configHint }}</p>
          </div>
          <div class="env-editor__grid">
          <template v-for="(field, index) in visibleFields" :key="field.key">
            <h4 v-if="showGroup(index)" class="env-editor__group">{{ groupLabel(showGroup(index)) }}</h4>
            <RsFormItem v-if="field.type === 'select'" :label="fieldLabel(field.key)">
              <RsSelect
                class="env-editor__full"
                :model-value="textValue(field.key)"
                :options="(field.options ?? []).map((opt) => ({ value: opt, label: optionLabel(field.key, opt) }))"
                @update:model-value="(v: RsSelectModelValue) => setConfig(field.key, asString(v))"
              />
            </RsFormItem>
            <RsFormItem v-else-if="field.type === 'bool'" :label="fieldLabel(field.key)">
              <RsSwitch :model-value="boolValue(field.key)" @update:model-value="setConfig(field.key, $event)" />
            </RsFormItem>
            <RsFormItem v-else :label="fieldLabel(field.key)" :required="field.required">
              <RsInput
                :model-value="textValue(field.key)"
                :type="field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'"
                :placeholder="fieldPlaceholder(field.key, field.secret || field.type === 'password')"
                @update:model-value="setConfig(field.key, field.type === 'number' ? numberOrEmpty($event) : $event)"
              />
            </RsFormItem>
          </template>
          </div>
        </div>
      </section>

      <section v-if="isMq" class="env-editor__section">
        <h3 class="env-editor__section-title">{{ t('envs.sectionMqTest') }}</h3>
        <p class="env-editor__mq-desc">{{ t('envs.mqTestHint') }}</p>
        <div class="env-editor__mq">
          <RsFormItem :label="t('envs.mqQueue')">
            <RsSelect
              class="env-editor__full"
              :model-value="mqQueue"
              :options="mqQueueOptions"
              :disabled="saving || probing || mqBusy"
              @update:model-value="(v: RsSelectModelValue) => (mqQueue = asString(v) === 'result' ? 'result' : 'request')"
            />
          </RsFormItem>

          <RsFormItem :label="t('envs.mqBody')">
            <RsTextarea
              v-model="mqBody"
              :rows="10"
              :placeholder="t('envs.mqBodyPlaceholder')"
              :disabled="saving || probing || mqBusy"
            />
          </RsFormItem>

          <div class="env-editor__mq-actions">
            <RsButton
              variant="ghost"
              :disabled="saving || probing || mqBusy"
              @click="mqProduce"
            >
              {{ t('envs.mqProduce') }}
            </RsButton>
            <RsSwitch
              :model-value="mqDeleteAfter"
              :disabled="saving || probing || mqBusy"
              @update:model-value="mqDeleteAfter = !!$event"
            >
              {{ t('envs.mqDeleteAfter') }}
            </RsSwitch>
            <RsButton
              variant="ghost"
              :disabled="saving || probing || mqBusy"
              @click="mqConsume"
            >
              {{ t('envs.mqConsume') }}
            </RsButton>
          </div>

          <RsAlert
            v-if="mqProduceResult"
            class="env-editor__mq-result"
            :type="mqProduceResult.ok ? 'success' : 'danger'"
            :title="mqProduceResult.ok ? t('envs.mqProduceOk') : t('envs.mqProduceFail')"
            closable
            @close="mqProduceResult = null"
          >
            <pre class="env-editor__mq-pre">{{ formatMqProduce(mqProduceResult) }}</pre>
          </RsAlert>

          <RsAlert
            v-if="mqConsumeResult"
            class="env-editor__mq-result"
            :type="mqConsumeResult.ok ? 'success' : 'danger'"
            :title="mqConsumeResult.ok ? (mqConsumeResult.empty ? t('envs.mqConsumeEmpty') : t('envs.mqConsumeOk')) : t('envs.mqConsumeFail')"
            closable
            @close="mqConsumeResult = null"
          >
            <pre class="env-editor__mq-pre">{{ formatMqConsume(mqConsumeResult) }}</pre>
          </RsAlert>
        </div>
      </section>

      <section class="env-editor__section">
        <h3 class="env-editor__section-title">{{ t('envs.sectionMeta') }}</h3>
        <RsFormItem :label="t('envs.enabled')">
          <RsSwitch
            :model-value="isEnabled"
            :checked-value="1"
            :unchecked-value="0"
            @update:model-value="emit('update:isEnabled', Number($event))"
          />
        </RsFormItem>
      </section>
    </RsForm>

    <template #footer>
      <div class="env-editor__footer">
        <div class="env-editor__probe-actions">
          <RsInput
            v-if="envKind === 'smtp'"
            v-model="testTo"
            class="env-editor__test-to"
            :placeholder="t('envs.testRecipient')"
            :title="t('envs.testRecipientHint')"
            :disabled="saving || probing"
          />
          <RsButton variant="ghost" :disabled="saving || probing || mqBusy" @click="onProbe">
            {{ t('envs.testConnection') }}
          </RsButton>
        </div>
        <div class="env-editor__footer-main">
          <RsButton variant="ghost" :disabled="saving || probing || mqBusy" @click="emit('cancel')">{{ t('common.cancel') }}</RsButton>
          <RsButton :disabled="saving || probing || mqBusy" @click="emit('save')">{{ t('common.save') }}</RsButton>
        </div>
      </div>
    </template>
  </RsCard>
</template>

<style scoped>
.env-editor {
  overflow: hidden;
}

.env-editor__head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.65rem 0.75rem;
}

.env-editor__title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 650;
  line-height: 1.4;
  color: var(--shell-title);
}

.env-editor__probe {
  margin: 1rem 1.25rem 0;
}

.env-editor__probe :deep(.rs-alert__content) {
  white-space: pre-wrap;
  word-break: break-word;
}

.env-editor__form {
  padding: 0.5rem 1.25rem 0.35rem;
}

.env-editor__section {
  padding: 1.15rem 0;
}

.env-editor__section + .env-editor__section {
  border-top: 1px solid var(--shell-border);
}

.env-editor__section-title {
  margin: 0 0 0.95rem;
  font-size: 0.82rem;
  font-weight: 650;
  letter-spacing: 0.02em;
  color: var(--shell-muted);
}

.env-editor__grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1.05rem 1.25rem;
}

.env-editor__config {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  padding: 1.15rem 1.15rem 1.05rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.75rem;
  background: var(--shell-inset-bg);
}

.env-editor__config-head {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-bottom: 0.85rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--shell-border);
}

.env-editor__config-head span {
  font-size: 0.88rem;
  font-weight: 650;
  color: var(--shell-title);
}

.env-editor__config-head p {
  margin: 0;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--shell-muted);
}

.env-editor__group {
  grid-column: 1 / -1;
  margin: 0.15rem 0 0;
  font-size: 0.78rem;
  font-weight: 650;
  color: var(--shell-muted);
}

.env-editor__full {
  width: 100%;
}

.env-editor__mq-desc {
  margin: -0.35rem 0 0.85rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--shell-muted);
}

.env-editor__mq {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.15rem 1.15rem 1.05rem;
  border: 1px solid var(--shell-border);
  border-radius: 0.75rem;
  background: var(--shell-inset-bg);
}

.env-editor__mq-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem 1rem;
}

.env-editor__mq-result {
  margin: 0;
}

.env-editor__mq-pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.78rem;
  line-height: 1.45;
}

.env-editor__footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  width: 100%;
}

.env-editor__probe-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.env-editor__test-to {
  width: 16rem;
  max-width: 100%;
}

.env-editor__footer-main {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.6rem;
  margin-left: auto;
}

.env-editor :deep(.rs-form-item) {
  width: 100%;
  margin: 0;
}

.env-editor :deep(.rs-form-item__control),
.env-editor :deep(.rs-form-item__control > *),
.env-editor :deep(.rs-select),
.env-editor :deep(.rs-input) {
  width: 100%;
  max-width: 100%;
}

.env-editor :deep(.rs-field__label) {
  justify-content: flex-start;
  margin-bottom: 0.4rem;
  font-weight: 550;
  color: var(--shell-muted);
}

@media (min-width: 720px) {
  .env-editor__grid {
    grid-template-columns: 1fr 1fr;
  }
}

.env-editor :deep(.rs-card__footer) {
  border-top: 1px solid var(--shell-border);
  background: var(--shell-inset-bg);
}
</style>
