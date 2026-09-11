import { rsConfirm, useRsToast } from '@/ui'
import { fetchMavenOrder, startAutoBuild, type MavenAnalysis, type Project } from '../api'
import { autoBuildBag, bumpAutoBuild, isAutoBuildLive } from '../auto-build/runtime'
import { setAutoBuildBusy } from '../composables/use-batch-activity'
import { isOrderedBuildType, joinPreview, unmatchedCoords, upstreamModules } from './ordered-build'

export type BuildNowChoice = 'single' | 'ordered' | 'abort'

type Translate = (key: string, params?: Record<string, string | number>) => string

/** 按「选中工程 + pom.xml 上游」立刻开跑；对话框只跟进度，不再让用户重选范围。 */
export async function startSelectedAutoBuild(
  workspaceId: string,
  projectIds: string[],
  analysis?: MavenAnalysis | null,
): Promise<void> {
  const started = await startAutoBuild({
    workspace_rec_id: workspaceId,
    scope: 'selected',
    project_rec_ids: projectIds,
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
}

/**
 * 立即构建 POM / npm 工程前先读工作空间依赖图：
 * 有同空间上游则建议走自动构建；只有空间外 SNAPSHOT 则警告后仍可单建。
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
  const upstreams = upstreamModules(project.rec_id, analysis)
  const unmatched = unmatchedCoords(project.rec_id, analysis)
  if (!upstreams.length && !unmatched.length) return 'single'

  if (upstreams.length) {
    const list = joinPreview(
      upstreams.map((m) => (m.coordinates ? `${m.project_name}（${m.coordinates}）` : m.project_name)),
    )
    const extra = unmatched.length ? `\n${t('workspaces.buildNowBothExtra', { list: joinPreview(unmatched) })}` : ''
    const ok = await rsConfirm.confirm({
      title: t('workspaces.buildNowUpstreamTitle'),
      description: `${t('workspaces.buildNowUpstreamDesc', {
        name: project.project_name,
        count: upstreams.length,
        list,
      })}${extra}`,
      confirmText: t('workspaces.buildNowUpstreamConfirm'),
      cancelText: t('workspaces.buildNowUpstreamCancel'),
    })
    if (ok) {
      await startSelectedAutoBuild(workspaceId, [project.rec_id], analysis)
      return 'ordered'
    }
    return 'single'
  }

  const ok = await rsConfirm.warning({
    title: t('workspaces.buildNowUnmatchedTitle'),
    description: t('workspaces.buildNowUnmatchedDesc', {
      name: project.project_name,
      list: joinPreview(unmatched),
    }),
    confirmText: t('workspaces.buildNowUnmatchedConfirm'),
    cancelText: t('common.cancel'),
  })
  return ok ? 'single' : 'abort'
}
