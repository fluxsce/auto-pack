export const projectDetailTabs = ['status', 'console', 'history', 'files', 'deps'] as const

export type ProjectDetailTab = (typeof projectDetailTabs)[number]

export const projectDetailIcons: Record<ProjectDetailTab, string> = {
  status: 'activity',
  console: 'terminal',
  history: 'history',
  files: 'folder',
  deps: 'share-2',
}

export function asProjectDetailTab(value: unknown): ProjectDetailTab {
  return projectDetailTabs.includes(value as ProjectDetailTab) ? (value as ProjectDetailTab) : 'status'
}
