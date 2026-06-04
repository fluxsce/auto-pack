<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EnvSetting } from '@/types'
import { APP_DIALOG_POPPER_CLASS } from '@/components/feedback/dialog/constants'
import { POST_BUILD_WHEN_OPTIONS, type PostBuildAction, type PostBuildWhen } from '@/types/post-build'
import { useMessage } from '@/components/feedback/message'
import { batchUpdatePostBuildEmail } from '@/request/api/project'
import { defaultPostBuildAction } from '../post-build/post-build-defaults'
import PostBuildEmailNotifyFields from '../post-build/fields/PostBuildEmailNotifyFields.vue'
import { useSmtpEnvOptions } from '../post-build/use-smtp-env-options'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  workspaceRecId: string
  workspaceName: string
  projectCount: number
  envOptions: (envType: string) => EnvSetting[]
}>()

const emit = defineEmits<{
  success: []
  reloadEnvs: []
}>()

const { t } = useI18n()
const message = useMessage()

const action = ref<PostBuildAction>(defaultPostBuildAction('email_notify'))
const saving = ref(false)

const { load: loadSmtpEnvs } = useSmtpEnvOptions(props.envOptions)

watch(open, (v) => {
  if (v) {
    action.value = defaultPostBuildAction('email_notify')
    emit('reloadEnvs')
    void loadSmtpEnvs()
  }
})

onMounted(() => {
  void loadSmtpEnvs()
})

function whenLabel(when: PostBuildWhen): string {
  return t(`projectPostBuildWhen_${when}`)
}

async function onSubmit(): Promise<void> {
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  if (action.value.enabled && !action.value.recipients?.trim()) {
    message.warning(t('projectPostBuildRecipientsRequired'))
    return
  }
  saving.value = true
  try {
    const res = await batchUpdatePostBuildEmail({
      workspace_rec_id: props.workspaceRecId,
      enabled: action.value.enabled,
      when: action.value.when,
      env_rec_id: action.value.env_rec_id?.trim() ?? '',
      recipients: action.value.recipients?.trim() ?? '',
      subject: action.value.subject?.trim() ?? '',
      body: action.value.body?.trim() ?? '',
      name: action.value.name?.trim() ?? '',
    })
    message.success(t('projectBulkPostBuildEmailSuccess', { count: res.updated }))
    open.value = false
    emit('success')
  } catch (e) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AppDialog
    v-model="open"
    :title="$t('projectBulkPostBuildEmailTitle')"
    width="560px"
    initial-height="auto"
    :show-footer="true"
    :confirm-loading="saving"
    @confirm="onSubmit"
  >
    <p class="project-bulk-email__hint">
      {{ $t('projectBulkPostBuildEmailHint', { name: workspaceName, count: projectCount }) }}
    </p>
    <el-form label-width="100px" class="project-bulk-email__form" @submit.prevent="onSubmit">
      <el-form-item :label="$t('enabled')">
        <el-switch v-model="action.enabled" />
      </el-form-item>
      <el-form-item :label="$t('projectPostBuildWhen')">
        <el-select
          v-model="action.when"
          teleported
          :popper-class="APP_DIALOG_POPPER_CLASS"
          class="project-bulk-email__full"
        >
          <el-option
            v-for="w in POST_BUILD_WHEN_OPTIONS"
            :key="w"
            :label="whenLabel(w)"
            :value="w"
          />
        </el-select>
      </el-form-item>
      <PostBuildEmailNotifyFields
        v-model="action"
        :env-options="envOptions"
        @reload-envs="emit('reloadEnvs')"
      />
    </el-form>
    <p class="project-bulk-email__note">{{ $t('projectBulkPostBuildEmailNote') }}</p>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-bulk-email {
  &__hint {
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__note {
    margin: 12px 0 0;
    font-size: 12px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
  }

  &__full {
    width: 100%;
  }

  &__form :deep(.post-build-fields__full) {
    width: 100%;
  }
}
</style>
