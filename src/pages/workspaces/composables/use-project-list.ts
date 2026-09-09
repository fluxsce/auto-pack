import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { rsConfirm, useRsToast, type RsContextMenuItem, type RsSelectModelValue, type RsTableColumn } from '@/ui'
import { useWorkspaceStore } from '@/stores/workspace'
import { asSelectString, deleteProject, errText, listProjects, submitBuild, type Project } from '../api'
import { parseRecipe, projectTypes } from '../lib/project-defaults'

export type TagVariant = 'default' | 'primary' | 'success' | 'warning' | 'danger' | 'info'

export function useProjectList() {
  const { t } = useI18n()
  const toast = useRsToast()
  const router = useRouter()
  const ws = useWorkspaceStore()
  const projects = ref<Project[]>([])
  const loading = ref(false)
  const busyId = ref('')
  const query = ref('')
  const typeFilter = ref('')
  const statusFilter = ref('')
  const autoBuildOpen = ref(false)
  const autoReleaseOpen = ref(false)
  const autoPushOpen = ref(false)

  const current = computed(() => ws.list.find((item) => item.rec_id === ws.currentId))

  const typeOptions = computed(() => projectTypes.map((item) => ({ value: item.value, label: item.label })))

  const statusOptions = computed(() => [
    { value: 'never', label: t('workspaces.neverBuilt') },
    { value: 'queued', label: t('jobs.statuses.queued') },
    { value: 'running', label: t('jobs.statuses.running') },
    { value: 'succeeded', label: t('jobs.statuses.succeeded') },
    { value: 'failed', label: t('jobs.statuses.failed') },
    { value: 'canceled', label: t('jobs.statuses.canceled') },
  ])

  const filtered = computed(() => {
    const q = query.value.trim().toLowerCase()
    return projects.value.filter((row) => {
      if (typeFilter.value && row.project_type !== typeFilter.value) return false
      if (statusFilter.value === 'never') {
        if (row.last_job_status) return false
      } else if (statusFilter.value) {
        const code = row.last_job_status || ''
        if (statusFilter.value === 'canceled') {
          if (code !== 'canceled' && code !== 'cancelled') return false
        } else if (code !== statusFilter.value) {
          return false
        }
      }
      if (!q) return true
      const source = projectSource(row)
      const hay = [row.project_name, source.location, source.branch, row.repo_url, row.repo_branch, row.cron_expr]
        .filter(Boolean)
        .join('\n')
        .toLowerCase()
      return hay.includes(q)
    })
  })

  const columns = computed<RsTableColumn<Project>[]>(() => [
    { key: 'project_name', title: t('workspaces.name'), minWidth: 160, sortable: true },
    { key: 'project_type', title: t('workspaces.type'), width: 110, sortable: true },
    {
      key: 'repo_branch',
      title: t('workspaces.branch'),
      width: 140,
      sortable: true,
      sorter: (left, right) => projectSource(left).branch.localeCompare(projectSource(right).branch),
    },
    { key: 'last_job_status', title: t('workspaces.buildStatus'), width: 120, sortable: true },
    {
      key: 'last_job_at',
      title: t('workspaces.updated'),
      minWidth: 168,
      sortable: true,
      sorter: (left, right) => stampMs(left.last_job_at || left.updated_at) - stampMs(right.last_job_at || right.updated_at),
    },
    { key: 'cron_expr', title: t('workspaces.cron'), width: 130, sortable: true },
    { key: 'actions', title: t('common.actions'), width: 220, align: 'right' },
  ])

  async function load(): Promise<void> {
    if (!ws.currentId) {
      projects.value = []
      return
    }
    loading.value = true
    try {
      projects.value = await listProjects(ws.currentId)
    } catch (e) {
      toast.error(errText(e))
    } finally {
      loading.value = false
    }
  }

  function resetFilters(): void {
    query.value = ''
    typeFilter.value = ''
    statusFilter.value = ''
  }

  function onTypeFilter(value: RsSelectModelValue): void {
    typeFilter.value = asSelectString(value)
  }

  function onStatusFilter(value: RsSelectModelValue): void {
    statusFilter.value = asSelectString(value)
  }

  onMounted(load)
  watch(() => ws.currentId, () => {
    resetFilters()
    void load()
  })
  watch(() => ws.projectsEpoch, () => {
    void load()
  })

  function create(): void {
    void router.push({ name: 'workspace-project-new' })
  }

  function openAutoBuild(): void {
    if (!ws.currentId) return
    autoBuildOpen.value = true
  }

  function openAutoRelease(): void {
    if (!ws.currentId) return
    autoReleaseOpen.value = true
  }

  function openAutoPush(): void {
    if (!ws.currentId) return
    autoPushOpen.value = true
  }

  function open(row: Project): void {
    void router.push({ name: 'workspace-project', params: { id: row.rec_id } })
  }

  function edit(row: Project): void {
    void router.push({ name: 'workspace-project-edit', params: { id: row.rec_id } })
  }

  function viewJobs(row: Project): void {
    void router.push({ name: 'jobs', query: { project: row.rec_id } })
  }

  async function submitJob(row: Project): Promise<void> {
    const workspaceId = row.workspace_rec_id || ws.currentId
    if (!workspaceId) return
    busyId.value = row.rec_id
    try {
      await submitBuild(workspaceId, row.rec_id)
      toast.success(t('workspaces.buildQueued'))
      await load()
    } catch (e) {
      toast.error(errText(e))
    } finally {
      busyId.value = ''
    }
  }

  function menuItems(row: Project | null): RsContextMenuItem[] {
    if (!row) return []
    return [
      { key: 'open', label: t('workspaces.detail.title'), icon: 'eye' },
      { key: 'configure', label: t('common.configure'), icon: 'pencil' },
      { key: 'clone', label: t('workspaces.cloneProject'), icon: 'copy' },
      { key: 'submit', label: t('workspaces.buildNow'), icon: 'play' },
      { key: 'jobs', label: t('workspaces.viewJobs'), icon: 'list' },
      { key: 'sep', label: '', separator: true },
      { key: 'delete', label: t('common.delete'), icon: 'trash-2', danger: true },
    ]
  }

  function onMenu(key: string, row: Project | null): void {
    if (!row) return
    if (key === 'open') {
      open(row)
      return
    }
    if (key === 'configure') {
      edit(row)
      return
    }
    if (key === 'clone') {
      clone(row)
      return
    }
    if (key === 'submit') {
      void submitJob(row)
      return
    }
    if (key === 'jobs') {
      viewJobs(row)
      return
    }
    if (key === 'delete') {
      void remove(row)
    }
  }

  function clone(row: Project): void {
    void router.push({ name: 'workspace-project-new', query: { cloneFrom: row.rec_id } })
  }

  async function remove(row: Project): Promise<void> {
    const ok = await rsConfirm.danger({
      title: t('common.delete'),
      description: t('workspaces.deleteProjectConfirm', { name: row.project_name }),
      confirmText: t('common.delete'),
    })
    if (!ok) {
      return
    }
    busyId.value = row.rec_id
    try {
      await deleteProject(row.rec_id)
      toast.success(t('common.deleted'))
      await load()
      await ws.load()
    } catch (e) {
      toast.error(errText(e))
    } finally {
      busyId.value = ''
    }
  }

  return {
    current,
    projects,
    filtered,
    query,
    typeFilter,
    statusFilter,
    typeOptions,
    statusOptions,
    loading,
    busyId,
    columns,
    onTypeFilter,
    onStatusFilter,
    autoBuildOpen,
    autoReleaseOpen,
    autoPushOpen,
    openAutoBuild,
    openAutoRelease,
    openAutoPush,
    load,
    create,
    open,
    edit,
    clone,
    remove,
    viewJobs,
    submitJob,
    menuItems,
    onMenu,
  }
}

export type ProjectSource = {
  kind: 'git' | 'local'
  location: string
  branch: string
}

/** 列表用的仓库展示：Git 用远程地址+分支，本地用相对路径（空则回退检出目录）。 */
export function projectSource(row: Project): ProjectSource {
  const recipe = parseRecipe(row.recipe_json, row.project_type)
  if (recipe.scm_mode === 'local') {
    const location = (recipe.local_source_path || row.work_rel_path || row.project_name || '').trim()
    return { kind: 'local', location, branch: '' }
  }
  return {
    kind: 'git',
    location: (row.repo_url || '').trim(),
    branch: (row.repo_branch || '').trim(),
  }
}

export function typeLabel(code: string): string {
  return projectTypes.find((item) => item.value === code)?.label ?? code
}

export function typeVariant(code: string): TagVariant {
  if (code === 'maven') return 'info'
  if (code === 'npm') return 'success'
  if (code === 'vue') return 'primary'
  if (code === 'go') return 'warning'
  return 'default'
}

export function statusVariant(code?: string): TagVariant {
  if (code === 'running') return 'info'
  if (code === 'succeeded') return 'success'
  if (code === 'failed') return 'danger'
  if (code === 'canceled' || code === 'cancelled') return 'warning'
  if (code === 'queued') return 'default'
  return 'default'
}

export function statusTone(code?: string): 'ok' | 'fail' | 'run' | 'warn' | 'muted' {
  if (code === 'succeeded') return 'ok'
  if (code === 'failed') return 'fail'
  if (code === 'running' || code === 'queued') return 'run'
  if (code === 'canceled' || code === 'cancelled') return 'warn'
  return 'muted'
}

export function stampMs(value?: string): number {
  if (!value) return 0
  const iso = value.includes('T') ? value : value.replace(' ', 'T')
  const zoned = /(?:[zZ]|[+-]\d{2}:?\d{2})$/.test(iso)
  const ms = new Date(zoned ? iso : `${iso}Z`).getTime()
  return Number.isNaN(ms) ? 0 : ms
}

export function formatStamp(value?: string, empty = '—'): string {
  const ms = stampMs(value)
  if (!value) return empty
  if (!ms) return value
  return new Date(ms).toLocaleString()
}
