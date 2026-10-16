<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AmbientWeather from '@/components/visual/AmbientWeather.vue'
import CursorGlow from '@/components/visual/CursorGlow.vue'
import RouteCurtain from '@/components/visual/RouteCurtain.vue'
import ShaderBackdrop from '@/components/visual/ShaderBackdrop.vue'
import WallpaperStage from '@/components/visual/WallpaperStage.vue'
import SiteFooter from '@/components/SiteFooter.vue'
import SiteHeader from '@/components/SiteHeader.vue'
import MusicPlayer from '@/components/music/MusicPlayer.vue'
import { useSite } from '@/composables/useSite'
import { useWallpaper } from '@/composables/useWallpaper'
import { CURTAIN_TIMING, useRouteCurtain } from '@/composables/usePageTransition'
import { startSmoothScroll, stopSmoothScroll } from '@/composables/useSmoothScroll'
import { motion, reduceMotion } from '@/utils/anime'

const { load } = useSite()
// Only used to decide how much of the shader backdrop the wallpaper replaces.
const { hasWallpaper } = useWallpaper()
const { active, phase, direction } = useRouteCurtain()

const mainEl = ref<HTMLElement | null>(null)

/**
 * anime.js leaves the values it ended on as inline styles. Any inline
 * `transform` makes that element the containing block for `position: fixed`
 * descendants, and the reading progress bar and the table of contents are both
 * fixed — so the leftovers have to be cleared once a page has come to rest.
 */
function releaseInlineStyles(el: HTMLElement): void {
  el.style.removeProperty('transform')
  el.style.removeProperty('opacity')
}

/**
 * The blade is on its way out, so the page it just uncovered travels the rest
 * of the way in behind it — in the same direction, which keeps a page reached
 * by going forward and one reached by going back from arriving identically.
 *
 * There is deliberately no `<Transition>` on the router view any more: the swap
 * itself now happens under the blade, and that is what keeps the new page's
 * first paint (and its data, if it has to wait for any) out of sight.
 */
watch(phase, (next) => {
  if (next !== 'revealing') return
  const target = mainEl.value?.firstElementChild
  if (!(target instanceof HTMLElement) || reduceMotion()) return
  motion(target, {
    opacity: [0.45, 1],
    translateX: [direction.value > 0 ? 42 : -42, 0],
    translateY: [12, 0],
    duration: CURTAIN_TIMING.reveal + 60,
    ease: 'outExpo',
    onComplete: () => releaseInlineStyles(target),
  })
})

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
  <div class="shell" :class="{ 'shell--wallpapered': hasWallpaper }">
    <a class="skip-link" href="#main">跳到正文</a>

    <WallpaperStage />
    <ShaderBackdrop class="shell__backdrop" :intensity="0.75" />
    <AmbientWeather />
    <CursorGlow />

    <SiteHeader />
    <MusicPlayer />

    <main id="main" ref="mainEl" class="shell__main">
      <!--
        No <Transition> here: the page is swapped while the curtain has the
        screen, so the new page's first paint never happens in view.
      -->
      <RouterView v-slot="{ Component, route: current }">
        <component :is="Component" :key="current.path" />
      </RouterView>
    </main>

    <SiteFooter />

    <RouteCurtain v-if="active" />
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
  transition: opacity var(--dur-slower) var(--ease-out);
}

/* The shader canvas is opaque, so a wallpaper underneath is only visible if the
   backdrop itself becomes translucent. On dark it blends as an aurora wash; on
   light that would wash the image out, so it simply fades. */
.shell--wallpapered .shell__backdrop {
  opacity: 0.42;
  mix-blend-mode: screen;
}

html[data-theme='light'] .shell--wallpapered .shell__backdrop {
  opacity: 0.24;
  mix-blend-mode: normal;
}

.shell__main {
  flex: 1;
  width: 100%;
  /*
    The page column is what slides during a route change, so a 42px enter offset
    would otherwise flash a horizontal scrollbar on every navigation. `clip`
    rather than `hidden`: it does not turn the column into a scroll container,
    visible-Y stays visible, and the fixed children (reading progress, table of
    contents) escape once the page's inline transform is cleared — while it is
    still there they ride in with the page, which is exactly what should happen.
  */
  overflow-x: clip;
  overflow-y: visible;
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
