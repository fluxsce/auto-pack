<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import { batchUpdateProjectVersion } from '@/request/api/project'

const open = defineModel<boolean>('open', { required: true })

const props = defineProps<{
  workspaceRecId: string
  workspaceName: string
  projectCount: number
}>()

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()
const message = useMessage()

const version = ref('1.0.0')
const saving = ref(false)

watch(open, (v) => {
  if (v) version.value = '1.0.0'
})

async function onSubmit(): Promise<void> {
  const value = version.value.trim()
  if (!value) {
    message.warning(t('projectVersionRequired'))
    return
  }
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  saving.value = true
  try {
    const res = await batchUpdateProjectVersion({
      workspace_rec_id: props.workspaceRecId,
      project_version: value,
    })
    message.success(
      t('projectBulkUpdateVersionSuccess', { count: res.updated, version: value }),
    )
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
    :title="$t('projectBulkUpdateVersionTitle')"
    width="480px"
    initial-height="auto"
    :show-footer="true"
    :confirm-loading="saving"
    @confirm="onSubmit"
  >
    <p class="project-bulk-version__hint">
      {{ $t('projectBulkUpdateVersionHint', { name: workspaceName, count: projectCount }) }}
    </p>
    <el-form label-width="88px" @submit.prevent="onSubmit">
      <el-form-item :label="$t('projectColVersion')" required>
        <el-input
          v-model="version"
          :placeholder="$t('projectVersionPlaceholder')"
          autocomplete="off"
        />
      </el-form-item>
      <p class="project-bulk-version__note">{{ $t('projectBulkUpdateVersionNote') }}</p>
    </el-form>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-bulk-version {
  &__hint {
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.55;
    color: var(--shell-content-text-secondary, var(--el-text-color-secondary));
  }

  &__note {
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
    color: var(--shell-content-text-secondary, var(--el-text-color-placeholder));
  }
}
</style>
