import './styles/index.scss'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import App from './App.vue'
import { i18n } from './i18n'
import { router } from './router'
import DialogPlugin from '@/components/feedback/dialog'
import MessagePlugin from '@/components/feedback/message'
import PopconfirmPlugin from '@/components/feedback/popconfirm'
import PopoverPlugin from '@/components/feedback/popover'
import { useSettingsStore } from '@/stores/settings'

const app = createApp(App)
const pinia = createPinia()

app
  .use(pinia)
  .use(i18n)
  .use(router)
  .use(ElementPlus)
  .use(MessagePlugin)
  .use(DialogPlugin)
  .use(PopconfirmPlugin)
  .use(PopoverPlugin)

useSettingsStore().init()

app.mount('#app')
