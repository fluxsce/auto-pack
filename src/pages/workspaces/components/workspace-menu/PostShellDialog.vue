<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsRadio, RsRadioItem, RsSelect, RsTextarea, useRsToast, type RsSelectModelValue } from '@/ui'
import type { Workspace } from '@/stores/workspace'
import { asSelectString, errText, listProjects, updateWorkspacePostActions, type Project } from '../../api'
import { type CommandFamily, matchCommandFamily } from '../../lib/command-patch'
import { defaultPostShellCommand } from '../../lib/post-shell-presets'
import { normalizeWhen, postWhenKinds } from '../../lib/post-actions'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  workspace?: Workspace | null
}>()

const { t } = useI18n()
const toast = useRsToast()
const saving = ref(false)
const loading = ref(false)
const rows = ref<Project[]>([])
const family = ref<CommandFamily>('maven')
const when = ref('success')
const command = ref('')

const whenOptions = computed(() =>
  postWhenKinds.map((value) => ({ value, label: t(`workspaces.postWhen.${value}`) })),
)
const familyRows = computed(() => rows.value.filter((row) => matchCommandFamily(row.project_type, family.value)))
const canSubmit = computed(() => familyRows.value.length > 0 && !!command.value.trim() && !saving.value)
const familyLabel = computed(() => (family.value === 'maven' ? 'Maven' : 'Node / Vue'))

function onWhen(value: RsSelectModelValue): void {
  when.value = normalizeWhen(asSelectString(value))
}

function applyPreset(): void {
  command.value = defaultPostShellCommand(family.value)
}

watch(open, (visible) => {
  if (!visible) return
  family.value = 'maven'
  when.value = 'success'
  applyPreset()
  void loadRows()
})

watch(family, () => {
  applyPreset()
})

async function loadRows(): Promise<void> {
  const id = props.workspace?.rec_id || ''
  if (!id) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    rows.value = await listProjects(id)
  } catch (e) {
    rows.value = []
    toast.error(errText(e))
  } finally {
    loading.value = false
  }
}

async function submit(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id || !canSubmit.value) return
  saving.value = true
  try {
    const out = await updateWorkspacePostActions(id, {
      family: family.value,
      kind: 'shell',
      enabled: true,
      when: normalizeWhen(when.value),
      command: command.value.trim(),
    })
    if (out.updated === 0) {
      toast.warning(t('workspaces.batchPostShellEmpty'))
      return
    }
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
    width="md"
    teleport-to="body"
    :title="t('workspaces.batchPostShell')"
    :description="t('workspaces.batchPostShellHint')"
  >
    <template #body>
      <div class="form-grid">
        <RsFormItem :label="t('workspaces.batchCommandFamily')">
          <RsRadio v-model="family">
            <RsRadioItem value="maven">Maven</RsRadioItem>
            <RsRadioItem value="node">Node / Vue</RsRadioItem>
          </RsRadio>
        </RsFormItem>
        <RsFormItem :label="t('workspaces.batchPostShellPreset')">
          <div class="tags">
            <button type="button" class="tag" @click="applyPreset">
              {{
                family === 'maven'
                  ? t('workspaces.batchPostShellPresetMaven')
                  : t('workspaces.batchPostShellPresetNode')
              }}
            </button>
          </div>
        </RsFormItem>
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
          :rows="5"
        />
        <p class="hint">
          {{
            loading
              ? t('workspaces.batchPostLoading')
              : familyRows.length
                ? t('workspaces.batchPostShellCount', {
                    n: familyRows.length,
                    name: workspace?.workspace_name || '',
                    family: familyLabel,
                  })
                : t('workspaces.batchPostShellNoFamily')
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
<style scoped>
.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.tag {
  padding: 0.28rem 0.65rem;
  border: 1px solid var(--shell-card-border);
  border-radius: 999px;
  background: var(--shell-inset-bg);
  color: var(--shell-title);
  font-size: 0.78rem;
  cursor: pointer;
}

.tag:hover {
  border-color: rgba(14, 165, 233, 0.45);
  color: #0ea5e9;
}
</style>
