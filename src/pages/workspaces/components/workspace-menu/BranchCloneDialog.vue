<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsInput, RsLog, RsSwitch, useRsToast } from '@/ui'
import type { Workspace } from '@/stores/workspace'
import {
  errText,
  getJob,
  getJobLog,
  listProjects,
  submitJob,
  type JobLogView,
  type Project,
} from '../../api'
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
const fromBranch = ref('')
const branch = ref('')
const overwriteRemote = ref(true)
const running = ref(false)
const loading = ref(false)
const logText = ref('')
const projects = ref<Project[]>([])
const fromInput = ref<FieldHandle | null>(null)
const branchInput = ref<FieldHandle | null>(null)
let followGen = 0

const gitCount = computed(() => projects.value.length)
const canSubmit = computed(() => gitCount.value > 0 && isGitBranch(branch.value) && !running.value && !loading.value)

watch(open, (visible) => {
  if (!visible) return
  if (running.value) return
  fromBranch.value = ''
  branch.value = ''
  overwriteRemote.value = true
  logText.value = ''
  void loadTargets()
})

async function loadTargets(): Promise<void> {
  const id = props.workspace?.rec_id || ''
  if (!id) {
    projects.value = []
    return
  }
  loading.value = true
  try {
    const rows = await listProjects(id)
    projects.value = rows.filter(isGitProject)
  } catch (e) {
    projects.value = []
    toast.error(errText(e))
  } finally {
    loading.value = false
  }
}

function optionalBranchValidator(value: string): true | string {
  if (!value.trim()) return true
  return isGitBranch(value) ? true : t('workspaces.batchGitBranchRule')
}

function branchValidator(value: string): true | string {
  return isGitBranch(value) ? true : t('workspaces.batchGitBranchRule')
}

function appendLog(text: string): void {
  const chunk = text.replaceAll('\r\n', '\n').replaceAll('\r', '\n')
  if (!chunk) return
  if (!logText.value) {
    logText.value = chunk
    return
  }
  logText.value = logText.value.endsWith('\n') ? `${logText.value}${chunk}` : `${logText.value}\n${chunk}`
}

function mergeFollowLog(view: JobLogView, size: number): number {
  if (view.offset === 0) {
    if (size === 0 && view.log) appendLog(view.log)
    return view.size
  }
  if (view.log) appendLog(view.log)
  return view.size || size
}

async function followJob(id: string): Promise<string> {
  const gen = followGen
  let size = 0
  const terminal = new Set(['succeeded', 'failed', 'canceled', 'cancelled'])
  for (;;) {
    if (gen !== followGen) return 'canceled'
    const [view, info] = await Promise.all([
      getJobLog(id, size).catch(() => null),
      getJob(id),
    ])
    if (gen !== followGen) return 'canceled'
    if (view) size = mergeFollowLog(view, size)
    if (terminal.has(info.job.job_status)) return info.job.job_status
    await new Promise((resolve) => setTimeout(resolve, 1500))
  }
}

function sourceOf(row: Project): string {
  return fromBranch.value.trim() || (row.repo_branch || '').trim() || t('common.dash')
}

async function submit(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id) return
  const checks = await Promise.all([
    fromInput.value?.validate() ?? true,
    branchInput.value?.validate() ?? true,
  ])
  if (!checks.every(Boolean) || !canSubmit.value) return
  followGen += 1
  running.value = true
  logText.value = ''
  const to = branch.value.trim()
  const from = fromBranch.value.trim()
  let ok = 0
  let failed = 0
  appendLog(`[${new Date().toLocaleString()}] ${t('workspaces.batchBranchCloneStart', { n: projects.value.length, branch: to })}`)
  try {
    for (const [i, row] of projects.value.entries()) {
      appendLog(`[${new Date().toLocaleString()}] ${i + 1}/${projects.value.length} ${row.project_name}  ${sourceOf(row)} -> ${to}`)
      try {
        const job = await submitJob(id, row.rec_id, 'branch_clone', {
          from_branch: from || undefined,
          branch: to,
          force: overwriteRemote.value,
        })
        const status = await followJob(job.rec_id)
        if (status === 'succeeded') {
          ok += 1
          appendLog(t('workspaces.batchBranchCloneOk', { name: row.project_name }))
        } else {
          failed += 1
          appendLog(t('workspaces.batchBranchCloneFailed', { name: row.project_name, msg: status }))
        }
      } catch (e) {
        failed += 1
        appendLog(t('workspaces.batchBranchCloneFailed', { name: row.project_name, msg: errText(e) }))
      }
    }
    appendLog(t('workspaces.batchBranchCloneDoneLog', { succeeded: ok, failed }))
    if (failed > 0) toast.warning(t('workspaces.batchBranchClonePartial', { succeeded: ok, failed }))
    else toast.success(t('workspaces.batchBranchCloneDone', { n: ok, branch: to }))
    if (ok > 0) emit('success')
  } finally {
    running.value = false
  }
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    layout="form"
    :width="960"
    teleport-to="body"
    :title="t('workspaces.batchBranchClone')"
    :description="t('workspaces.batchBranchCloneHint')"
  >
    <template #body>
      <div class="clone-split">
        <div class="form-grid">
          <RsFormItem :label="t('workspaces.fromBranch')">
            <RsInput
              ref="fromInput"
              v-model="fromBranch"
              :disabled="running"
              :placeholder="t('workspaces.fromBranchPh')"
              :validator="optionalBranchValidator"
              validate-trigger="both"
            />
          </RsFormItem>
          <RsFormItem :label="t('workspaces.newBranch')" required>
            <RsInput
              ref="branchInput"
              v-model="branch"
              :disabled="running"
              :placeholder="t('workspaces.newBranchPh')"
              required
              :validator="branchValidator"
              validate-trigger="both"
            />
          </RsFormItem>
          <RsFormItem :label="t('workspaces.overwriteRemote')">
            <RsSwitch v-model="overwriteRemote" :disabled="running" />
          </RsFormItem>
          <p class="hint">{{ t('workspaces.fromBranchHint') }}</p>
          <p class="hint">{{ t('workspaces.overwriteRemoteHint') }}</p>
          <p class="hint">
            {{
              loading
                ? t('workspaces.batchGitBranchLoading')
                : gitCount
                  ? t('workspaces.batchBranchCloneCount', { n: gitCount, name: workspace?.workspace_name || '' })
                  : t('workspaces.batchGitBranchEmpty')
            }}
          </p>
        </div>
        <RsLog
          v-model:lines="logText"
          class="log"
          height="100%"
          :empty-text="t('workspaces.batchBranchCloneLogEmpty')"
          :aria-label="t('workspaces.batchBranchClone')"
        />
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" @click="open = false">{{ t('common.close') }}</RsButton>
      <RsButton :loading="running" :disabled="!canSubmit" @click="submit">{{ t('workspaces.batchBranchCloneExecute') }}</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped src="./dialog.css"></style>
<style scoped>
.clone-split {
  display: grid;
  grid-template-columns: minmax(0, 22rem) minmax(0, 1fr);
  gap: 0.85rem;
  min-height: 22rem;
}

.log {
  min-height: 22rem;
  height: 100%;
  min-width: 0;
}

@media (max-width: 800px) {
  .clone-split {
    grid-template-columns: 1fr;
  }
}
</style>
