import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useRsToast } from '@/ui'
import { useWorkspaceStore } from '@/stores/workspace'
import {
  errText,
  getJob,
  applyJobLog,
  getJobLog,
  getProject,
  isBuildJob,
  listJobs,
  submitBuild,
  type JobRow,
  type JobStep,
  type Project,
} from '../api'
import { isAutoBuildLive } from '../auto-build/runtime'
import { chooseBuildNow } from '../lib/build-now'
import { checkoutDisplay, parseRecipe } from '../lib/project-defaults'
import { asProjectDetailTab, type ProjectDetailTab } from '../lib/project-detail'

const LIVE = new Set(['queued', 'running'])
const POLL_MS = 3000

export function useProjectDetail() {
  const { t } = useI18n()
  const toast = useRsToast()
  const route = useRoute()
  const router = useRouter()
  const ws = useWorkspaceStore()
  const project = ref<Project | null>(null)
  const jobs = ref<JobRow[]>([])
  const steps = ref<JobStep[]>([])
  const logText = ref('')
  const logSize = ref(0)
  const selectedJobId = ref('')
  const loading = ref(false)
  const building = ref(false)
  const autoBuildOpen = ref(false)
  let pollId = 0
  let pageAlive = true

  const projectId = computed(() => String(route.params.id ?? ''))
  const tab = computed(() => asProjectDetailTab(route.query.tab))
  const current = computed(() => ws.list.find((item) => item.rec_id === (project.value?.workspace_rec_id || ws.currentId)))
  const recipe = computed(() => parseRecipe(project.value?.recipe_json || '', project.value?.project_type || 'maven'))
  const checkoutPath = computed(() =>
    checkoutDisplay(current.value?.root_dir || '', project.value?.project_name || '', recipe.value, project.value?.work_rel_path),
  )
  const latest = computed(() => jobs.value[0] ?? null)
  const selected = computed(() => jobs.value.find((item) => item.rec_id === selectedJobId.value) ?? latest.value)
  const live = computed(() => !!selected.value && LIVE.has(selected.value.job_status))

  function goTab(next: ProjectDetailTab): void {
    void router.replace({ query: { ...route.query, tab: next } })
  }

  function back(): void {
    void router.push({ name: 'workspaces' })
  }

  function configure(): void {
    if (!projectId.value) return
    void router.push({ name: 'workspace-project-edit', params: { id: projectId.value } })
  }

  async function loadProject(): Promise<void> {
    if (!projectId.value) {
      back()
      return
    }
    loading.value = true
    try {
      const row = await getProject(projectId.value)
      project.value = row
      if (row.workspace_rec_id && row.workspace_rec_id !== ws.currentId) {
        ws.select(row.workspace_rec_id)
      }
      await loadJobs()
    } catch (e) {
      toast.error(errText(e))
      back()
    } finally {
      loading.value = false
    }
  }

  function resetLog(): void {
    logText.value = ''
    logSize.value = 0
  }

  function patchJob(row?: JobRow): void {
    if (!row) return
    const i = jobs.value.findIndex((item) => item.rec_id === row.rec_id)
    if (i >= 0) {
      jobs.value[i] = { ...jobs.value[i], ...row }
      return
    }
    jobs.value = [row, ...jobs.value]
  }

  async function refreshJobList(): Promise<void> {
    const workspaceId = project.value?.workspace_rec_id || ws.currentId
    if (!workspaceId || !projectId.value) {
      jobs.value = []
      return
    }
    const rows = await listJobs(workspaceId, { projectId: projectId.value, jobType: 'build' })
    jobs.value = rows.filter((item) => isBuildJob(item))
    const qid = String(route.query.job ?? '')
    if (qid && jobs.value.some((item) => item.rec_id === qid)) {
      selectedJobId.value = qid
    } else if (!selectedJobId.value || !jobs.value.some((item) => item.rec_id === selectedJobId.value)) {
      selectedJobId.value = jobs.value[0]?.rec_id ?? ''
      resetLog()
    }
  }

  async function loadJobs(): Promise<void> {
    try {
      await refreshJobList()
      await loadSelected()
    } catch (e) {
      toast.error(errText(e))
    }
  }

  async function loadSelected(): Promise<void> {
    const id = selectedJobId.value
    if (!id) {
      steps.value = []
      resetLog()
      return
    }
    try {
      const [detail, view] = await Promise.all([
        getJob(id),
        getJobLog(id, 0).catch((e) => ({ log: errText(e), size: 0, offset: 0 })),
      ])
      patchJob(detail.job)
      steps.value = detail.steps ?? []
      const next = applyJobLog('', view)
      logText.value = next.text
      logSize.value = next.size
    } catch (e) {
      steps.value = []
      logText.value = errText(e)
    }
  }

  async function pollLive(): Promise<void> {
    if (!pageAlive) return
    try {
      await refreshJobList()
      const id = selectedJobId.value
      if (!id) return
      const wantLog = tab.value === 'console'
      const [detail, view] = await Promise.all([
        getJob(id),
        wantLog
          ? getJobLog(id, logSize.value).catch(() => ({ log: '', size: logSize.value, offset: logSize.value }))
          : Promise.resolve(null),
      ])
      if (!pageAlive) return
      patchJob(detail.job)
      steps.value = detail.steps ?? []
      if (view) {
        const next = applyJobLog(logText.value, view)
        logText.value = next.text
        logSize.value = next.size
      }
    } catch {
      /* keep the last good snapshot */
    }
  }

  function selectJob(id: string): void {
    selectedJobId.value = id
    void loadSelected()
    if (tab.value !== 'console') goTab('console')
  }

  async function buildNow(): Promise<void> {
    const workspaceId = project.value?.workspace_rec_id || ws.currentId
    const row = project.value
    if (!workspaceId || !projectId.value || !row) return
    building.value = true
    try {
      const choice = await chooseBuildNow(row, workspaceId, t, toast)
      if (choice === 'abort') return
      if (choice === 'ordered') {
        autoBuildOpen.value = true
        toast.info(t('workspaces.buildNowOrderOpen'))
        return
      }
      const job = await submitBuild(workspaceId, projectId.value)
      selectedJobId.value = job.rec_id
      toast.success(t('workspaces.buildQueued'))
      goTab('console')
      await loadJobs()
    } catch (e) {
      toast.error(errText(e))
    } finally {
      building.value = false
    }
  }

  const batchLive = computed(() => isAutoBuildLive(project.value?.workspace_rec_id || ws.currentId))

  function syncPoll(): void {
    if (pollId) window.clearInterval(pollId)
    pollId = 0
    if (!live.value && !batchLive.value) return
    pollId = window.setInterval(() => {
      void pollLive()
    }, POLL_MS)
  }

  onMounted(() => {
    pageAlive = true
    void loadProject()
  })
  onUnmounted(() => {
    pageAlive = false
    if (pollId) window.clearInterval(pollId)
  })
  watch(projectId, () => {
    selectedJobId.value = ''
    resetLog()
    void loadProject()
  })
  watch(
    () => String(route.query.job ?? ''),
    (id) => {
      if (id && id !== selectedJobId.value) {
        selectJob(id)
      }
    },
  )
  watch([live, batchLive], syncPoll, { immediate: true })

  return {
    project,
    jobs,
    steps,
    logText,
    selectedJobId,
    selected,
    latest,
    loading,
    building,
    autoBuildOpen,
    live,
    tab,
    current,
    recipe,
    checkoutPath,
    goTab,
    back,
    configure,
    buildNow,
    selectJob,
    loadJobs,
    loadProject,
  }
}
