export const projectDetailTabs = ['status', 'console', 'files', 'deps'] as const

export type ProjectDetailTab = (typeof projectDetailTabs)[number]

export const projectDetailIcons: Record<ProjectDetailTab, string> = {
  status: 'activity',
  console: 'terminal',
  files: 'folder',
  deps: 'share-2',
}

export function asProjectDetailTab(value: unknown): ProjectDetailTab {
  if (value === 'history') return 'console'
  return projectDetailTabs.includes(value as ProjectDetailTab) ? (value as ProjectDetailTab) : 'status'
}
