<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsAlert, RsButton, RsDialog, RsFormItem, RsInput, useRsToast } from '@/ui'
import { useWorkspaceStore, type Workspace } from '@/stores/workspace'
import { errText, importWorkspace, type WorkspaceBundle } from '../../api'
import { isSafeDisplayName, isWorkspaceCode, uniqueCopyName, UNSAFE_NAME_CHARS } from '../../lib/path-slug'

type FieldHandle = { validate: () => Promise<boolean> }

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  mode: 'new' | 'replace'
  target?: Workspace | null
  bundle: WorkspaceBundle | null
}>()

const emit = defineEmits<{
  imported: [id: string]
}>()

const { t } = useI18n()
const toast = useRsToast()
const ws = useWorkspaceStore()
const saving = ref(false)
const form = ref({ code: '', name: '' })
const codeInput = ref<FieldHandle | null>(null)
const nameInput = ref<FieldHandle | null>(null)

const title = computed(() =>
  submitAsReplace.value ? t('workspaces.importReplace') : t('workspaces.importWorkspace'),
)
const projectCount = computed(() => props.bundle?.projects?.length ?? 0)
const formatHint = computed(() => {
  const ver = props.bundle?.format_version ?? 0
  if (ver === 1) return t('workspaces.importV1Hint')
  return t('workspaces.importHint')
})

const conflict = computed(() => {
  if (props.mode === 'replace') return props.target ?? null
  return findByCode(form.value.code)
})
const canReplaceConflict = computed(() => conflict.value?.member_role === 'owner')
const submitAsReplace = computed(() => props.mode === 'replace' || Boolean(conflict.value && canReplaceConflict.value))
const suggestedCopyCode = computed(() => {
  const taken = ws.list.map((item) => item.workspace_code)
  const base = form.value.code.trim() || props.bundle?.workspace?.workspace_code || 'workspace'
  return uniqueCopyName(base, taken)
})

watch(open, (visible) => {
  if (!visible) return
  const meta = props.bundle?.workspace
  form.value = {
    code: meta?.workspace_code || '',
    name: meta?.workspace_name || '',
  }
})

function findByCode(code: string): Workspace | null {
  const key = code.trim().toLowerCase()
  if (!key) return null
  return ws.list.find((item) => item.workspace_code.toLowerCase() === key) ?? null
}

function slugValidator(value: string): true | string {
  if (!isWorkspaceCode(value)) return t('workspaces.codeRule')
  const hit = findByCode(value)
  if (hit && hit.member_role !== 'owner') return t('workspaces.importCodeTaken')
  return true
}

function nameValidator(value: string): true | string {
  return isSafeDisplayName(value) ? true : t('workspaces.nameRule', { chars: UNSAFE_NAME_CHARS })
}

function applyCopyCode(): void {
  const sourceName = props.bundle?.workspace?.workspace_name || form.value.name
  form.value.code = suggestedCopyCode.value
  form.value.name = t('workspaces.cloneNameSuffix', { name: sourceName })
}

function importError(e: unknown): string {
  const msg = errText(e)
  if (/workspace code already exists|UK_AP_WS_CODE|Duplicate entry/i.test(msg)) {
    return t('workspaces.importCodeTaken')
  }
  return msg
}

async function submit(): Promise<void> {
  if (!props.bundle) return
  if (props.mode === 'new') {
    const checks = [codeInput.value?.validate() ?? true]
    if (!submitAsReplace.value) {
      checks.push(nameInput.value?.validate() ?? true)
    }
    const ok = (await Promise.all(checks)).every(Boolean)
    if (!ok) return
    if (conflict.value && !canReplaceConflict.value) return
  }
  saving.value = true
  try {
    const replace = submitAsReplace.value
    const out = await importWorkspace({
      mode: replace ? 'replace' : 'new',
      target_workspace_rec_id: replace ? (conflict.value?.rec_id ?? props.target?.rec_id) : undefined,
      workspace_code: replace ? undefined : form.value.code,
      workspace_name: replace ? undefined : form.value.name,
      bundle: props.bundle,
    })
    toast.success(t('workspaces.importDone', { n: out.projects_imported }))
    open.value = false
    await ws.load()
    if (out.rec_id) {
      ws.select(out.rec_id)
      emit('imported', out.rec_id)
    }
  } catch (e) {
    toast.error(importError(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <RsDialog v-model:open="open" layout="form" width="sm" teleport-to="body" :title="title" :description="formatHint">
    <template #body>
      <div class="form-grid">
        <p v-if="mode === 'replace'" class="page__quiet">
          {{ t('workspaces.importReplaceHint', { name: target?.workspace_name ?? '', n: projectCount }) }}
        </p>
        <template v-else>
          <RsFormItem :label="t('workspaces.code')" :tooltip="t('workspaces.codeHint')" required>
            <RsInput
              ref="codeInput"
              v-model="form.code"
              :placeholder="t('workspaces.code')"
              required
              :validator="slugValidator"
              validate-trigger="both"
            />
          </RsFormItem>
          <div v-if="conflict" class="import-conflict">
            <RsAlert type="warning">
              {{
                canReplaceConflict
                  ? t('workspaces.importConflict', { code: form.code, name: conflict.workspace_name })
                  : t('workspaces.importConflictNoAccess', { code: form.code })
              }}
            </RsAlert>
            <RsButton size="sm" variant="ghost" @click="applyCopyCode">
              {{ t('workspaces.importUseCopyCode', { code: suggestedCopyCode }) }}
            </RsButton>
          </div>
          <RsFormItem
            v-if="!submitAsReplace"
            :label="t('workspaces.wsName')"
            :tooltip="t('workspaces.nameHint', { chars: UNSAFE_NAME_CHARS })"
            required
          >
            <RsInput
              ref="nameInput"
              v-model="form.name"
              :placeholder="t('workspaces.wsName')"
              required
              :validator="nameValidator"
              validate-trigger="both"
            />
          </RsFormItem>
          <p v-if="submitAsReplace" class="page__quiet">
            {{ t('workspaces.importReplaceHint', { name: conflict?.workspace_name ?? '', n: projectCount }) }}
          </p>
          <p v-else class="page__quiet">{{ t('workspaces.importCount', { n: projectCount }) }}</p>
        </template>
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" :disabled="saving" @click="open = false">{{ t('common.cancel') }}</RsButton>
      <RsButton :loading="saving" :disabled="!bundle || Boolean(conflict && !canReplaceConflict)" @click="submit">
        {{ submitAsReplace ? t('workspaces.importReplace') : t('workspaces.importWorkspace') }}
      </RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.import-conflict {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.45rem;
}
</style>
