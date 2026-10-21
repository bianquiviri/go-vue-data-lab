import './assets/main.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import i18n from './i18n'
import router from './router'
import { useLocaleStore } from './stores/locale'
import { useThemeStore } from './stores/theme'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(i18n)
app.use(router)

// Hidrata las preferencias persistidas antes de montar: tema (Claro/Oscuro/
// Sistema) e idioma (ES/EN). Ambas se aplican al <html> sin parpadeo.
useThemeStore(pinia).init()
useLocaleStore(pinia).init()

app.mount('#app')
