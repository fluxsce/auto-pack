import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import {
  fetchProjectBuildLog,
  fetchProjectBuilds,
  fetchProjectWorkspace,
  triggerProjectBuild,
} from '@/request/api/build'
import { fetchProject } from '@/request/api/project'
import type { PackBuild, PackProject, PackWorkspace, ProjectWorkspaceEntry } from '@/types'
import { resolveProjectSourceDisplay } from './project-scm'

export type ProjectDetailSection = 'status' | 'workspace' | 'log'

export function useProjectDetail(getWorkspace: () => PackWorkspace | null | undefined) {
  const { t } = useI18n()
  const message = useMessage()

  const project = ref<PackProject | null>(null)
  const builds = ref<PackBuild[]>([])
  const loading = ref(false)
  const building = ref(false)
  const activeSection = ref<ProjectDetailSection>('status')
  const selectedBuildId = ref<string | null>(null)
  const logContent = ref('')
  const logOffset = ref(0)
  const logTotal = ref(0)
  const logLoading = ref(false)
  const logComplete = ref(true)

  const workspacePath = ref('.')
  const workspaceEntries = ref<ProjectWorkspaceEntry[]>([])
  const workspaceExists = ref(true)
  const workspaceAbsPath = ref('')
  const workspaceLoading = ref(false)

  let logPollTimer: ReturnType<typeof setInterval> | null = null

  const selectedBuild = computed(() =>
    builds.value.find((b) => b.rec_id === selectedBuildId.value) ?? null,
  )

  const lastBuild = computed(() => builds.value[0] ?? null)

  const workPathPreview = computed(() => {
    const p = project.value
    const ws = getWorkspace()
    if (!p) return ''
    return resolveProjectSourceDisplay(p, ws)
  })

  function stopLogPoll(): void {
    if (logPollTimer) {
      clearInterval(logPollTimer)
      logPollTimer = null
    }
  }

  function isLogFullyRead(): boolean {
    return logOffset.value >= logTotal.value
  }

  function shouldKeepLogPolling(): boolean {
    return !logComplete.value || !isLogFullyRead()
  }

  async function loadProject(recId: string): Promise<void> {
    loading.value = true
    try {
      project.value = await fetchProject(recId)
    } catch (e) {
      message.error((e as Error).message)
      project.value = null
    } finally {
      loading.value = false
    }
  }

  async function loadBuilds(recId: string): Promise<void> {
    try {
      const res = await fetchProjectBuilds(recId)
      builds.value = res.items ?? []
      if (!selectedBuildId.value && builds.value.length > 0) {
        selectedBuildId.value = builds.value[0].rec_id
      }
    } catch {
      builds.value = []
    }
  }

  async function refreshAll(recId: string): Promise<void> {
    await Promise.all([loadProject(recId), loadBuilds(recId)])
  }

  async function onBuildNow(): Promise<void> {
    if (!project.value) return
    building.value = true
    try {
      const build = await triggerProjectBuild(project.value.rec_id)
      message.success(t('projectBuildTriggered', { num: build.build_num }))
      await loadBuilds(project.value.rec_id)
      selectedBuildId.value = build.rec_id
      activeSection.value = 'log'
      await loadLog(true)
      startLogPoll()
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      building.value = false
    }
  }

  async function loadLog(reset = false): Promise<void> {
    if (!project.value || !selectedBuildId.value) return
    logLoading.value = true
    try {
      if (reset) {
        logContent.value = ''
        logOffset.value = 0
        logTotal.value = 0
      }

      for (;;) {
        const res = await fetchProjectBuildLog(
          project.value.rec_id,
          selectedBuildId.value,
          logOffset.value,
        )
        if (res.content) {
          logContent.value += res.content
        }
        logOffset.value = res.offset + res.size
        logTotal.value = res.total
        logComplete.value = res.complete

        const idx = builds.value.findIndex((b) => b.rec_id === res.build.rec_id)
        if (idx >= 0) builds.value[idx] = res.build

        if (!reset) break
        if (!res.complete || isLogFullyRead()) break
      }

      if (!shouldKeepLogPolling()) stopLogPoll()
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      logLoading.value = false
    }
  }

  function startLogPoll(): void {
    stopLogPoll()
    logPollTimer = setInterval(() => {
      if (!shouldKeepLogPolling()) {
        stopLogPoll()
        return
      }
      void loadLog(false)
    }, 2000)
  }

  async function selectBuild(buildId: string): Promise<void> {
    selectedBuildId.value = buildId
    activeSection.value = 'log'
    await loadLog(true)
    if (shouldKeepLogPolling()) startLogPoll()
    else stopLogPoll()
  }

  async function loadWorkspace(path = workspacePath.value): Promise<void> {
    if (!project.value) return
    workspaceLoading.value = true
    try {
      const res = await fetchProjectWorkspace(project.value.rec_id, path)
      workspacePath.value = res.path
      workspaceEntries.value = res.entries ?? []
      workspaceExists.value = res.exists
      workspaceAbsPath.value = res.abs_path
    } catch (e) {
      message.error((e as Error).message)
      workspaceEntries.value = []
    } finally {
      workspaceLoading.value = false
    }
  }

  async function openWorkspaceEntry(entry: ProjectWorkspaceEntry): Promise<void> {
    if (!project.value || !entry.is_dir) return
    await loadWorkspace(entry.path)
  }

  function openSection(section: ProjectDetailSection): void {
    activeSection.value = section
    if (section === 'workspace' && project.value) {
      void loadWorkspace('.')
    }
    if (section === 'log' && selectedBuildId.value) {
      void loadLog(true).then(() => {
        if (shouldKeepLogPolling()) startLogPoll()
      })
    }
  }

  function workspaceUp(): void {
    if (workspacePath.value === '.' || !workspacePath.value) return
    const parts = workspacePath.value.split('/').filter(Boolean)
    parts.pop()
    void loadWorkspace(parts.length ? parts.join('/') : '.')
  }

  function reset(): void {
    stopLogPoll()
    project.value = null
    builds.value = []
    selectedBuildId.value = null
    logContent.value = ''
    logOffset.value = 0
    logTotal.value = 0
    activeSection.value = 'status'
    workspacePath.value = '.'
    workspaceEntries.value = []
  }

  onUnmounted(() => stopLogPoll())

  return {
    project,
    builds,
    loading,
    building,
    activeSection,
    selectedBuildId,
    selectedBuild,
    lastBuild,
    logContent,
    logLoading,
    logComplete,
    workPathPreview,
    workspacePath,
    workspaceEntries,
    workspaceExists,
    workspaceAbsPath,
    workspaceLoading,
    refreshAll,
    onBuildNow,
    selectBuild,
    loadBuilds,
    loadWorkspace,
    openWorkspaceEntry,
    openSection,
    workspaceUp,
    reset,
    stopLogPoll,
  }
}
