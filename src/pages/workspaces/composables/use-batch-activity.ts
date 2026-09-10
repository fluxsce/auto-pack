import { computed, ref, type ComputedRef } from 'vue'
import { autoReleaseStatus } from '../api'
import { hasRunningAutoBuildSession } from '../auto-build/session'

const buildIds = new Set<string>()
const releaseLocalIds = new Set<string>()
const releaseServerIds = new Set<string>()
const epoch = ref(0)

function bump(): void {
  epoch.value += 1
}

function setFlag(bag: Set<string>, workspaceId: string, on: boolean): void {
  const id = workspaceId.trim()
  if (!id) return
  const had = bag.has(id)
  if (on && !had) {
    bag.add(id)
    bump()
  } else if (!on && had) {
    bag.delete(id)
    bump()
  }
}

/** 当前页自动构建（含预同步）占用该空间。 */
export function setAutoBuildBusy(workspaceId: string, busy: boolean): void {
  setFlag(buildIds, workspaceId, busy)
}

/** 当前页自动发版对话框占用该空间（含发版后构建）。 */
export function setAutoReleaseBusy(workspaceId: string, busy: boolean): void {
  setFlag(releaseLocalIds, workspaceId, busy)
}

export type BatchBusyReason = 'build' | 'release' | ''

export function useBatchActivity(workspaceId: () => string): {
  buildBusy: ComputedRef<boolean>
  releaseBusy: ComputedRef<boolean>
  busy: ComputedRef<boolean>
  reason: ComputedRef<BatchBusyReason>
  refresh: () => Promise<void>
} {
  const idOf = (): string => workspaceId().trim()
  const buildBusy = computed(() => {
    void epoch.value
    return buildIds.has(idOf())
  })
  const releaseBusy = computed(() => {
    void epoch.value
    const id = idOf()
    return releaseLocalIds.has(id) || releaseServerIds.has(id)
  })
  const busy = computed(() => buildBusy.value || releaseBusy.value)
  const reason = computed<BatchBusyReason>(() => {
    if (buildBusy.value) return 'build'
    if (releaseBusy.value) return 'release'
    return ''
  })

  async function refresh(): Promise<void> {
    const id = idOf()
    if (!id) return
    if (hasRunningAutoBuildSession(id)) setAutoBuildBusy(id, true)
    try {
      const st = await autoReleaseStatus(id)
      setFlag(releaseServerIds, id, st.status === 'running' && !st.foreign)
    } catch {
      /* 轮询失败不改本地占用 */
    }
  }

  return { buildBusy, releaseBusy, busy, reason, refresh }
}
