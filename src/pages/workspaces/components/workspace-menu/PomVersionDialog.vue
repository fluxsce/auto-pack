<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsFormItem, RsInput, RsLog, useRsToast } from '@/ui'
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
import { isPomVersion } from '../../lib/path-slug'
import { isMavenProject } from '../../lib/workspace-targets'

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
const version = ref('')
const running = ref(false)
const loading = ref(false)
const logText = ref('')
const projects = ref<Project[]>([])
const versionInput = ref<FieldHandle | null>(null)
let followGen = 0

const mavenCount = computed(() => projects.value.length)
const canSubmit = computed(() => mavenCount.value > 0 && isPomVersion(version.value) && !running.value && !loading.value)

watch(open, (visible) => {
  if (!visible) return
  if (running.value) return
  version.value = ''
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
    projects.value = rows.filter(isMavenProject)
  } catch (e) {
    projects.value = []
    toast.error(errText(e))
  } finally {
    loading.value = false
  }
}

function versionValidator(value: string): true | string {
  return isPomVersion(value) ? true : t('workspaces.batchPomVersionRule')
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

async function submit(): Promise<void> {
  const id = props.workspace?.rec_id
  if (!id) return
  const okField = await (versionInput.value?.validate() ?? Promise.resolve(true))
  if (!okField || !canSubmit.value) return
  followGen += 1
  running.value = true
  logText.value = ''
  const ver = version.value.trim()
  let ok = 0
  let failed = 0
  appendLog(`[${new Date().toLocaleString()}] ${t('workspaces.batchPomVersionStart', { n: projects.value.length, version: ver })}`)
  try {
    for (const [i, row] of projects.value.entries()) {
      const branch = (row.repo_branch || '').trim() || t('common.dash')
      appendLog(`[${new Date().toLocaleString()}] ${i + 1}/${projects.value.length} ${row.project_name}  ${branch} -> ${ver}`)
      try {
        const job = await submitJob(id, row.rec_id, 'pom_version', { pom_version: ver })
        const status = await followJob(job.rec_id)
        if (status === 'succeeded') {
          ok += 1
          appendLog(t('workspaces.batchPomVersionOk', { name: row.project_name }))
        } else {
          failed += 1
          appendLog(t('workspaces.batchPomVersionFailed', { name: row.project_name, msg: status }))
        }
      } catch (e) {
        failed += 1
        appendLog(t('workspaces.batchPomVersionFailed', { name: row.project_name, msg: errText(e) }))
      }
    }
    appendLog(t('workspaces.batchPomVersionDoneLog', { succeeded: ok, failed }))
    if (failed > 0) toast.warning(t('workspaces.batchPomVersionPartial', { succeeded: ok, failed }))
    else toast.success(t('workspaces.batchPomVersionDone', { n: ok, version: ver }))
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
    :title="t('workspaces.batchPomVersion')"
    :description="t('workspaces.batchPomVersionHint')"
  >
    <template #body>
      <div class="pom-split">
        <div class="form-grid">
          <RsFormItem :label="t('jobs.types.pom_version')" required>
            <RsInput
              ref="versionInput"
              v-model="version"
              :disabled="running"
              :placeholder="t('workspaces.batchPomVersionPh')"
              required
              :validator="versionValidator"
              validate-trigger="both"
            />
          </RsFormItem>
          <p class="hint">{{ t('workspaces.batchPomVersionSyncHint') }}</p>
          <p class="hint">
            {{
              loading
                ? t('workspaces.batchPomVersionLoading')
                : mavenCount
                  ? t('workspaces.batchPomVersionCount', { n: mavenCount, name: workspace?.workspace_name || '' })
                  : t('workspaces.batchPomVersionEmpty')
            }}
          </p>
        </div>
        <RsLog
          v-model:lines="logText"
          class="log"
          height="100%"
          :empty-text="t('workspaces.batchPomVersionLogEmpty')"
          :aria-label="t('workspaces.batchPomVersion')"
        />
      </div>
    </template>
    <template #footer>
      <RsButton variant="ghost" @click="open = false">{{ t('common.close') }}</RsButton>
      <RsButton :loading="running" :disabled="!canSubmit" @click="submit">{{ t('workspaces.batchPomVersionExecute') }}</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped src="./dialog.css"></style>
<style scoped>
.pom-split {
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
  .pom-split {
    grid-template-columns: 1fr;
  }
}
</style>
