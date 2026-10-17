<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsInput, RsSelect, RsTextarea, useRsToast } from '@/ui'
import { errText, getWorkspaceEnv, sendEnvMail } from '../../api'
import {
  buildFtpNotifyEmailBody,
  buildFtpNotifyEmailHTML,
  buildFtpNotifyEmailSubject,
  FTP_NOTIFY_FROM_NAME,
  FTP_NOTIFY_SCOPE_OPTIONS,
  FTP_NOTIFY_UPDATE_METHOD,
  normalizeNotifyRecipients,
  notifyTemplateMeta,
  smtpFromNameFromConfig,
  toUpdateMailFieldsPayload,
  type FtpNotifyEmailFields,
  type NotifyMailTemplate,
} from '../../lib/ftp-notify-email'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{
  initial: FtpNotifyEmailFields | null
  smtpOptions: { value: string; label: string }[]
  /**
   * 仅人为改动通知传 project_update（走服务端模板）。
   * 推送后「发送告知」不传，保持原来的客户端拼装 + 原文发送。
   */
  template?: NotifyMailTemplate
}>()

const { t } = useI18n()
const toast = useRsToast()
const smtpEnvRecId = ref('')
const fromName = ref(FTP_NOTIFY_FROM_NAME)
const fromNameManual = ref(false)
const recipients = ref('')
const subject = ref('')
const saving = ref(false)
const fields = ref<FtpNotifyEmailFields>(emptyFields())

const isProjectUpdate = computed(() => props.template === 'project_update')
const mailTemplate = computed<NotifyMailTemplate>(() => props.template || 'ftp_update')
const defaultFromName = computed(() =>
  isProjectUpdate.value ? notifyTemplateMeta('project_update').fromName : FTP_NOTIFY_FROM_NAME,
)
const dialogTitle = computed(() =>
  isProjectUpdate.value ? t('workspaces.remote.notify.projectTitle') : t('workspaces.remote.notify.title'),
)

const yesNo = [
  { value: '是', label: t('workspaces.remote.notify.yes') },
  { value: '否', label: t('workspaces.remote.notify.no') },
]
const yesSkip = [
  { value: '是', label: t('workspaces.remote.notify.yes') },
  { value: '不', label: t('workspaces.remote.notify.skip') },
]
const scopeOptions = FTP_NOTIFY_SCOPE_OPTIONS.map((value) => ({ value, label: value }))
const htmlBody = computed(() => buildFtpNotifyEmailHTML(fields.value, new Date(), mailTemplate.value))
const textBody = computed(() => buildFtpNotifyEmailBody(fields.value, new Date(), mailTemplate.value))
const canSubmit = computed(() => Boolean(smtpEnvRecId.value && recipients.value.trim() && subject.value.trim() && !saving.value))

function emptyFields(): FtpNotifyEmailFields {
  return {
    projectLabel: '',
    updateContent: '',
    taskNo: '',
    ftpVersionSource: '',
    updateMethod: FTP_NOTIFY_UPDATE_METHOD,
    needRestart: '是',
    browserCacheRefresh: '是',
    impactScope: '全局',
    updateScope: 'DATAHUB界面',
    affectsApi: '是',
    warPackage: '',
    other: '',
    backup: '',
  }
}

function applyInitial(): void {
  const init = props.initial
  fromNameManual.value = false
  fields.value = {
    ...emptyFields(),
    ...init,
    updateMethod: FTP_NOTIFY_UPDATE_METHOD,
    needRestart: init?.needRestart || '是',
    browserCacheRefresh: init?.browserCacheRefresh || '是',
    impactScope: init?.impactScope || '全局',
    updateScope: init?.updateScope || 'DATAHUB界面',
    affectsApi: init?.affectsApi || '是',
  }
  smtpEnvRecId.value = props.smtpOptions[0]?.value || ''
  fromName.value = defaultFromName.value
  recipients.value = ''
  subject.value = buildFtpNotifyEmailSubject(fields.value, mailTemplate.value)
  if (smtpEnvRecId.value) void loadSmtpDefaults(smtpEnvRecId.value)
}

async function loadSmtpDefaults(id: string): Promise<void> {
  try {
    const row = await getWorkspaceEnv(id)
    if (!fromNameManual.value) {
      const name = smtpFromNameFromConfig(row.config)
      fromName.value = !name || name === 'Auto Pack' ? defaultFromName.value : name
    }
    const to = String(row.config?.to ?? '').trim()
    if (to && !recipients.value.trim()) recipients.value = to
  } catch {
    /* keep defaults */
  }
}

watch(open, (visible) => {
  if (visible) applyInitial()
})

watch(
  fields,
  () => {
    subject.value = buildFtpNotifyEmailSubject(fields.value, mailTemplate.value)
  },
  { deep: true },
)

watch(smtpEnvRecId, (id) => {
  if (id && open.value) void loadSmtpDefaults(id)
})

function onRecipientsBlur(): void {
  if (!recipients.value.trim()) return
  recipients.value = normalizeNotifyRecipients(recipients.value)
}

async function submit(): Promise<void> {
  if (!canSubmit.value) return
  recipients.value = normalizeNotifyRecipients(recipients.value)
  if (!recipients.value.trim()) {
    toast.warning(t('workspaces.remote.notify.recipientsRequired'))
    return
  }
  saving.value = true
  try {
    // 推送告知：仍只发 subject/body/html（原逻辑）。
    // 人为改动：带 template=project_update，由服务端按模板渲染。
    if (isProjectUpdate.value) {
      await sendEnvMail(smtpEnvRecId.value, {
        recipients: recipients.value.trim(),
        subject: subject.value.trim(),
        from_name: fromName.value.trim() || defaultFromName.value,
        template: 'project_update',
        fields: toUpdateMailFieldsPayload(fields.value),
      })
    } else {
      await sendEnvMail(smtpEnvRecId.value, {
        recipients: recipients.value.trim(),
        subject: subject.value.trim(),
        body: textBody.value.trim(),
        html: htmlBody.value.trim(),
        from_name: fromName.value.trim() || FTP_NOTIFY_FROM_NAME,
      })
    }
    toast.success(t('workspaces.remote.notify.sendOk'))
    open.value = false
  } catch (e) {
    toast.error(errText(e) || t('workspaces.remote.notify.sendFail'))
  } finally {
    saving.value = false
  }
}
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
    :title="dialogTitle"
  >
    <template #body>
      <div class="notify-split">
        <div class="form-col">
          <section class="sec">
            <header class="sec-head">
              <h3>{{ t('workspaces.remote.notify.sectionSend') }}</h3>
            </header>
            <div class="pair">
              <RsFormItem :label="t('workspaces.remote.notify.smtp')" required>
                <RsSelect
                  v-model="smtpEnvRecId"
                  :options="smtpOptions"
                  :placeholder="t('workspaces.remote.notify.smtpPh')"
                />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.remote.notify.fromName')">
                <RsInput
                  v-model="fromName"
                  :placeholder="
                    isProjectUpdate
                      ? t('workspaces.remote.notify.projectFromNamePh')
                      : t('workspaces.remote.notify.fromNamePh')
                  "
                  @update:model-value="fromNameManual = true"
                />
              </RsFormItem>
            </div>
            <p v-if="!smtpOptions.length" class="hint">{{ t('workspaces.remote.notify.smtpEmpty') }}</p>
            <RsFormItem
              :label="t('workspaces.remote.notify.recipients')"
              :tooltip="t('workspaces.remote.notify.recipientsHint')"
              required
            >
              <RsInput
                v-model="recipients"
                :placeholder="t('workspaces.remote.notify.recipientsPh')"
                @blur="onRecipientsBlur"
              />
            </RsFormItem>
          </section>
          <section class="sec">
            <header class="sec-head">
              <h3>{{ t('workspaces.remote.notify.sectionFields') }}</h3>
            </header>
            <div class="grid">
              <RsFormItem :label="t('workspaces.remote.notify.projectLabel')">
                <RsInput v-model="fields.projectLabel" :placeholder="t('workspaces.remote.notify.projectLabelPh')" />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.remote.notify.taskNo')">
                <RsInput v-model="fields.taskNo" />
              </RsFormItem>
              <RsFormItem class="span2" :label="t('workspaces.remote.notify.updateContent')">
                <RsInput
                  v-model="fields.updateContent"
                  :placeholder="
                    isProjectUpdate
                      ? t('workspaces.remote.notify.projectUpdateContentPh')
                      : t('workspaces.remote.notify.updateContentPh')
                  "
                />
              </RsFormItem>
              <RsFormItem class="span2" :label="t('workspaces.remote.notify.warPackage')">
                <RsInput v-model="fields.warPackage" />
              </RsFormItem>
              <RsFormItem class="span2" :label="t('workspaces.remote.notify.ftpSource')">
                <RsInput v-model="fields.ftpVersionSource" />
              </RsFormItem>
              <RsFormItem class="span2" :label="t('workspaces.remote.notify.backup')">
                <RsTextarea v-model="fields.backup" :rows="2" />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.remote.notify.updateMethod')">
                <RsInput v-model="fields.updateMethod" readonly />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.remote.notify.updateScope')">
                <RsSelect v-model="fields.updateScope" :options="scopeOptions" />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.remote.notify.affectsApi')">
                <RsSelect v-model="fields.affectsApi" :options="yesNo" />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.remote.notify.needRestart')">
                <RsSelect v-model="fields.needRestart" :options="yesSkip" />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.remote.notify.browserCache')">
                <RsSelect v-model="fields.browserCacheRefresh" :options="yesSkip" />
              </RsFormItem>
              <RsFormItem :label="t('workspaces.remote.notify.impactScope')">
                <RsInput v-model="fields.impactScope" />
              </RsFormItem>
              <RsFormItem class="span2" :label="t('workspaces.remote.notify.other')">
                <RsTextarea v-model="fields.other" :rows="2" />
              </RsFormItem>
            </div>
          </section>
        </div>
        <div class="preview-col">
          <RsFormItem :label="t('workspaces.mailSubject')" required>
            <RsInput v-model="subject" />
          </RsFormItem>
          <RsFormItem class="preview-item" :label="t('workspaces.remote.notify.preview')" required>
            <div class="preview-wrap">
              <iframe v-if="open" class="preview" :title="t('workspaces.remote.notify.preview')" :srcdoc="htmlBody" sandbox />
            </div>
          </RsFormItem>
          <p class="hint">
            {{
              isProjectUpdate
                ? t('workspaces.remote.notify.projectPreviewHint')
                : t('workspaces.remote.notify.previewHint')
            }}
          </p>
        </div>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" :disabled="saving" @click="open = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="saving" :disabled="!canSubmit" @click="submit">{{ t('workspaces.remote.notify.send') }}</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.notify-split {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(22rem, 1fr) minmax(22rem, 1fr);
  gap: 1rem;
}

.form-col,
.preview-col {
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

.sec {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
}

.sec-head h3 {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--shell-title);
}

.pair,
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.7rem;
}

.span2 {
  grid-column: 1 / -1;
}

.hint {
  margin: 0;
  font-size: 0.78rem;
  line-height: 1.45;
  color: var(--shell-muted);
}

.preview-item {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.preview-item :deep(.rs-form-item__control),
.preview-wrap {
  flex: 1;
  min-height: 0;
}

.preview-wrap {
  width: 100%;
  overflow: hidden;
  border: 1px solid var(--shell-border);
  border-radius: 0.55rem;
  background: #eef2f7;
}

.preview {
  display: block;
  width: 100%;
  height: 100%;
  min-height: 16rem;
  border: 0;
  background: #eef2f7;
}

@media (max-width: 900px) {
  .notify-split,
  .pair,
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
