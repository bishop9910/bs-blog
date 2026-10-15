import { createApp } from 'vue'
import VueParticles from '@tsparticles/vue3'
import type { IParticlesProviderOptions } from '@tsparticles/vue3'
import { loadSlim } from '@tsparticles/slim'

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

// The particle engine is loaded once for the whole app. This options object —
// and therefore the init callback — must keep a stable identity for the whole
// app lifecycle, or @tsparticles/vue3 throws on re-registration.
const particles: IParticlesProviderOptions = {
  init: async (engine) => {
    await loadSlim(engine)
  },
}

const app = createApp(App)

app.directive('reveal', reveal)
app.use(router)
app.use(VueParticles, particles)
app.mount('#app')
