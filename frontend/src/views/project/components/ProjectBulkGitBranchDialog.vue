<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import { batchUpdateRepoBranch } from '@/request/api/project'

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

const branch = ref('main')
const saving = ref(false)

watch(open, (v) => {
  if (v) branch.value = 'main'
})

async function onSubmit(): Promise<void> {
  const value = branch.value.trim()
  if (!value) {
    message.warning(t('projectBranchRequired'))
    return
  }
  if (!props.workspaceRecId) {
    message.warning(t('workspaceSelectRequired'))
    return
  }
  saving.value = true
  try {
    const res = await batchUpdateRepoBranch({
      workspace_rec_id: props.workspaceRecId,
      repo_branch: value,
    })
    message.success(
      t('projectBulkUpdateGitBranchSuccess', { count: res.updated, branch: value }),
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
    :title="$t('projectBulkUpdateGitBranchTitle')"
    width="480px"
    initial-height="auto"
    :show-footer="true"
    :confirm-loading="saving"
    @confirm="onSubmit"
  >
    <p class="project-bulk-branch__hint">
      {{ $t('projectBulkUpdateGitBranchHint', { name: workspaceName, count: projectCount }) }}
    </p>
    <el-form label-width="88px" @submit.prevent="onSubmit">
      <el-form-item :label="$t('projectColBranch')" required>
        <el-input
          v-model="branch"
          :placeholder="$t('projectBranchPlaceholder')"
          autocomplete="off"
        />
      </el-form-item>
      <p class="project-bulk-branch__note">{{ $t('projectBranchHint') }}</p>
    </el-form>
  </AppDialog>
</template>

<style scoped lang="scss">
.project-bulk-branch {
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
