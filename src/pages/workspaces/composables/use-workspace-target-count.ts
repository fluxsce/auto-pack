import { ref } from 'vue'
import { listProjects, type Project } from '../api'

export function useWorkspaceTargetCount(match: (row: Project) => boolean) {
  const loading = ref(false)
  const count = ref(0)

  async function load(workspaceId: string): Promise<number> {
    if (!workspaceId) {
      count.value = 0
      return 0
    }
    loading.value = true
    try {
      const rows = await listProjects(workspaceId)
      count.value = rows.filter(match).length
      return count.value
    } catch (e) {
      count.value = 0
      throw e
    } finally {
      loading.value = false
    }
  }

  return { loading, count, load }
}
