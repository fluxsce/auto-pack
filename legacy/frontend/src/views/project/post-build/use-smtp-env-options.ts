import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EnvSetting } from '@/types'
import { fetchEnvSettings } from '@/request/api/env'

function asEnvList(raw: EnvSetting[] | null | undefined): EnvSetting[] {
  return Array.isArray(raw) ? raw : []
}

/** Loads SMTP environments for post-build email (includes disabled entries). */
export function useSmtpEnvOptions(fallback?: (envType: string) => EnvSetting[]) {
  const { t } = useI18n()
  const smtpEnvs = ref<EnvSetting[]>([])
  const loading = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    try {
      const res = await fetchEnvSettings('smtp')
      smtpEnvs.value = asEnvList(res.items)
    } catch {
      smtpEnvs.value = []
    } finally {
      loading.value = false
    }
  }

  const options = computed(() => {
    if (smtpEnvs.value.length > 0) return smtpEnvs.value
    return fallback?.('smtp') ?? []
  })

  const hasOptions = computed(() => options.value.length > 0)

  function optionLabel(item: EnvSetting): string {
    if (item.is_enabled === 1) return item.env_name
    return `${item.env_name} (${t('disabled')})`
  }

  return { smtpEnvs, loading, options, hasOptions, load, optionLabel }
}
