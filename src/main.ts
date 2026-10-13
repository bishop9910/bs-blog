import { createApp } from 'vue'

import App from './App.vue'
import router from './router'
import { reveal } from './directives/reveal'
import { initTheme } from './composables/useTheme'

// Token layer first — base and motion both depend on it.
import './styles/tokens.css'
import './styles/base.css'
import './styles/motion.css'
import './styles/highlight.css'
import './styles/components.css'

// Paint the correct theme before the first frame to avoid a flash.
initTheme()

const app = createApp(App)

app.directive('reveal', reveal)
app.use(router)
app.mount('#app')
