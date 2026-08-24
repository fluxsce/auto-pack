<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  APP_DIALOG_NESTED_Z_INDEX,
  APP_DIALOG_POPPER_CLASS,
} from '@/components/feedback/dialog/constants'
import { useMessage } from '@/components/feedback/message'
import { sendEnvMail } from '@/request/api/env'
import {
  buildFtpNotifyEmailBody,
  buildFtpNotifyEmailHTML,
  buildFtpNotifyEmailSubject,
  FTP_NOTIFY_FROM_NAME,
  FTP_NOTIFY_SCOPE_OPTIONS,
  FTP_NOTIFY_UPDATE_METHOD,
  normalizeNotifyRecipients,
  smtpFromNameFromConfig,
  type FtpNotifyEmailFields,
} from '@/utils/ftp-notify-email'
import { useSmtpEnvOptions } from '../post-build/use-smtp-env-options'
import type { EnvSetting } from '@/types'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  initial: FtpNotifyEmailFields | null
  envOptions?: (envType: string) => EnvSetting[]
}>()

const emit = defineEmits<{
  sent: []
}>()

const { t } = useI18n()
const message = useMessage()

const smtpEnvRecId = ref('')
const fromName = ref(FTP_NOTIFY_FROM_NAME)
const fromNameManual = ref(false)
const recipients = ref('')
const subject = ref('')
const saving = ref(false)

const fields = ref<FtpNotifyEmailFields>({
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
})

const { loading, options, hasOptions, load, optionLabel } = useSmtpEnvOptions(props.envOptions)

const textBody = computed(() => buildFtpNotifyEmailBody(fields.value))
const htmlBody = computed(() => buildFtpNotifyEmailHTML(fields.value))

watch(open, (v) => {
  if (!v) return
  resetFromInitial()
  void load()
})

watch(
  fields,
  () => {
    subject.value = buildFtpNotifyEmailSubject(fields.value)
  },
  { deep: true },
)

watch(smtpEnvRecId, (recId) => {
  if (fromNameManual.value) return
  const env = options.value.find((e) => e.rec_id === recId)
  const name = smtpFromNameFromConfig(env?.config_json)
  fromName.value = !name || name === 'Auto Pack' ? FTP_NOTIFY_FROM_NAME : name
})

function resetFromInitial(): void {
  const init = props.initial
  fromNameManual.value = false
  fields.value = {
    projectLabel: init?.projectLabel ?? '',
    updateContent: init?.updateContent ?? '',
    taskNo: init?.taskNo ?? '',
    ftpVersionSource: init?.ftpVersionSource ?? '',
    updateMethod: FTP_NOTIFY_UPDATE_METHOD,
    needRestart: init?.needRestart || '是',
    browserCacheRefresh: init?.browserCacheRefresh || '是',
    impactScope: init?.impactScope || '全局',
    updateScope: init?.updateScope || 'DATAHUB界面',
    affectsApi: init?.affectsApi || '是',
    warPackage: init?.warPackage ?? '',
    other: init?.other ?? '',
    backup: init?.backup ?? '',
  }
  smtpEnvRecId.value = ''
  fromName.value = FTP_NOTIFY_FROM_NAME
  recipients.value = ''
  subject.value = buildFtpNotifyEmailSubject(fields.value)
}

function onEnvSelectVisible(visible: boolean): void {
  if (visible) void load()
}

function onFromNameInput(): void {
  fromNameManual.value = true
}

function onRecipientsBlur(): void {
  if (!recipients.value.trim()) return
  recipients.value = normalizeNotifyRecipients(recipients.value)
}

async function onSubmit(): Promise<void> {
  if (!smtpEnvRecId.value) {
    message.warning(t('projectFtpNotifySmtpRequired'))
    return
  }
  recipients.value = normalizeNotifyRecipients(recipients.value)
  if (!recipients.value.trim()) {
    message.warning(t('projectFtpNotifyRecipientsRequired'))
    return
  }
  if (!subject.value.trim() || !htmlBody.value.trim()) {
    message.warning(t('projectFtpNotifyBodyRequired'))
    return
  }
  saving.value = true
  try {
    await sendEnvMail(smtpEnvRecId.value, {
      recipients: recipients.value.trim(),
      subject: subject.value.trim(),
      body: textBody.value.trim(),
      html: htmlBody.value.trim(),
      from_name: fromName.value.trim() || FTP_NOTIFY_FROM_NAME,
    })
    message.success(t('projectFtpNotifySendSuccess'))
    open.value = false
    emit('sent')
  } catch (e) {
    message.error((e as Error).message || t('projectFtpNotifySendFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectFtpNotifyTitle')"
    width="760px"
    :z-index="APP_DIALOG_NESTED_Z_INDEX"
    :show-footer="true"
    :confirm-text="$t('projectFtpNotifySend')"
    :confirm-loading="saving"
    @confirm="onSubmit"
  >
    <el-form label-width="128px" class="ftp-notify" @submit.prevent="onSubmit">
      <el-form-item :label="$t('projectPostBuildSmtpEnv')" required>
        <el-select
          v-model="smtpEnvRecId"
          filterable
          teleported
          :loading="loading"
          :popper-class="APP_DIALOG_POPPER_CLASS"
          class="ftp-notify__full"
          :placeholder="$t('projectPostBuildSmtpEnvPlaceholder')"
          @visible-change="onEnvSelectVisible"
        >
          <el-option
            v-for="e in options"
            :key="e.rec_id"
            :label="optionLabel(e)"
            :value="e.rec_id"
            :disabled="e.is_enabled !== 1"
          />
        </el-select>
        <p v-if="!hasOptions && !loading" class="ftp-notify__hint">
          {{ $t('projectPostBuildSmtpEnvHint') }}
        </p>
      </el-form-item>
      <el-form-item :label="$t('projectFtpNotifyFromName')">
        <el-input
          v-model="fromName"
          :placeholder="$t('projectFtpNotifyFromNamePlaceholder')"
          @input="onFromNameInput"
        />
      </el-form-item>
      <el-form-item :label="$t('projectPostBuildRecipients')" required>
        <el-input
          v-model="recipients"
          :placeholder="$t('projectFtpNotifyRecipientsPlaceholder')"
          @blur="onRecipientsBlur"
        />
        <p class="ftp-notify__hint">{{ $t('projectFtpNotifyRecipientsHint') }}</p>
      </el-form-item>

      <div class="ftp-notify__grid">
        <el-form-item :label="$t('projectFtpNotifyProjectLabel')">
          <el-input v-model="fields.projectLabel" :placeholder="$t('projectFtpNotifyProjectLabelPlaceholder')" />
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyTaskNo')">
          <el-input v-model="fields.taskNo" />
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyUpdateContent')" class="ftp-notify__span2">
          <el-input v-model="fields.updateContent" :placeholder="$t('projectFtpNotifyUpdateContentPlaceholder')" />
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyWarPackage')" class="ftp-notify__span2">
          <el-input v-model="fields.warPackage" />
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyFtpSource')" class="ftp-notify__span2">
          <el-input v-model="fields.ftpVersionSource" />
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyBackup')" class="ftp-notify__span2">
          <el-input v-model="fields.backup" />
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyUpdateMethod')">
          <el-input :model-value="FTP_NOTIFY_UPDATE_METHOD" disabled />
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyUpdateScope')">
          <el-select
            v-model="fields.updateScope"
            teleported
            :popper-class="APP_DIALOG_POPPER_CLASS"
            class="ftp-notify__full"
          >
            <el-option v-for="opt in FTP_NOTIFY_SCOPE_OPTIONS" :key="opt" :label="opt" :value="opt" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyAffectsApi')">
          <el-select
            v-model="fields.affectsApi"
            teleported
            :popper-class="APP_DIALOG_POPPER_CLASS"
            class="ftp-notify__full"
          >
            <el-option label="是" value="是" />
            <el-option label="否" value="否" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyNeedRestart')">
          <el-select
            v-model="fields.needRestart"
            teleported
            :popper-class="APP_DIALOG_POPPER_CLASS"
            class="ftp-notify__full"
          >
            <el-option label="是" value="是" />
            <el-option label="不" value="不" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyBrowserCache')">
          <el-select
            v-model="fields.browserCacheRefresh"
            teleported
            :popper-class="APP_DIALOG_POPPER_CLASS"
            class="ftp-notify__full"
          >
            <el-option label="是" value="是" />
            <el-option label="不" value="不" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('projectFtpNotifyImpactScope')">
          <el-input v-model="fields.impactScope" />
        </el-form-item>
      </div>

      <el-form-item :label="$t('projectPostBuildSubject')" required>
        <el-input v-model="subject" />
      </el-form-item>
      <el-form-item :label="$t('projectFtpNotifyPreview')" required>
        <div class="ftp-notify__preview-wrap">
          <iframe
            v-if="open"
            class="ftp-notify__preview"
            title="email-preview"
            :srcdoc="htmlBody"
            sandbox=""
          />
        </div>
        <p class="ftp-notify__hint">{{ $t('projectFtpNotifyBodyHint') }}</p>
      </el-form-item>
    </el-form>
  </AppDialog>
</template>

<style scoped lang="scss">
.ftp-notify {
  &__full {
    width: 100%;
  }

  &__hint {
    margin: 4px 0 0;
    font-size: 12px;
    line-height: 1.45;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
  }

  &__grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0 8px;
  }

  &__span2 {
    grid-column: 1 / -1;
  }

  &__preview-wrap {
    width: 100%;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    overflow: hidden;
    background: #eef2f7;
  }

  &__preview {
    display: block;
    width: 100%;
    height: 320px;
    border: 0;
    background: #eef2f7;
  }
}
</style>
