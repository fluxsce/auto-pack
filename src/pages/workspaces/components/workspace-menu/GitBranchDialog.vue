<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsInput, useRsToast } from '@/ui'
import type { Workspace } from '@/stores/workspace'
import { errText, updateWorkspaceGitBranches } from '../../api'
import { useWorkspaceTargetCount } from '../../composables/use-workspace-target-count'
import { isGitBranch } from '../../lib/path-slug'
import { isGitProject } from '../../lib/workspace-targets'

type FieldHandle = { validate: () => Promise<boolean> }

const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{
  success: []
}>()

const props = defineProps<{
  workspace?: Workspace | null
}>()

const { t } = useI18n()
const toast = useRsToast()
const branch = ref('main')
const saving = ref(false)
const branchInput = ref<FieldHandle | null>(null)
const presets = ['main', 'develop', 'release']
const { loading, count: gitCount, load } = useWorkspaceTargetCount(isGitProject)

const canSubmit = computed(() => gitCount.value > 0 && isGitBranch(branch.value) && !saving.value)

watch(open, (visible) => {
  if (!visible) return
  branch.value = 'main'
  void loadCount()
})

async function loadCount(): Promise<void> {
  try {
    await load(props.workspace?.rec_id || '')
  } catch (e) {
    toast.error(errText(e))
  }
}

function branchValidator(value: string): true | string {
  return isGitBranch(value) ? true : t('workspaces.batchGitBranchRule')
}

async function submit(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id) return
  const ok = await (branchInput.value?.validate() ?? Promise.resolve(true))
  if (!ok || !canSubmit.value) return
  saving.value = true
  try {
    const out = await updateWorkspaceGitBranches(id, branch.value.trim())
    if (out.updated === 0) {
      toast.error(t('workspaces.batchGitBranchEmpty'))
      return
    }
    toast.success(t('workspaces.batchGitBranchDone', { n: out.updated, branch: out.branch }))
    open.value = false
    emit('success')
  } catch (e) {
    toast.error(errText(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="form"
    width="sm"
    teleport-to="body"
    :title="t('workspaces.batchGitBranch')"
    :description="t('workspaces.batchGitBranchHint')"
  >
    <template #body>
      <div class="form-grid">
        <RsFormItem :label="t('workspaces.branch')" required>
          <RsInput
            ref="branchInput"
            v-model="branch"
            :placeholder="t('workspaces.branchPh')"
            required
            :validator="branchValidator"
            validate-trigger="both"
          />
        </RsFormItem>
        <div class="tags">
          <button v-for="item in presets" :key="item" type="button" class="tag" @click="branch = item">{{ item }}</button>
        </div>
        <p class="hint">
          {{
            loading
              ? t('workspaces.batchGitBranchLoading')
              : gitCount
                ? t('workspaces.batchGitBranchCount', { n: gitCount, name: workspace?.workspace_name || '' })
                : t('workspaces.batchGitBranchEmpty')
          }}
        </p>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" :disabled="saving" @click="open = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="saving" :disabled="!canSubmit || loading" @click="submit">{{ t('common.save') }}</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped src="./dialog.css"></style>
