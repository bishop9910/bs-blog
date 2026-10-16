<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AmbientWeather from '@/components/visual/AmbientWeather.vue'
import CursorGlow from '@/components/visual/CursorGlow.vue'
import ShaderBackdrop from '@/components/visual/ShaderBackdrop.vue'
import WallpaperStage from '@/components/visual/WallpaperStage.vue'
import SiteFooter from '@/components/SiteFooter.vue'
import SiteHeader from '@/components/SiteHeader.vue'
import MusicPlayer from '@/components/music/MusicPlayer.vue'
import { useSite } from '@/composables/useSite'
import { useWallpaper } from '@/composables/useWallpaper'
import { startSmoothScroll, stopSmoothScroll } from '@/composables/useSmoothScroll'
import { motion, reduceMotion } from '@/utils/anime'

const { load } = useSite()
// Only used to decide how much of the shader backdrop the wallpaper replaces.
const { hasWallpaper } = useWallpaper()

const route = useRoute()

/** How long a page takes to leave, and to arrive. The gap between them is the beat. */
const LEAVE_MS = 200
const ENTER_MS = 420
/** The streak is slower than the page so it reads as passing behind it. */
const SWEEP_MS = 720

/**
 * Where a page sits in the site's reading order (`order` in the route meta).
 * The header's nav list and the archive/tag pages all declare one, so a page
 * further along the list arrives from the right and one further back from the
 * left — going back to the archive stops looking like going forward to it.
 */
const orderOf = (meta: { order?: number }): number => (typeof meta.order === 'number' ? meta.order : 0)

const direction = ref(1)
let previousOrder = orderOf(route.meta)

watch(
  () => route.path,
  () => {
    const current = orderOf(route.meta)
    direction.value = current < previousOrder ? -1 : 1
    previousOrder = current
  },
)

const sweepEl = ref<HTMLElement | null>(null)

/**
 * A watchdog around Vue's transition `done` callback. A navigation landing
 * mid-transition can cancel the running animation, and `anime.js` then never
 * fires `onComplete` — without this the outgoing page would stay mounted and
 * the router view would be stuck on it for good.
 */
function settleLater(done: () => void, limit: number): () => void {
  let settled = false
  let timer = 0
  const settle = (): void => {
    if (settled) return
    settled = true
    window.clearTimeout(timer)
    done()
  }
  timer = window.setTimeout(settle, limit)
  return settle
}

/**
 * anime.js leaves the values it ended on as inline styles. Any inline
 * `transform` makes that element the containing block for `position: fixed`
 * descendants, and the reading progress bar and the table of contents are
 * both fixed — so the leftovers have to be cleared once a page has arrived.
 */
function releaseInlineStyles(el: HTMLElement): void {
  el.style.removeProperty('transform')
  el.style.removeProperty('opacity')
}

/** One lap of the light streak, in the current direction of travel. */
function playSweep(): void {
  const el = sweepEl.value
  if (!el || reduceMotion()) return
  const travel = window.innerWidth
  const from = direction.value > 0 ? -travel * 0.75 : travel * 1.25
  const to = direction.value > 0 ? travel * 1.25 : -travel * 0.75
  el.classList.add('is-sweeping')
  motion(el, {
    opacity: [0, 0.85, 0],
    translateX: [from, to],
    duration: SWEEP_MS,
    ease: 'inOutQuad',
    onComplete: () => el.classList.remove('is-sweeping'),
  })
}

function onLeave(el: Element, done: () => void): void {
  const settle = settleLater(done, LEAVE_MS + 320)
  const anim = motion(el as HTMLElement, {
    opacity: [1, 0],
    translateX: [0, direction.value > 0 ? -18 : 18],
    translateY: [0, -6],
    scale: [1, 0.994],
    duration: LEAVE_MS,
    ease: 'inQuad',
    onComplete: settle,
  })
  if (!anim) settle()
}

function onEnter(el: Element, done: () => void): void {
  const target = el as HTMLElement
  const settle = settleLater(done, ENTER_MS + 420)
  playSweep()
  const anim = motion(target, {
    opacity: [0, 1],
    translateX: [direction.value > 0 ? 26 : -26, 0],
    translateY: [8, 0],
    scale: [0.996, 1],
    duration: ENTER_MS,
    ease: 'outExpo',
    onComplete: () => {
      releaseInlineStyles(target)
      settle()
    },
  })
  if (!anim) {
    releaseInlineStyles(target)
    settle()
  }
}

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

    <main id="main" class="shell__main">
      <!--
        JS hooks rather than CSS classes: the motion is directional and has a
        leave/enter beat that a single class pair cannot express. The admin
        shell keeps its own plain `route-fade` CSS transition.
      -->
      <RouterView v-slot="{ Component, route: current }">
        <Transition :css="false" mode="out-in" @leave="onLeave" @enter="onEnter">
          <component :is="Component" :key="current.path" />
        </Transition>
      </RouterView>
    </main>

    <SiteFooter />

    <span ref="sweepEl" class="route-sweep" aria-hidden="true" />
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
    The page column is what slides during a route change, so a 26px enter
    offset would otherwise flash a horizontal scrollbar on every navigation.
    `clip` rather than `hidden`: it does not turn the column into a scroll
    container, visible-Y stays visible, and the fixed children (reading
    progress, table of contents) escape once the page's inline transform is
    cleared — while it is still there they ride in with the page, which is
    exactly what should happen.
  */
  overflow-x: clip;
  overflow-y: visible;
}

/*
  A light streak that crosses the viewport once per navigation, following the
  direction of travel. It lives outside the transitioning subtree on purpose:
  a page being swapped out must not take the streak with it mid-flight — that
  is what turns a cross-fade into something that feels like a slide.

  Idle it is `visibility: hidden`: it carries a 30px blur, and an
  invisible-but-painted layer that large is pure GPU cost on every frame.
*/
.route-sweep {
  position: fixed;
  top: -25vh;
  bottom: -25vh;
  left: 0;
  width: 44vw;
  z-index: var(--z-overlay);
  pointer-events: none;
  visibility: hidden;
  opacity: 0;
  rotate: -14deg;
  background: linear-gradient(
    90deg,
    transparent 0%,
    color-mix(in srgb, var(--accent) 16%, transparent) 42%,
    color-mix(in srgb, var(--accent) 30%, transparent) 52%,
    transparent 100%
  );
  filter: blur(30px);
}

.route-sweep.is-sweeping {
  visibility: visible;
}

@media (prefers-reduced-motion: reduce) {
  .route-sweep {
    display: none;
  }
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
