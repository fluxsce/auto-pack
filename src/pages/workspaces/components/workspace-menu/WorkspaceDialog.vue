<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsInput, useRsToast } from '@/ui'
import { useWorkspaceStore, type Workspace } from '@/stores/workspace'
import { cloneWorkspace, createWorkspace, errText, updateWorkspace, type WorkspaceMode } from '../../api'
import { isSafeDisplayName, isWorkspaceCode, UNSAFE_NAME_CHARS } from '../../lib/path-slug'

type FieldHandle = { validate: () => Promise<boolean> }

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  mode: WorkspaceMode
  editing?: Workspace | null
}>()

const { t } = useI18n()
const toast = useRsToast()
const ws = useWorkspaceStore()
const form = ref({ rec_id: '', source_rec_id: '', code: '', name: '', remark: '' })
const codeInput = ref<FieldHandle | null>(null)
const nameInput = ref<FieldHandle | null>(null)

const title = computed(() => {
  if (props.mode === 'edit') return t('workspaces.editWorkspace')
  if (props.mode === 'clone') return t('workspaces.cloneWorkspace')
  return t('workspaces.newWorkspace')
})

watch(open, (visible) => {
  if (!visible) return
  const row = props.editing
  if (props.mode === 'edit' && row) {
    form.value = {
      rec_id: row.rec_id,
      source_rec_id: '',
      code: row.workspace_code,
      name: row.workspace_name,
      remark: row.remark || '',
    }
    return
  }
  if (props.mode === 'clone' && row) {
    form.value = {
      rec_id: '',
      source_rec_id: row.rec_id,
      code: `${row.workspace_code}-copy`,
      name: t('workspaces.cloneNameSuffix', { name: row.workspace_name }),
      remark: '',
    }
    return
  }
  form.value = { rec_id: '', source_rec_id: '', code: '', name: '', remark: '' }
})

function slugValidator(value: string): true | string {
  return isWorkspaceCode(value) ? true : t('workspaces.codeRule')
}

function nameValidator(value: string): true | string {
  return isSafeDisplayName(value) ? true : t('workspaces.nameRule', { chars: UNSAFE_NAME_CHARS })
}

async function submit(): Promise<void> {
  const checks = [nameInput.value?.validate() ?? true]
  if (props.mode !== 'edit') {
    checks.unshift(codeInput.value?.validate() ?? true)
  }
  const ok = (await Promise.all(checks)).every(Boolean)
  if (!ok) return
  try {
    if (props.mode === 'edit') {
      await updateWorkspace(form.value.rec_id, { workspace_name: form.value.name, remark: form.value.remark })
      toast.success(t('common.saved'))
      open.value = false
      await ws.load()
      return
    }
    const created =
      props.mode === 'clone'
        ? await cloneWorkspace({
            source_rec_id: form.value.source_rec_id,
            workspace_code: form.value.code,
            workspace_name: form.value.name,
          })
        : await createWorkspace({ workspace_code: form.value.code, workspace_name: form.value.name })
    toast.success(t('common.saved'))
    open.value = false
    await ws.load()
    if (created?.rec_id) ws.select(created.rec_id)
  } catch (e) {
    toast.error(errText(e))
  }
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="form"
    width="sm"
    teleport-to="body"
    :title="title"
    :description="mode === 'clone' ? t('workspaces.cloneHint') : undefined"
  >
    <template #body>
      <div class="form-grid">
        <RsFormItem :label="t('workspaces.code')" :tooltip="t('workspaces.codeHint')" required>
          <RsInput
            ref="codeInput"
            v-model="form.code"
            :placeholder="t('workspaces.code')"
            :disabled="mode === 'edit'"
            required
            :validator="slugValidator"
            validate-trigger="both"
          />
        </RsFormItem>
        <RsFormItem :label="t('workspaces.wsName')" :tooltip="t('workspaces.nameHint', { chars: UNSAFE_NAME_CHARS })" required>
          <RsInput
            ref="nameInput"
            v-model="form.name"
            :placeholder="t('workspaces.wsName')"
            required
            :validator="nameValidator"
            validate-trigger="both"
          />
        </RsFormItem>
        <RsInput
          v-if="mode === 'edit' && editing?.root_dir"
          :model-value="editing.root_dir"
          :label="t('workspaces.root')"
          disabled
        />
        <RsInput
          v-if="mode === 'edit'"
          v-model="form.remark"
          :label="t('workspaces.remark')"
          :placeholder="t('workspaces.remark')"
        />
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" @click="open = false">{{ t('common.cancel') }}</RsButton>
      <RsButton @click="submit">{{ mode === 'clone' ? t('workspaces.cloneWorkspace') : t('common.save') }}</RsButton>
    </template>
  </RsDialog>
</template>
