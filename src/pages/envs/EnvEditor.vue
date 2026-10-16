<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  RsAlert,
  RsButton,
  RsCard,
  RsDialog,
  RsForm,
  RsFormItem,
  RsInput,
  RsSelect,
  RsSwitch,
  type RsSelectModelValue,
} from '@/ui'
import {
  buildFtpNotifyEmailHTML,
  buildFtpNotifyEmailSubject,
  FTP_NOTIFY_UPDATE_METHOD,
  type FtpNotifyEmailFields,
  type NotifyMailTemplate,
} from '@/pages/workspaces/lib/ftp-notify-email'
import {
  FALLBACK_KINDS,
  canonicalKind,
  defaultsOf,
  fieldVisible,
  findKind,
  kindTagClass,
  type EnvKindSpec,
  type EnvRow,
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
  envRows?: EnvRow[]
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
const testTo = ref(props.envKind === 'smtp' ? 'shangjian' : '')
const mailPreviewOpen = ref(false)

const mailTemplate = computed(() => String(props.config.mail_template ?? '').trim() || 'none')
const mailPreviewTemplate = computed<NotifyMailTemplate>(() => {
  if (mailTemplate.value === 'project_update') return 'project_update'
  if (mailTemplate.value === 'oss_update' || mailTemplate.value === 'mq_notify') return 'oss_update'
  return 'ftp_update'
})
const mailPreviewFields = computed<FtpNotifyEmailFields>(() => {
  if (mailPreviewTemplate.value === 'oss_update') {
    return {
      projectLabel: '零食有鸣',
      updateContent: '申请更新TSK-0001',
      taskNo: 'TSK-0001',
      ftpVersionSource: 'https://oss.example.com/开发/S01/app.war',
      sourceLabel: 'OSS访问路径',
      updateMethod: FTP_NOTIFY_UPDATE_METHOD,
      needRestart: '是',
      browserCacheRefresh: '是',
      impactScope: '全局',
      updateScope: 'release/customer',
      affectsApi: '是',
      warPackage: 'app.war、config.zip',
      other: '有效期至 2026-10-17 23:00:00',
      backup: '/开发/S01/BAK/20261017004900123/app.war',
    }
  }
  return {
    projectLabel: 'datahub',
    updateContent: '推送路径 /开发/S01/app.war，有效期至 2026-10-17 23:00:00',
    taskNo: 'TSK-0001',
    ftpVersionSource: '/开发/S01/app.war',
    updateMethod: FTP_NOTIFY_UPDATE_METHOD,
    needRestart: '是',
    browserCacheRefresh: '是',
    impactScope: '全局',
    updateScope: 'release/customer',
    affectsApi: '是',
    warPackage: 'app.war',
    other: 'https://oss.example.com/开发/S01/app.war',
    backup: '',
  }
})
const mailPreviewHtml = computed(() =>
  buildFtpNotifyEmailHTML(mailPreviewFields.value, new Date(), mailPreviewTemplate.value),
)
const mailPreviewSubject = computed(() =>
  buildFtpNotifyEmailSubject(mailPreviewFields.value, mailPreviewTemplate.value),
)

const spec = computed(() => {
  const found = findKind(props.kinds, props.envKind)
  const fallback = findKind(FALLBACK_KINDS, props.envKind)
  if (!found) return fallback
  if (!fallback) return found
  const byKey = new Map(found.fields.map((field) => [field.key, field]))
  const fields = fallback.fields.map((field) => {
    const current = byKey.get(field.key)
    if (!current || field.group === 'links') return field
    return current
  })
  for (const field of found.fields) {
    if (!fallback.fields.some((item) => item.key === field.key)) fields.push(field)
  }
  return { ...found, fields }
})

const kindOptions = computed(() =>
  props.kinds.map((item) => ({
    value: item.kind,
    label: kindLabel(item.kind),
  })),
)

const visibleFields = computed(() => (spec.value?.fields ?? []).filter((field) => fieldVisible(field, props.config)))
const configFields = computed(() => visibleFields.value.filter((field) => field.group !== 'links'))
const linkFields = computed(() => visibleFields.value.filter((field) => field.group === 'links'))

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
  testTo.value = next === 'smtp' ? 'shangjian' : ''
}

function onProbe(): void {
  emit('probe', props.envKind === 'smtp' ? testTo.value.trim() : '')
}

function isEnvPick(key: string): boolean {
  return key === 'oss_env_rec_id' || key === 'ftp_env_rec_id' || key === 'smtp_env_rec_id'
}

function envPickKinds(key: string): string[] {
  if (key === 'oss_env_rec_id') return ['oss']
  if (key === 'ftp_env_rec_id') return ['ftp', 'sftp', 'ssh']
  if (key === 'smtp_env_rec_id') return ['smtp']
  return []
}

function envPickOptions(key: string): { value: string; label: string }[] {
  const kinds = new Set(envPickKinds(key))
  const options = [{ value: '', label: t('envs.linkNone') }]
  for (const row of props.envRows ?? []) {
    if (!kinds.has(canonicalKind(row.env_kind)) || row.is_enabled === 0) continue
    const kind = kindLabel(canonicalKind(row.env_kind))
    options.push({ value: row.rec_id, label: `${row.env_name}（${kind}）` })
  }
  const current = textValue(key)
  if (current && !options.some((item) => item.value === current)) {
    options.push({ value: current, label: current })
  }
  return options
}

function showGroup(index: number): string {
  const field = configFields.value[index]
  if (!field?.group) return ''
  const prev = configFields.value[index - 1]
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
          <template v-for="(field, index) in configFields" :key="field.key">
            <h4 v-if="showGroup(index)" class="env-editor__group">{{ groupLabel(showGroup(index)) }}</h4>
            <RsFormItem v-if="field.type === 'select'" :label="fieldLabel(field.key)">
              <RsSelect
                class="env-editor__full"
                :model-value="textValue(field.key)"
                :options="(field.options ?? []).map((opt) => ({ value: opt, label: optionLabel(field.key, opt) }))"
                @update:model-value="(v: RsSelectModelValue) => setConfig(field.key, asString(v))"
              />
            </RsFormItem>
            <RsFormItem v-else-if="isEnvPick(field.key)" :label="fieldLabel(field.key)">
              <RsSelect
                class="env-editor__full"
                :model-value="textValue(field.key)"
                :options="envPickOptions(field.key)"
                :placeholder="t('envs.linkNone')"
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

      <section v-if="linkFields.length" class="env-editor__section">
        <h3 class="env-editor__section-title">{{ t('envs.groups.links') }}</h3>
        <p class="env-editor__link-hint">{{ t('envs.linkHint') }}</p>
        <div class="env-editor__grid">
          <template v-for="field in linkFields" :key="field.key">
            <RsFormItem v-if="isEnvPick(field.key)" :label="fieldLabel(field.key)">
              <RsSelect
                class="env-editor__full"
                :model-value="textValue(field.key)"
                :options="envPickOptions(field.key)"
                :placeholder="t('envs.linkNone')"
                @update:model-value="(v: RsSelectModelValue) => setConfig(field.key, asString(v))"
              />
            </RsFormItem>
            <RsFormItem v-else :label="fieldLabel(field.key)">
              <div class="env-editor__mail-row">
                <RsSelect
                  class="env-editor__full"
                  :model-value="textValue(field.key) || 'none'"
                  :options="(field.options ?? []).map((opt) => ({ value: opt, label: optionLabel(field.key, opt) }))"
                  @update:model-value="(v: RsSelectModelValue) => setConfig(field.key, asString(v))"
                />
                <RsButton
                  v-if="field.key === 'mail_template'"
                  variant="ghost"
                  :disabled="mailTemplate === 'none'"
                  @click="mailPreviewOpen = true"
                >
                  {{ t('envs.mailPreview') }}
                </RsButton>
              </div>
            </RsFormItem>
          </template>
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
          <RsButton variant="ghost" :disabled="saving || probing" @click="onProbe">
            {{ t('envs.testConnection') }}
          </RsButton>
        </div>
        <div class="env-editor__footer-main">
          <RsButton variant="ghost" :disabled="saving || probing" @click="emit('cancel')">{{ t('common.cancel') }}</RsButton>
          <RsButton :disabled="saving || probing" @click="emit('save')">{{ t('common.save') }}</RsButton>
        </div>
      </div>
    </template>
  </RsCard>

  <RsDialog
    v-model:open="mailPreviewOpen"
    class="env-mail-preview"
    layout="window"
    width="820px"
    height="86%"
    draggable
    resizable
    fullscreenable
    teleport-to="body"
    :title="t('envs.mailPreviewTitle')"
  >
    <template #body>
      <div class="env-editor__mail-preview">
        <p class="env-editor__mail-subject">{{ t('envs.mailPreviewSubject') }}：{{ mailPreviewSubject }}</p>
        <p class="env-editor__mail-hint">{{ t('envs.mailPreviewHint') }}</p>
        <iframe
          v-if="mailPreviewOpen"
          class="env-editor__mail-frame"
          :title="t('envs.mailPreviewTitle')"
          :srcdoc="mailPreviewHtml"
          sandbox=""
        />
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" @click="mailPreviewOpen = false">{{ t('common.close') }}</RsButton>
    </template>
  </RsDialog>
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

.env-editor__link-hint {
  margin: -0.35rem 0 0.95rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--shell-muted);
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

.env-editor__mail-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
}

.env-editor__mail-row :deep(.rs-select) {
  flex: 1;
  min-width: 0;
}

.env-editor__mail-preview {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.env-editor__mail-subject {
  margin: 0 0 0.35rem;
  font-size: 0.84rem;
  font-weight: 650;
  color: var(--shell-title);
}

.env-editor__mail-hint {
  margin: 0 0 0.75rem;
  font-size: 0.78rem;
  line-height: 1.5;
  color: var(--shell-muted);
}

.env-editor__mail-frame {
  display: block;
  flex: 1 1 auto;
  width: 100%;
  min-height: 0;
  border: 1px solid var(--shell-border);
  border-radius: 0.65rem;
  background: #eef2f7;
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

<style>
.env-mail-preview.rs-dialog__content .rs-dialog__body {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
</style>
