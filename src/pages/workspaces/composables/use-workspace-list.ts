import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { rsConfirm, useRsToast, type RsContextMenuItem } from '@/ui'
import { useWorkspaceStore, type Workspace } from '@/stores/workspace'
import {
  deleteWorkspace,
  downloadWorkspaceBundle,
  errText,
  exportWorkspace,
  type WorkspaceBundle,
  type WorkspaceMode,
} from '../api'
import { canWriteWorkspaceProjects, WorkspaceContextMenu } from './workspace-context-menu'

export function useWorkspaceList() {
  const { t } = useI18n()
  const toast = useRsToast()
  const route = useRoute()
  const router = useRouter()
  const ws = useWorkspaceStore()
  const filter = ref('')
  const busyId = ref('')
  const dialogOpen = ref(false)
  const dialogMode = ref<WorkspaceMode>('create')
  const editing = ref<Workspace | null>(null)
  const bundleOpen = ref(false)
  const bundleMode = ref<'new' | 'replace'>('new')
  const bundleTarget = ref<Workspace | null>(null)
  const bundle = ref<WorkspaceBundle | null>(null)
  const fileInput = ref<HTMLInputElement | null>(null)
  const branchOpen = ref(false)
  const cloneOpen = ref(false)
  const pomOpen = ref(false)
  const commandOpen = ref(false)
  const emailOpen = ref(false)
  const shellOpen = ref(false)
  const actionTarget = ref<Workspace | null>(null)

  const filtered = computed(() => {
    const q = filter.value.trim().toLowerCase()
    if (!q) return ws.list
    return ws.list.filter((item) => {
      return item.workspace_name.toLowerCase().includes(q) || item.workspace_code.toLowerCase().includes(q)
    })
  })

  function isOwner(item: Workspace): boolean {
    return item.member_role === 'owner'
  }

  function roleLabel(role?: string): string {
    if (!role) return ''
    const key = `users.roles.${role}`
    const label = t(key)
    return label === key ? role : label
  }

  function select(item: Workspace): void {
    ws.select(item.rec_id)
    if (route.name !== 'workspaces') {
      void router.push({ name: 'workspaces' })
    }
  }

  function openCreate(): void {
    editing.value = null
    dialogMode.value = 'create'
    dialogOpen.value = true
  }

  function openEdit(item: Workspace): void {
    editing.value = item
    dialogMode.value = 'edit'
    dialogOpen.value = true
  }

  function openClone(item: Workspace): void {
    editing.value = item
    dialogMode.value = 'clone'
    dialogOpen.value = true
  }

  const contextMenu = new WorkspaceContextMenu({
    t: (key) => t(key),
    isOwner,
    canWriteProjects: canWriteWorkspaceProjects,
    exportWorkspace: (item) => {
      void doExport(item)
    },
    importNew: () => {
      openImport('new')
    },
    importReplace: (item) => {
      openImport('replace', item)
    },
    editGitBranch: (item) => {
      actionTarget.value = item
      branchOpen.value = true
    },
    cloneGitBranch: (item) => {
      actionTarget.value = item
      cloneOpen.value = true
    },
    editPomVersion: (item) => {
      actionTarget.value = item
      pomOpen.value = true
    },
    editBuildCommand: (item) => {
      actionTarget.value = item
      commandOpen.value = true
    },
    editPostEmail: (item) => {
      actionTarget.value = item
      emailOpen.value = true
    },
    editPostShell: (item) => {
      actionTarget.value = item
      shellOpen.value = true
    },
  })

  function menuItems(item: Workspace): RsContextMenuItem[] {
    return contextMenu.items(item)
  }

  function onMenu(item: Workspace, key: string): void {
    contextMenu.handle(item, key)
  }

  function openImport(mode: 'new' | 'replace', target: Workspace | null = null): void {
    bundleMode.value = mode
    bundleTarget.value = target
    bundle.value = null
    fileInput.value?.click()
  }

  async function onFile(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file) return
    try {
      const parsed = JSON.parse(await file.text()) as WorkspaceBundle
      if (!parsed || typeof parsed !== 'object' || !parsed.workspace) {
        throw new Error(t('workspaces.importInvalid'))
      }
      bundle.value = parsed
      bundleOpen.value = true
    } catch (e) {
      toast.error(errText(e instanceof SyntaxError ? new Error(t('workspaces.importInvalid')) : e))
    }
  }

  function onGitBranchSuccess(): void {
    if (actionTarget.value?.rec_id === ws.currentId) {
      ws.refreshProjects()
    }
  }

  async function doExport(item: Workspace): Promise<void> {
    busyId.value = item.rec_id
    try {
      const data = await exportWorkspace(item.rec_id)
      downloadWorkspaceBundle(data)
      toast.success(t('workspaces.exportDone'))
    } catch (e) {
      toast.error(errText(e))
    } finally {
      busyId.value = ''
    }
  }

  async function remove(item: Workspace): Promise<void> {
    if (item.is_default) return
    const ok = await rsConfirm.danger({
      title: t('common.delete'),
      description: t('workspaces.deleteConfirm', { name: item.workspace_name, n: item.project_count ?? 0 }),
      confirmText: t('common.delete'),
    })
    if (!ok) {
      return
    }
    busyId.value = item.rec_id
    try {
      await deleteWorkspace(item.rec_id)
      toast.success(t('common.deleted'))
      const wasCurrent = ws.currentId === item.rec_id
      await ws.load()
      if (wasCurrent) {
        const next = ws.list[0]?.rec_id ?? ''
        if (next) ws.select(next)
      }
    } catch (e) {
      toast.error(errText(e))
    } finally {
      busyId.value = ''
    }
  }

  return {
    ws,
    filter,
    busyId,
    dialogOpen,
    dialogMode,
    editing,
    filtered,
    isOwner,
    roleLabel,
    select,
    openCreate,
    openEdit,
    openClone,
    openImport,
    onMenu,
    menuItems,
    doExport,
    remove,
    fileInput,
    onFile,
    bundleOpen,
    bundleMode,
    bundleTarget,
    bundle,
    branchOpen,
    onGitBranchSuccess,
    cloneOpen,
    pomOpen,
    commandOpen,
    emailOpen,
    shellOpen,
    actionTarget,
  }
}
