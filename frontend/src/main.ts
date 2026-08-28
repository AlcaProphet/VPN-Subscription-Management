import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import AppSelect from './components/AppSelect.vue'
import './style.css'

const app = createApp(App)
app.component('AppSelect', AppSelect)
app.use(createPinia())
app.use(router)
app.mount('#app')
