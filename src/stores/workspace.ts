import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiPath } from '@/api/prefix'
import { request } from '@/api/http'

export type Workspace = {
  rec_id: string
  workspace_code: string
  workspace_name: string
  remark?: string
  member_role?: string
  project_count?: number
  is_default?: number
  root_dir?: string
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const list = ref<Workspace[]>([])
  const currentId = ref('')
  const projectsEpoch = ref(0)

  async function load(): Promise<void> {
    list.value = await request<Workspace[]>({ method: 'GET', url: apiPath('workspaces') })
    const saved = localStorage.getItem('ap_ws')
    if (saved && list.value.some((w) => w.rec_id === saved)) {
      currentId.value = saved
      return
    }
    currentId.value = list.value[0]?.rec_id ?? ''
  }

  function select(id: string): void {
    currentId.value = id
    localStorage.setItem('ap_ws', id)
  }

  /** 通知当前工作空间的工程列表重新拉取（左右栏不共享 composable）。 */
  function refreshProjects(): void {
    projectsEpoch.value += 1
  }

  return { list, currentId, projectsEpoch, load, select, refreshProjects }
})
