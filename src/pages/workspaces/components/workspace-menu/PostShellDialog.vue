<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsSelect, RsTextarea, useRsToast, type RsSelectModelValue } from '@/ui'
import type { Workspace } from '@/stores/workspace'
import { asSelectString, errText, updateWorkspacePostActions } from '../../api'
import { useWorkspaceTargetCount } from '../../composables/use-workspace-target-count'
import { normalizeWhen, postWhenKinds } from '../../lib/post-actions'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  workspace?: Workspace | null
}>()

const { t } = useI18n()
const toast = useRsToast()
const saving = ref(false)
const when = ref('success')
const command = ref('')
const { loading, count, load } = useWorkspaceTargetCount(() => true)

const whenOptions = computed(() =>
  postWhenKinds.map((value) => ({ value, label: t(`workspaces.postWhen.${value}`) })),
)
const canSubmit = computed(() => count.value > 0 && !!command.value.trim() && !saving.value)

function onWhen(value: RsSelectModelValue): void {
  when.value = normalizeWhen(asSelectString(value))
}

watch(open, (visible) => {
  if (!visible) return
  when.value = 'success'
  command.value = ''
  void loadCount()
})

async function loadCount(): Promise<void> {
  try {
    await load(props.workspace?.rec_id || '')
  } catch (e) {
    toast.error(errText(e))
  }
}

async function submit(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id || !canSubmit.value) return
  saving.value = true
  try {
    const out = await updateWorkspacePostActions(id, {
      kind: 'shell',
      enabled: true,
      when: normalizeWhen(when.value),
      command: command.value.trim(),
    })
    toast.success(t('workspaces.batchPostShellDone', { n: out.updated }))
    open.value = false
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
    :title="t('workspaces.batchPostShell')"
    :description="t('workspaces.batchPostShellHint')"
  >
    <template #body>
      <div class="form-grid">
        <RsFormItem :label="t('workspaces.postWhenLabel')">
          <RsSelect
            :model-value="when"
            :options="whenOptions"
            @update:model-value="onWhen"
          />
        </RsFormItem>
        <RsTextarea
          v-model="command"
          :label="t('workspaces.shellCommand')"
          :placeholder="t('workspaces.shellCommandPh')"
          :rows="4"
        />
        <p class="hint">
          {{
            loading
              ? t('workspaces.batchPostLoading')
              : count
                ? t('workspaces.batchPostCount', { n: count, name: workspace?.workspace_name || '' })
                : t('workspaces.empty')
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
