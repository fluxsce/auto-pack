import type { RsContextMenuItem } from '@/ui'
import type { Workspace } from '@/stores/workspace'

export const WorkspaceMenuKey = {
  Export: 'export',
  ImportNew: 'import-new',
  ImportReplace: 'import-replace',
  GitBranch: 'git-branch',
  BranchClone: 'branch-clone',
  PomVersion: 'pom-version',
  BuildCommand: 'build-command',
  PostEmail: 'post-email',
  PostShell: 'post-shell',
} as const

export type WorkspaceMenuKey = (typeof WorkspaceMenuKey)[keyof typeof WorkspaceMenuKey]

export type WorkspaceMenuHandlers = {
  t: (key: string) => string
  canWriteProjects: (item: Workspace) => boolean
  isOwner: (item: Workspace) => boolean
  exportWorkspace: (item: Workspace) => void
  importNew: () => void
  importReplace: (item: Workspace) => void
  editGitBranch: (item: Workspace) => void
  cloneGitBranch: (item: Workspace) => void
  editPomVersion: (item: Workspace) => void
  editBuildCommand: (item: Workspace) => void
  editPostEmail: (item: Workspace) => void
  editPostShell: (item: Workspace) => void
}

/** 工作空间列表右键操作：导出导入、改分支、克隆分支、改 POM、改构建命令、构建后邮件/脚本。 */
export class WorkspaceContextMenu {
  constructor(private readonly handlers: WorkspaceMenuHandlers) {}

  items(item: Workspace): RsContextMenuItem[] {
    const { t, isOwner, canWriteProjects } = this.handlers
    const items: RsContextMenuItem[] = [
      { key: WorkspaceMenuKey.Export, label: t('workspaces.exportWorkspace'), icon: 'download' },
      { key: WorkspaceMenuKey.ImportNew, label: t('workspaces.importWorkspace'), icon: 'upload' },
    ]
    if (isOwner(item)) {
      items.push({ key: WorkspaceMenuKey.ImportReplace, label: t('workspaces.importReplace'), icon: 'refresh-cw' })
    }
    if (canWriteProjects(item)) {
      items.push(
        { key: 'sep-git-branch', label: '', separator: true },
        { key: WorkspaceMenuKey.GitBranch, label: t('workspaces.batchGitBranch'), icon: 'git-branch' },
        { key: WorkspaceMenuKey.BranchClone, label: t('workspaces.batchBranchClone'), icon: 'copy' },
        { key: WorkspaceMenuKey.PomVersion, label: t('workspaces.batchPomVersion'), icon: 'package' },
        { key: WorkspaceMenuKey.BuildCommand, label: t('workspaces.batchCommand'), icon: 'wrench' },
        { key: 'sep-post', label: '', separator: true },
        { key: WorkspaceMenuKey.PostEmail, label: t('workspaces.batchPostEmail'), icon: 'mail' },
        { key: WorkspaceMenuKey.PostShell, label: t('workspaces.batchPostShell'), icon: 'terminal' },
      )
    }
    return items
  }

  handle(item: Workspace, key: string): void {
    if (key === WorkspaceMenuKey.Export) {
      this.handlers.exportWorkspace(item)
      return
    }
    if (key === WorkspaceMenuKey.ImportNew) {
      this.handlers.importNew()
      return
    }
    if (key === WorkspaceMenuKey.ImportReplace) {
      this.handlers.importReplace(item)
      return
    }
    if (key === WorkspaceMenuKey.GitBranch) {
      this.handlers.editGitBranch(item)
      return
    }
    if (key === WorkspaceMenuKey.BranchClone) {
      this.handlers.cloneGitBranch(item)
      return
    }
    if (key === WorkspaceMenuKey.PomVersion) {
      this.handlers.editPomVersion(item)
      return
    }
    if (key === WorkspaceMenuKey.BuildCommand) {
      this.handlers.editBuildCommand(item)
      return
    }
    if (key === WorkspaceMenuKey.PostEmail) {
      this.handlers.editPostEmail(item)
      return
    }
    if (key === WorkspaceMenuKey.PostShell) {
      this.handlers.editPostShell(item)
    }
  }
}

export function canWriteWorkspaceProjects(item: Workspace): boolean {
  return item.member_role === 'owner' || item.member_role === 'operator'
}
