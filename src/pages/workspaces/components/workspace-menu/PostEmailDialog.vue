<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsInput, RsSelect, RsSwitch, useRsToast, type RsSelectModelValue } from '@/ui'
import type { Workspace } from '@/stores/workspace'
import { asSelectString, errText, listWorkspaceEnvs, updateWorkspacePostActions } from '../../api'
import { useWorkspaceTargetCount } from '../../composables/use-workspace-target-count'
import { normalizeWhen, postWhenKinds } from '../../lib/post-actions'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  workspace?: Workspace | null
}>()

const { t } = useI18n()
const toast = useRsToast()
const saving = ref(false)
const envId = ref('')
const when = ref('success')
const to = ref('')
const subject = ref('')
const attachLog = ref(true)
const smtpOptions = ref<{ value: string; label: string }[]>([])
const { loading, count, load } = useWorkspaceTargetCount(() => true)

const whenOptions = computed(() =>
  postWhenKinds.map((value) => ({ value, label: t(`workspaces.postWhen.${value}`) })),
)
const canSubmit = computed(() => count.value > 0 && !!envId.value && !saving.value)

function onEnv(value: RsSelectModelValue): void {
  envId.value = asSelectString(value)
}

function onWhen(value: RsSelectModelValue): void {
  when.value = normalizeWhen(asSelectString(value))
}

watch(open, (visible) => {
  if (!visible) return
  envId.value = ''
  when.value = 'success'
  to.value = ''
  subject.value = ''
  attachLog.value = true
  void loadAll()
})

async function loadAll(): Promise<void> {
  const id = props.workspace?.rec_id || ''
  try {
    await load(id)
    if (!id) {
      smtpOptions.value = []
      return
    }
    const rows = await listWorkspaceEnvs(id)
    smtpOptions.value = rows
      .filter((item) => item.env_kind === 'smtp' && item.is_enabled !== 0)
      .map((item) => ({ value: item.rec_id, label: item.env_name }))
  } catch (e) {
    smtpOptions.value = []
    toast.error(errText(e))
  }
}

async function submit(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id || !canSubmit.value) return
  saving.value = true
  try {
    const out = await updateWorkspacePostActions(id, {
      kind: 'email',
      enabled: true,
      env_rec_id: envId.value,
      when: normalizeWhen(when.value),
      to: to.value.trim() || undefined,
      subject: subject.value.trim() || undefined,
      attach_log: attachLog.value,
    })
    toast.success(t('workspaces.batchPostEmailDone', { n: out.updated }))
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
    :title="t('workspaces.batchPostEmail')"
    :description="t('workspaces.batchPostEmailHint')"
  >
    <template #body>
      <div class="form-grid">
        <RsFormItem :label="t('workspaces.envPick')" required>
          <RsSelect
            :model-value="envId"
            :options="smtpOptions"
            :placeholder="t('workspaces.envPick')"
            @update:model-value="onEnv"
          />
        </RsFormItem>
        <RsFormItem :label="t('workspaces.postWhenLabel')">
          <RsSelect
            :model-value="when"
            :options="whenOptions"
            @update:model-value="onWhen"
          />
        </RsFormItem>
        <RsInput v-model="to" :label="t('workspaces.mailTo')" :placeholder="t('workspaces.mailToPh')" />
        <p class="hint">{{ t('workspaces.mailToHint') }}</p>
        <RsInput v-model="subject" :label="t('workspaces.mailSubject')" :placeholder="t('workspaces.mailSubjectPh')" />
        <RsFormItem :label="t('workspaces.mailAttachLog')">
          <RsSwitch v-model="attachLog" />
        </RsFormItem>
        <p class="hint">{{ t('workspaces.mailAttachLogHint') }}</p>
        <p class="hint">
          {{
            loading
              ? t('workspaces.batchPostLoading')
              : !count
                ? t('workspaces.empty')
                : !smtpOptions.length
                  ? t('workspaces.batchPostEmailNoEnv')
                  : t('workspaces.batchPostCount', { n: count, name: workspace?.workspace_name || '' })
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
