import { ref } from 'vue'
import type { MavenAnalysis, Project, WorkspaceBuildRecord } from '../api'
import type { AutoBuildScope } from './session'

export type AutoBuildBag = {
  scope: AutoBuildScope
  selectedRecIds: string[]
  includeNonMaven: boolean
  loadingPlan: boolean
  running: boolean
  logText: string
  planned: Project[]
  warnings: string[]
  projects: Project[]
  analysis: MavenAnalysis | null
  syncHint: string
  planTask: Promise<void> | null
  history: WorkspaceBuildRecord[]
  selectedHistory: string
  followRecId: string
  pollTimer: ReturnType<typeof setInterval> | null
}

const bags = new Map<string, AutoBuildBag>()

/** 任意空间的自动构建袋变更时 +1，供当前弹窗重读。 */
export const autoBuildEpoch = ref(0)

export function emptyAutoBuildBag(): AutoBuildBag {
  return {
    scope: 'workspace',
    selectedRecIds: [],
    includeNonMaven: true,
    loadingPlan: false,
    running: false,
    logText: '',
    planned: [],
    warnings: [],
    projects: [],
    analysis: null,
    syncHint: '',
    planTask: null,
    history: [],
    selectedHistory: '',
    followRecId: '',
    pollTimer: null,
  }
}

export function autoBuildBag(workspaceId: string): AutoBuildBag {
  const id = workspaceId.trim()
  if (!id) return emptyAutoBuildBag()
  let bag = bags.get(id)
  if (!bag) {
    bag = emptyAutoBuildBag()
    bags.set(id, bag)
  }
  return bag
}

export function isAutoBuildLive(workspaceId: string): boolean {
  const id = workspaceId.trim()
  if (!id) return false
  const bag = bags.get(id)
  return Boolean(bag && (bag.running || bag.pollTimer != null))
}

export function stopAutoBuildPolls(): void {
  for (const bag of bags.values()) {
    if (bag.pollTimer) {
      clearInterval(bag.pollTimer)
      bag.pollTimer = null
    }
  }
}

export function bumpAutoBuild(): void {
  autoBuildEpoch.value += 1
}
