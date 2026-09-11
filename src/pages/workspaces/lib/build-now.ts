import { useRsToast } from '@/ui'
import { autoBuildStatus, fetchMavenOrder, listProjects, startAutoBuild, type MavenAnalysis, type Project } from '../api'
import { autoBuildBag, bumpAutoBuild, isAutoBuildLive } from '../auto-build/runtime'
import { setAutoBuildBusy } from '../composables/use-batch-activity'
import { isOrderedBuildType, joinPreview, missingUpstreamProjects, unmatchedCoords } from './ordered-build'

export type BuildNowChoice = 'single' | 'ordered' | 'abort'

type Translate = (key: string, params?: Record<string, string | number>) => string

function followAutoBuild(workspaceId: string): void {
  const slot = autoBuildBag(workspaceId)
  if (slot.pollTimer) return
  const tick = async (): Promise<void> => {
    try {
      const st = await autoBuildStatus(workspaceId)
      if (st.status === 'running') {
        slot.running = true
        slot.followRecId = st.build_rec_id || slot.followRecId
        setAutoBuildBusy(workspaceId, true)
        bumpAutoBuild()
        return
      }
      if (slot.pollTimer) {
        clearInterval(slot.pollTimer)
        slot.pollTimer = null
      }
      slot.running = false
      setAutoBuildBusy(workspaceId, false)
      bumpAutoBuild()
    } catch {
      /* 跟进失败不打断批次 */
    }
  }
  void tick()
  slot.pollTimer = setInterval(() => {
    void tick()
  }, 2000)
}

/** 按给定工程列表开跑；exact 时不展开 pom.xml 全部上游。不打开自动构建对话框。 */
export async function startSelectedAutoBuild(
  workspaceId: string,
  projectIds: string[],
  analysis?: MavenAnalysis | null,
  exact = false,
): Promise<void> {
  const started = await startAutoBuild({
    workspace_rec_id: workspaceId,
    scope: 'selected',
    project_rec_ids: projectIds,
    exact_targets: exact,
  })
  const slot = autoBuildBag(workspaceId)
  slot.scope = 'selected'
  slot.selectedRecIds = [...projectIds]
  if (analysis) slot.analysis = analysis
  slot.followRecId = started.build_rec_id
  slot.running = true
  slot.selectedHistory = ''
  bumpAutoBuild()
  setAutoBuildBusy(workspaceId, true)
  followAutoBuild(workspaceId)
}

/**
 * 立即构建：分析当前工程 pom.xml / package.json 依赖，补齐空间内尚未成功的上游，再构建本工程。
 * 不弹确认、不打开自动构建对话框。
 */
export async function chooseBuildNow(
  project: Project,
  workspaceId: string,
  t: Translate,
  toast: ReturnType<typeof useRsToast>,
): Promise<BuildNowChoice> {
  if (!isOrderedBuildType(project.project_type)) return 'single'
  if (isAutoBuildLive(workspaceId)) {
    toast.error(t('workspaces.autoBatchBusyBuild'))
    return 'abort'
  }
  let analysis: MavenAnalysis
  try {
    analysis = await fetchMavenOrder(workspaceId)
  } catch {
    toast.warning(t('workspaces.buildNowAnalyzeFail'))
    return 'single'
  }
  const peers = await listProjects(workspaceId).catch(() => [] as Project[])
  const { missing, disabled } = missingUpstreamProjects(project.rec_id, analysis, peers)
  const unmatched = unmatchedCoords(project.rec_id, analysis)
  if (disabled.length) {
    toast.warning(t('workspaces.buildNowDisabledSkip', { list: joinPreview(disabled.map((p) => p.project_name)) }))
  }
  if (unmatched.length) {
    toast.warning(t('workspaces.buildNowUnmatchedHint', { list: joinPreview(unmatched) }))
  }
  if (!missing.length) return 'single'
  await startSelectedAutoBuild(
    workspaceId,
    [...missing.map((p) => p.rec_id), project.rec_id],
    analysis,
    true,
  )
  toast.info(
    t('workspaces.buildNowFilled', {
      count: missing.length,
      list: joinPreview(missing.map((p) => p.project_name)),
    }),
  )
  return 'ordered'
}
