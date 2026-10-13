<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import CursorGlow from '@/components/visual/CursorGlow.vue'
import ShaderBackdrop from '@/components/visual/ShaderBackdrop.vue'
import SiteFooter from '@/components/SiteFooter.vue'
import SiteHeader from '@/components/SiteHeader.vue'
import { useSite } from '@/composables/useSite'
import { startSmoothScroll, stopSmoothScroll } from '@/composables/useSmoothScroll'

const { load } = useSite()

onMounted(() => {
  // Smooth scrolling and the shader backdrop are public-site affordances only;
  // the admin area stays plain and predictable.
  startSmoothScroll()
  void load()
})

onBeforeUnmount(() => {
  stopSmoothScroll()
})
</script>

<template>
  <div class="shell">
    <a class="skip-link" href="#main">跳到正文</a>

    <ShaderBackdrop class="shell__backdrop" :intensity="0.75" />
    <CursorGlow />

    <SiteHeader />

    <main id="main" class="shell__main">
      <RouterView v-slot="{ Component, route }">
        <Transition name="route-fade" mode="out-in">
          <component :is="Component" :key="route.path" />
        </Transition>
      </RouterView>
    </main>

    <SiteFooter />

    <span class="grain" aria-hidden="true" />
  </div>
</template>

<style scoped>
.shell {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.shell__backdrop {
  position: fixed;
  inset: 0;
  z-index: -1;
  pointer-events: none;
}

.shell__main {
  flex: 1;
  width: 100%;
}
</style>

<!--
  Lenis only needs the class `html.lenis` at runtime; these are the documented
  rules, inlined here so the package's stylesheet path never has to be guessed.
-->
<style>
html.lenis,
html.lenis body {
  height: auto;
}

.lenis.lenis-smooth {
  scroll-behavior: auto !important;
}

.lenis.lenis-smooth [data-lenis-prevent] {
  overscroll-behavior: contain;
}

.lenis.lenis-stopped {
  overflow: hidden;
}

.lenis.lenis-smooth iframe {
  pointer-events: none;
}
</style>
