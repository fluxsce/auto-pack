import { reactive } from 'vue'
import type { Project, WorkspaceReleaseRecord } from '../api'

export type ReleaseOutcomeKind = 'idle' | 'running' | 'success' | 'partial' | 'failed' | 'foreign'

export type AutoReleasePushForm = {
  envRecId: string
  pushPath: string
  backupPath: string
  pusher: string
  taskRef: string
  detail: string
}

export type AutoReleaseBag = {
  releaseBranch: string
  tagBranch: string
  releaseNote: string
  releaseDate: string
  updatePomVersion: boolean
  createTargetBranch: boolean
  targetBranch: string
  autoBuildAfterRelease: boolean
  autoFtpPushAfterBuild: boolean
  ftpPushProjectIds: string[]
  pushForm: AutoReleasePushForm
  backupManual: boolean
  remoteNote: string
  remoteNoteState: 'idle' | 'loading' | 'exists' | 'missing' | 'error'
  envOptions: { value: string; label: string }[]
  projects: Project[]
  logText: string
  logOutcome: { kind: ReleaseOutcomeKind; text: string }
  releasing: boolean
  postProcessing: boolean
  saving: boolean
  history: WorkspaceReleaseRecord[]
  selectedHistory: string
  pollTimer: ReturnType<typeof setInterval> | null
  handledPostFor: string
  postTask: Promise<void> | null
  applyingPost: boolean
}

const bags = new Map<string, AutoReleaseBag>()

export function emptyAutoReleaseBag(): AutoReleaseBag {
  return {
    releaseBranch: '',
    tagBranch: '',
    releaseNote: '',
    releaseDate: '',
    updatePomVersion: true,
    createTargetBranch: false,
    targetBranch: '',
    autoBuildAfterRelease: true,
    autoFtpPushAfterBuild: true,
    ftpPushProjectIds: [],
    pushForm: { envRecId: '', pushPath: '', backupPath: '', pusher: '', taskRef: '', detail: '' },
    backupManual: false,
    remoteNote: '',
    remoteNoteState: 'idle',
    envOptions: [],
    projects: [],
    logText: '',
    logOutcome: { kind: 'idle', text: '' },
    releasing: false,
    postProcessing: false,
    saving: false,
    history: [],
    selectedHistory: '',
    pollTimer: null,
    handledPostFor: '',
    postTask: null,
    applyingPost: false,
  }
}

export function autoReleaseBag(workspaceId: string): AutoReleaseBag {
  const id = workspaceId.trim()
  if (!id) return reactive(emptyAutoReleaseBag())
  let bag = bags.get(id)
  if (!bag) {
    bag = reactive(emptyAutoReleaseBag())
    bags.set(id, bag)
  }
  return bag
}

export function isAutoReleaseLive(workspaceId: string): boolean {
  const id = workspaceId.trim()
  if (!id) return false
  const bag = bags.get(id)
  return Boolean(bag && (bag.releasing || bag.postProcessing || bag.postTask != null))
}

export function stopAutoReleasePolls(): void {
  for (const bag of bags.values()) {
    if (bag.pollTimer) {
      clearInterval(bag.pollTimer)
      bag.pollTimer = null
    }
  }
}
