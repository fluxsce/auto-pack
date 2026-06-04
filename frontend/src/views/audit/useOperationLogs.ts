import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import {
  fetchOperationLogActions,
  fetchOperationLogModules,
  fetchOperationLogs,
} from '@/request/api/operation-log'
import type { OperationLogEntry } from '@/types'

const PAGE_SIZE = 50

export function useOperationLogs() {
  const { t } = useI18n()
  const message = useMessage()

  const items = ref<OperationLogEntry[]>([])
  const total = ref(0)
  const loading = ref(false)
  const modules = ref<string[]>([])
  const actions = ref<string[]>([])
  const page = ref(1)

  const filters = reactive({
    module: '',
    action: '',
    keyword: '',
  })

  const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))

  function moduleLabel(module: string): string {
    const key = `opLogModule_${module}`
    const text = t(key)
    return text === key ? module : text
  }

  function actionLabel(action: string): string {
    const key = `opLogAction_${action}`
    const text = t(key)
    return text === key ? action : text
  }

  async function loadMeta(): Promise<void> {
    try {
      const [m, a] = await Promise.all([fetchOperationLogModules(), fetchOperationLogActions()])
      modules.value = m.items ?? []
      actions.value = a.items ?? []
    } catch {
      modules.value = []
      actions.value = []
    }
  }

  async function loadList(): Promise<void> {
    loading.value = true
    try {
      const offset = (page.value - 1) * PAGE_SIZE
      const res = await fetchOperationLogs({
        module: filters.module || undefined,
        action: filters.action || undefined,
        keyword: filters.keyword.trim() || undefined,
        limit: PAGE_SIZE,
        offset,
      })
      items.value = res.items ?? []
      total.value = res.total ?? 0
    } catch (e) {
      message.error((e as Error).message)
      items.value = []
      total.value = 0
    } finally {
      loading.value = false
    }
  }

  function applyFilters(): void {
    page.value = 1
    void loadList()
  }

  function onPageChange(p: number): void {
    page.value = p
    void loadList()
  }

  onMounted(() => {
    void loadMeta()
    void loadList()
  })

  return {
    items,
    total,
    loading,
    filters,
    modules,
    actions,
    page,
    totalPages,
    moduleLabel,
    actionLabel,
    applyFilters,
    onPageChange,
    loadList,
  }
}
