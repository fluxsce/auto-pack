import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useMessage } from '@/components/feedback/message'
import {
  createEnvSetting,
  deleteEnvSetting,
  fetchEnvSettings,
  fetchEnvTypes,
  probeEnvSetting,
  updateEnvSetting,
  verifyEnvSetting,
} from '@/request/api/env'
import type { EnvProbeResult, EnvSetting, EnvSettingPayload, EnvType } from '@/types'
import { DEFAULT_ENV_CONFIG, FALLBACK_ENV_TYPES } from './env-defaults'
import { envTypeLabel, normalizeEnvTypes } from './env-meta'

export type EnvPanelMode = 'none' | 'create' | 'edit'

export function useEnvSettings() {
  const { t } = useI18n()
  const route = useRoute()
  const message = useMessage()

  const envTypes = ref<EnvType[]>([])
  const activeType = ref<string>('all')
  const items = ref<EnvSetting[]>([])
  const loading = ref(false)
  const panelMode = ref<EnvPanelMode>('none')
  const saving = ref(false)
  const probing = ref(false)
  const verifyingId = ref<string | null>(null)
  const editingId = ref<string | null>(null)
  const probeResult = ref<EnvProbeResult | null>(null)

  const form = reactive<EnvSettingPayload>({
    env_name: '',
    env_type: 'ssh',
    config_json: '{}',
    is_default: 0,
    is_enabled: 1,
    sort_num: 0,
    remark: '',
  })

  const configObj = ref<Record<string, unknown>>({})

  const filteredItems = computed(() => {
    if (activeType.value === 'all') return items.value
    return items.value.filter((i) => i.env_type === activeType.value)
  })

  const editorOpen = computed(() => panelMode.value !== 'none')

  const editorTitle = computed(() =>
    panelMode.value === 'create' ? t('envCreateTitle') : t('envEditTitle'),
  )

  function typeLabel(type: unknown): string {
    return envTypeLabel(t, type)
  }

  function formatProbeResult(res: EnvProbeResult): EnvProbeResult {
    const lines = [res.message]
    if (res.details) {
      for (const [key, value] of Object.entries(res.details)) {
        if (value) lines.push(`${key}: ${value}`)
      }
    }
    return { ...res, message: lines.filter(Boolean).join('\n') }
  }

  function parseConfig(): void {
    try {
      configObj.value = JSON.parse(form.config_json || '{}') as Record<string, unknown>
    } catch {
      configObj.value = {}
    }
  }

  function syncConfigJson(): void {
    form.config_json = JSON.stringify(configObj.value)
  }

  function resetForm(type: EnvType = 'ssh'): void {
    editingId.value = null
    form.env_name = ''
    form.env_type = type
    form.is_default = 0
    form.is_enabled = 1
    form.sort_num = 0
    form.remark = ''
    configObj.value = structuredClone(DEFAULT_ENV_CONFIG[type])
    syncConfigJson()
    probeResult.value = null
  }

  function openCreate(): void {
    const type = activeType.value === 'all' ? 'ssh' : (activeType.value as EnvType)
    resetForm(type)
    panelMode.value = 'create'
  }

  function openEdit(row: EnvSetting): void {
    editingId.value = row.rec_id
    panelMode.value = 'edit'
    form.env_name = row.env_name
    form.env_type = row.env_type
    form.config_json = row.config_json
    form.is_default = row.is_default
    form.is_enabled = row.is_enabled
    form.sort_num = row.sort_num
    form.remark = row.remark
    parseConfig()
    probeResult.value = null
  }

  function closeEditor(): void {
    panelMode.value = 'none'
    editingId.value = null
    probeResult.value = null
  }

  function onTypeChange(type: EnvType): void {
    form.env_type = type
    configObj.value = structuredClone(DEFAULT_ENV_CONFIG[type])
    syncConfigJson()
    probeResult.value = null
  }

  async function loadList(): Promise<void> {
    loading.value = true
    try {
      const res = await fetchEnvSettings()
      items.value = res.items
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      loading.value = false
    }
  }

  async function onSave(): Promise<void> {
    syncConfigJson()
    if (!form.env_name.trim()) {
      message.warning(t('envNameRequired'))
      return
    }
    saving.value = true
    try {
      if (editingId.value) {
        await updateEnvSetting(editingId.value, { ...form })
        message.success(t('envSaveSuccess'))
      } else {
        const created = await createEnvSetting({ ...form })
        editingId.value = created.rec_id
        panelMode.value = 'edit'
        message.success(t('envCreateSuccess'))
      }
      probeResult.value = null
      await loadList()
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      saving.value = false
    }
  }

  async function onDelete(row: EnvSetting): Promise<void> {
    try {
      await deleteEnvSetting(row.rec_id)
      message.success(t('envDeleteSuccess'))
      if (editingId.value === row.rec_id) closeEditor()
      await loadList()
    } catch (e) {
      message.error((e as Error).message)
    }
  }

  async function onVerifyRow(row: EnvSetting): Promise<void> {
    verifyingId.value = row.rec_id
    try {
      const res = await verifyEnvSetting(row.rec_id)
      const formatted = formatProbeResult(res)
      if (formatted.ok) {
        message.success(formatted.message || t('envVerifyOk'))
      } else {
        message.error(formatted.message || t('envVerifyFail'))
      }
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      verifyingId.value = null
    }
  }

  async function onProbeDraft(): Promise<void> {
    syncConfigJson()
    probing.value = true
    probeResult.value = null
    try {
      const res = await probeEnvSetting({
        env_type: form.env_type,
        config_json: form.config_json,
        merge_from_rec_id: editingId.value ?? undefined,
      })
      const formatted = formatProbeResult(res)
      probeResult.value = formatted
      if (formatted.ok) {
        message.success(t('envVerifyOk'))
      } else {
        message.warning(t('envVerifyFail'))
      }
    } catch (e) {
      message.error((e as Error).message)
    } finally {
      probing.value = false
    }
  }

  onMounted(async () => {
    try {
      const res = await fetchEnvTypes()
      envTypes.value = normalizeEnvTypes(res.types)
    } catch {
      envTypes.value = [...FALLBACK_ENV_TYPES]
    }
    await loadList()

    const qType = route.query.type
    if (qType === 'git') {
      activeType.value = 'git'
      if (route.query.create === '1') {
        resetForm('git')
        panelMode.value = 'create'
      }
    }
  })

  return {
    t,
    envTypes,
    activeType,
    items,
    loading,
    panelMode,
    editorOpen,
    saving,
    probing,
    verifyingId,
    editingId,
    probeResult,
    form,
    configObj,
    filteredItems,
    editorTitle,
    typeLabel,
    openCreate,
    openEdit,
    closeEditor,
    onTypeChange,
    onSave,
    onDelete,
    onVerifyRow,
    onProbeDraft,
  }
}
