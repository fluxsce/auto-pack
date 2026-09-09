import { watch } from 'vue'
import { createI18n } from 'vue-i18n'
import type { AppLocale } from '@/prefs/appearance'
import { locale } from '@/prefs/appearance'
import enUS from './en-US'
import zhCN from './zh-CN'

export const i18n = createI18n({
  legacy: false,
  locale: locale.value,
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

function applyDocumentTitle(): void {
  if (typeof document === 'undefined') {
    return
  }
  document.title = String(i18n.global.t('brand.appTitle'))
}

watch(locale, (next: AppLocale) => {
  i18n.global.locale.value = next
  applyDocumentTitle()
})

applyDocumentTitle()
