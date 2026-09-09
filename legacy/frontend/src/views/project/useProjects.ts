import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import { fetchEnvSettings } from '@/request/api/env'
import {
  createProject,
  deleteProject,
  fetchProject,
  fetchProjectTypes,
  fetchProjects,
  updateProject,
} from '@/request/api/project'
import { fetchWorkspaces } from '@/request/api/workspace'
import type {
  EnvSetting,
  PackBuildStatus,
  PackProject,
  PackProjectPayload,
  PackWorkspace,
  ProjectBuildConfig,
  ProjectType,
} from '@/types'
import { isActiveBuildStatus } from './project-build-status'
import {
  migrateDeployEnvToActions,
  primaryDeployEnvFromActions,
} from './post-build/post-build-defaults'
import { DEFAULT_BUILD_TRIGGERS } from '@/types/build-triggers'
import { DEFAULT_BUILD_CONFIG, mergeBuildConfig } from './project-defaults'
import { FALLBACK_PROJECT_TYPES, normalizeProjectTypes, projectTypeLabel } from './project-meta'
import { normalizeProjectFormFields, trimProjectFromApi } from './project-form-trim'
import { effectiveScmMode } from './project-scm'
import { workspaceOptionLabel } from './workspace/workspace-paths'

export type ProjectPanelMode = 'none' | 'create' | 'edit' | 'clone'
export type ProjectBuildStatusFilter = 'all' | 'none' | 'success' | 'failed'

export type WorkspaceManageDialogExpose = {
  setItems: (list: PackWorkspace[]) => void
}

function asItemList<T>(raw: T[] | null | undefined): T[] {
  return Array.isArray(raw) ? raw : []
}

export function useProjects() {
  const { t } = useI18n()
  const message = useMessage()

  const projectTypes = ref<ProjectType[]>([])
  const workspaces = ref<PackWorkspace[]>([])
  const activeWorkspaceRecId = ref<string>('')
  const activeType = ref<string>('all')
  const searchQuery = ref('')
  const buildStatusFilter = ref<ProjectBuildStatusFilter>('all')
  const items = ref<PackProject[]>([])
  const envItems = ref<EnvSetting[]>([])
  const loading = ref(false)
  const panelMode = ref<ProjectPanelMode>('none')
  const saving = ref(false)
  const editingId = ref<string | null>(null)
  const workspaceDialogOpen = ref(false)

  const form = reactive<PackProjectPayload>({
    workspace_rec_id: '',
    project_name: '',
    project_version: '1.0.0',
    project_type: 'maven',
    repo_url: '',
    repo_branch: 'main',
    config_json: '{}',
    git_env_rec_id: '',
    jdk_env_rec_id: '',
    maven_env_rec_id: '',
    nodejs_env_rec_id: '',
    deploy_env_rec_id: '',
    is_enabled: 1,
    sort_num: 0,
    remark: '',
  })

  const buildConfig = ref<ProjectBuildConfig>({ ...DEFAULT_BUILD_CONFIG.maven })

  const activeWorkspace = computed(() =>
    workspaces.value.find((w) => w.rec_id === activeWorkspaceRecId.value),
  )

  const activeWorkspaceLabel = computed(() => {
    const ws = activeWorkspace.value
    if (!ws) return ''
    return workspaceOptionLabel(ws)
  })

  function matchesBuildStatusFilter(row: PackProject): boolean {
    const f = buildStatusFilter.value
    if (f === 'all') return true
    const st = row.last_build_status
    if (f === 'none') return !st
    return st === f
  }

  function matchesSearchQuery(row: PackProject): boolean {
    const needle = searchQuery.value.trim().toLowerCase()
    if (!needle) return true
    const hay = [
      row.project_name,
      row.project_version,
      row.repo_branch,
      row.repo_url,
      row.project_type,
    ]
      .join(' ')
      .toLowerCase()
    return hay.includes(needle)
  }

  const filteredItems = computed(() => {
    let list = items.value
    if (activeType.value !== 'all') {
      list = list.filter((i) => i.project_type === activeType.value)
    }
    list = list.filter(matchesBuildStatusFilter)
    list = list.filter(matchesSearchQuery)
    return list
  })

  const hasListFilters = computed(
    () =>
      activeType.value !== 'all' ||
      buildStatusFilter.value !== 'all' ||
      searchQuery.value.trim() !== '',
  )

  let listPollTimer: ReturnType<typeof setInterval> | null = null

  function stopListPoll(): void {
    if (listPollTimer) {
      clearInterval(listPollTimer)
      listPollTimer = null
    }
  }

  function syncListPoll(): void {
    stopListPoll()
    const needsPoll = items.value.some((p) => isActiveBuildStatus(p.last_build_status ?? undefined))
    if (needsPoll) {
      listPollTimer = setInterval(() => void loadList(true), 5000)
    }
  }

  const editorOpen = computed(() => panelMode.value !== 'none')

  const editorTitle = computed(() => {
    if (panelMode.value === 'create') return t('projectCreateTitle')
    if (panelMode.value === 'clone') return t('projectCloneTitle')
    return t('projectEditTitle')
  })

  function suggestCloneName(baseName: string): string {
    const suffix = t('projectCloneNameSuffix')
    const names = new Set(items.value.map((i) => i.project_name))
    let candidate = `${baseName}${suffix}`
    let n = 2
    while (names.has(candidate)) {
      candidate = `${baseName}${suffix}${n}`
      n += 1
    }
    return candidate
  }

  function typeLabel(type: unknown): string {
    return projectTypeLabel(t, type)
  }

  function envOptions(envType: string): EnvSetting[] {
    const kind = envType.trim().toLowerCase()
    return envItems.value.filter((e) => e.env_type?.trim().toLowerCase() === kind)
  }

  function parseBuildConfig(): void {
    const merged = mergeBuildConfig(form.project_type, form.config_json)
    if (!merged.post_build_actions?.length && form.deploy_env_rec_id.trim()) {
      merged.post_build_actions = migrateDeployEnvToActions(form.deploy_env_rec_id)
    }
    merged.post_build_actions = merged.post_build_actions ?? []
    if (!merged.build_triggers) {
      merged.build_triggers = { depends_on: [], schedule: { enabled: false, cron: '0 2 * * *' } }
    }
    buildConfig.value = merged
  }

  function applyProjectToForm(row: PackProject): void {
    const p = trimProjectFromApi(row)
    form.workspace_rec_id = p.workspace_rec_id
    form.project_name = p.project_name
    form.project_version = p.project_version
    form.project_type = p.project_type
    form.repo_url = p.repo_url
    form.repo_branch = p.repo_branch
    form.config_json = p.config_json
    form.git_env_rec_id = p.git_env_rec_id
    form.jdk_env_rec_id = p.jdk_env_rec_id
    form.maven_env_rec_id = p.maven_env_rec_id
    form.nodejs_env_rec_id = p.nodejs_env_rec_id
    form.deploy_env_rec_id = p.deploy_env_rec_id
    form.is_enabled = p.is_enabled
    form.sort_num = p.sort_num
    form.remark = p.remark
    parseBuildConfig()
  }

  function buildProjectPayload(): PackProjectPayload {
    normalizeProjectFormFields(form)
    return {
      workspace_rec_id: form.workspace_rec_id,
      project_name: form.project_name,
      project_version: form.project_version,
      project_type: form.project_type,
      repo_url: form.repo_url,
      repo_branch: form.repo_branch,
      work_dir: '',
      config_json: form.config_json,
      git_env_rec_id: form.git_env_rec_id,
      jdk_env_rec_id: form.jdk_env_rec_id,
      maven_env_rec_id: form.maven_env_rec_id,
      nodejs_env_rec_id: form.nodejs_env_rec_id,
      deploy_env_rec_id: form.deploy_env_rec_id,
      is_enabled: form.is_enabled,
      sort_num: form.sort_num,
      remark: form.remark,
    }
  }

  function syncConfigJson(): void {
    form.deploy_env_rec_id = primaryDeployEnvFromActions(
      buildConfig.value.post_build_actions ?? [],
    )
    form.config_json = JSON.stringify(buildConfig.value)
  }

  function resetForm(type: ProjectType = 'maven'): void {
    editingId.value = null
    form.workspace_rec_id = activeWorkspaceRecId.value
    form.project_name = ''
    form.project_version = '1.0.0'
    form.project_type = type
    form.repo_url = ''
    form.repo_branch = 'main'
    form.git_env_rec_id = ''
    form.jdk_env_rec_id = ''
    form.maven_env_rec_id = ''
    form.nodejs_env_rec_id = ''
    form.deploy_env_rec_id = ''
    form.is_enabled = 1
    form.sort_num = 0
    form.remark = ''
    form.config_json = '{}'
    buildConfig.value = {
      ...structuredClone(DEFAULT_BUILD_CONFIG[type]),
      scm_mode: 'local',
      local_source_path: '',
    }
  }

  function openCreate(): void {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    const type = activeType.value === 'all' ? 'maven' : (activeType.value as ProjectType)
    resetForm(type)
    panelMode.value = 'create'
  }

  async function openEdit(row: PackProject): Promise<void> {
    editingId.value = row.rec_id
    panelMode.value = 'edit'
    applyProjectToForm(row)
    try {
      const fresh = await fetchProject(row.rec_id)
      applyProjectToForm(fresh)
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  async function openClone(row: PackProject): Promise<void> {
    if (!activeWorkspaceRecId.value) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    panelMode.value = 'clone'
    editingId.value = null
    applyProjectToForm(row)
    form.project_name = suggestCloneName(row.project_name)
    try {
      const fresh = await fetchProject(row.rec_id)
      applyProjectToForm(fresh)
      form.project_name = suggestCloneName(fresh.project_name)
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  function closeEditor(): void {
    panelMode.value = 'none'
    editingId.value = null
  }

  function onTypeChange(type: ProjectType): void {
    form.project_type = type
    const prev = buildConfig.value
    const defaults = DEFAULT_BUILD_CONFIG[type]
    buildConfig.value = {
      ...structuredClone(defaults),
      scm_mode: prev.scm_mode ?? 'local',
      local_source_path: prev.local_source_path ?? '',
      log_retention_days: prev.log_retention_days ?? defaults.log_retention_days,
      post_build_actions: prev.post_build_actions ?? [],
      build_triggers: prev.build_triggers ?? structuredClone(DEFAULT_BUILD_TRIGGERS),
    }
  }

  function onWorkspaceFilterChange(): void {
    void loadList()
  }

  async function loadWorkspaces(): Promise<void> {
    try {
      const res = await fetchWorkspaces()
      workspaces.value = asItemList(res.items).filter((w) => w.is_enabled === 1)
      if (!activeWorkspaceRecId.value) {
        const def = workspaces.value.find((w) => w.is_default === 1) ?? workspaces.value[0]
        if (def) activeWorkspaceRecId.value = def.rec_id
      }
    } catch (e) {
      message.error((e as Error).message)
      workspaces.value = []
    }
  }

  async function loadList(silent = false): Promise<void> {
    if (!activeWorkspaceRecId.value) {
      items.value = []
      stopListPoll()
      return
    }
    if (!silent) loading.value = true
    try {
      const res = await fetchProjects({ workspace_rec_id: activeWorkspaceRecId.value })
      items.value = asItemList(res.items)
      syncListPoll()
    } catch (e) {
      if (!silent) message.error((e as Error).message)
      items.value = []
      stopListPoll()
    } finally {
      if (!silent) loading.value = false
    }
  }

  async function loadEnvs(): Promise<void> {
    try {
      const res = await fetchEnvSettings()
      envItems.value = asItemList(res.items)
    } catch {
      envItems.value = []
    }
  }

  async function onSave(): Promise<void> {
    syncConfigJson()
    if (!form.workspace_rec_id) {
      message.warning(t('workspaceSelectRequired'))
      return
    }
    if (!form.project_name.trim()) {
      message.warning(t('projectNameRequired'))
      return
    }
    if (!form.project_version.trim()) {
      message.warning(t('projectVersionRequired'))
      return
    }
    if (effectiveScmMode(form, buildConfig.value) === 'git' && !form.repo_url.trim()) {
      message.warning(t('projectScmGitRepoRequired'))
      return
    }
    saving.value = true
    const payload = buildProjectPayload()
    try {
      if (editingId.value) {
        const saved = await updateProject(editingId.value, payload)
        applyProjectToForm(saved)
        message.success(t('projectSaveSuccess'))
      } else {
        const created = await createProject(payload)
        editingId.value = created.rec_id
        panelMode.value = 'edit'
        applyProjectToForm(created)
        message.success(t('projectCreateSuccess'))
      }
      await loadList()
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      saving.value = false
    }
  }

  async function onDelete(row: PackProject): Promise<void> {
    try {
      await deleteProject(row.rec_id)
      message.success(t('projectDeleteSuccess'))
      if (editingId.value === row.rec_id) closeEditor()
      await loadList()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  async function onWorkspacesChanged(
    dialog?: WorkspaceManageDialogExpose | null,
  ): Promise<void> {
    const prev = activeWorkspaceRecId.value
    try {
      const res = await fetchWorkspaces()
      const all = asItemList(res.items)
      dialog?.setItems(all)
      workspaces.value = all.filter((w) => w.is_enabled === 1)

      if (prev && workspaces.value.some((w) => w.rec_id === prev)) {
        activeWorkspaceRecId.value = prev
        return
      }

      const next = workspaces.value[0]?.rec_id ?? ''
      const selectionChanged = next !== prev
      activeWorkspaceRecId.value = next
      if (selectionChanged) await loadList()
    } catch (e) {
      message.error((e as Error).message)
      workspaces.value = []
      dialog?.setItems([])
    }
  }

  onUnmounted(() => {
    stopListPoll()
  })

  onMounted(async () => {
    try {
      const res = await fetchProjectTypes()
      projectTypes.value = normalizeProjectTypes(res.types)
    } catch {
      projectTypes.value = [...FALLBACK_PROJECT_TYPES]
    }
    await loadWorkspaces()
    await Promise.all([loadList(), loadEnvs()])
  })

  return {
    projectTypes,
    workspaces,
    activeWorkspaceRecId,
    activeWorkspace,
    activeWorkspaceLabel,
    activeType,
    searchQuery,
    buildStatusFilter,
    hasListFilters,
    items,
    envItems,
    loading,
    panelMode,
    editorOpen,
    saving,
    editingId,
    workspaceDialogOpen,
    form,
    buildConfig,
    filteredItems,
    editorTitle,
    typeLabel,
    envOptions,
    openCreate,
    openEdit,
    openClone,
    closeEditor,
    onTypeChange,
    onSave,
    onDelete,
    onWorkspaceFilterChange,
    onWorkspacesChanged,
    loadEnvs,
    loadList,
  }
}
