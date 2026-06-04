import type { PackWorkspace } from '@/types'

/** Backend-resolved root; when root_dir empty, falls back to data/workspaces/{code}. */
export function effectiveWorkspaceRoot(
  ws: Pick<PackWorkspace, 'root_dir' | 'effective_root_dir' | 'workspace_code'>,
): string {
  const effective = ws.effective_root_dir?.trim()
  if (effective) return effective
  const custom = ws.root_dir?.trim()
  if (custom) return custom
  const code = ws.workspace_code?.trim() || 'default'
  return `data/workspaces/${code}`
}

export function workspaceOptionLabel(ws: PackWorkspace): string {
  return `${ws.workspace_name} (${ws.workspace_code})`
}
