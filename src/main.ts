import '@/prefs/appearance'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import '@niuma/ui/styles.css'
import './styles/app.css'
import App from './App.vue'
import { i18n } from './i18n'
import { router, bindUnauthorizedRedirect } from './router'

const app = createApp(App)
app.use(createPinia())
app.use(i18n)
app.use(router)
bindUnauthorizedRedirect()
app.mount('#app')
