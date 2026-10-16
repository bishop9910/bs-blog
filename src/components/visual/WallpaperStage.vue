<script setup lang="ts">
/**
 * Wallpaper stage.
 *
 * Layers, each owning exactly one transform, so they can animate independently
 * without fighting over `transform`:
 *
 *   .wallpaper__scroll  pointer + scroll parallax (driven from JS)
 *     .wallpaper__drift Ken Burns drift (CSS animation, runs forever)
 *       img             bottom layer (CSS reveal) + incoming layer (anime.js)
 *
 * The image is picked once per page load by useWallpaper; the entrance reveal is
 * CSS to keep the very first paint dependency-free, while every later change is
 * a cross-fade the stage owns (see `promoteOverlay`).
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { motion, utils } from '@/utils/anime'
import { useWallpaper } from '@/composables/useWallpaper'
import { mediaUrl } from '@/api/music'

const { current, pool, hasWallpaper, rotate } = useWallpaper()

const stage = ref<HTMLElement | null>(null)
const overlayImage = ref<HTMLImageElement | null>(null)

/** The path painted by the bottom layer; the entrance reveal is triggered here. */
const shown = ref('')
/** The path currently fading in above it; empty when the stage is idle. */
const overlay = ref('')
const baseReady = ref(false)
const broken = ref(false)

const baseUrl = computed(() => mediaUrl(shown.value))
const overlayUrl = computed(() => mediaUrl(overlay.value))

/** One rotation per interval, and only while the tab is actually being watched. */
const ROTATE_MS = 45_000
const FADE_MS = 1400
/** Long enough for a slow-but-real download, short enough to ever recover. */
const OVERLAY_WATCHDOG_MS = 15_000

/** A wallpaper that fails to load is skipped, but only a pool's worth of times. */
let failures = 0

const reduced =
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

/**
 * Swap the visible image. The first image of a page load has nothing to fade
 * over, so it is assigned straight to the bottom layer and revealed by CSS;
 * later ones arrive through the cross-fade below.
 */
watch(
  current,
  (next) => {
    if (!next) return
    broken.value = false
    if (!shown.value) {
      shown.value = next
      return
    }
    startCrossfade(next)
  },
  { immediate: true },
)

watch(
  () => pool.value.join('|'),
  () => {
    failures = 0
    broken.value = false
    startRotation()
  },
)

/* -------------------------------------------------------------------------- */
/* Rotation                                                                   */
/* -------------------------------------------------------------------------- */

let timer = 0
let fade: ReturnType<typeof motion> = null
/** True while the bottom layer is taking over from a finished cross-fade. */
let pendingLift = false
/** Watchdog for a request that neither loads nor errors. */
let overlayGuard = 0

function canRotate(): boolean {
  return (
    !broken.value &&
    pool.value.length > 1 &&
    !reduced &&
    document.visibilityState === 'visible'
  )
}

function startRotation(): void {
  stopRotation()
  if (!canRotate()) return
  timer = window.setInterval(() => {
    // A cross-fade that has not finished owns the stage; the next tick waits.
    if (!canRotate() || overlay.value) return
    rotate()
  }, ROTATE_MS)
}

function stopRotation(): void {
  if (timer) window.clearInterval(timer)
  timer = 0
}

function onVisibilityChange(): void {
  if (document.visibilityState === 'visible') startRotation()
  else stopRotation()
}

/**
 * Queue the incoming image. Nothing is faded in before it has loaded, otherwise
 * the visitor would watch a broken frame slide over a good one.
 */
function startCrossfade(path: string): void {
  if (path === shown.value || overlay.value) return
  overlay.value = path
  // A request that neither loads nor errors would otherwise hold the stage for
  // good (the layer is invisible, so nobody would notice) and block every later
  // rotation. The watchdog only re-arms the stage; it never interrupts a fade.
  clearOverlayGuard()
  overlayGuard = window.setTimeout(() => {
    const stalled = overlay.value
    if (!stalled || fade || pendingLift) return
    dropOverlay()
    // Retry only when the stage has moved on: asking for the very path that just
    // stalled would reproduce the stall instead of recovering from it.
    if (current.value && current.value !== shown.value && current.value !== stalled) {
      startCrossfade(current.value)
    }
  }, OVERLAY_WATCHDOG_MS)
}

function clearOverlayGuard(): void {
  if (overlayGuard) window.clearTimeout(overlayGuard)
  overlayGuard = 0
}

/** Discard whatever the incoming layer was holding, without touching the base. */
function dropOverlay(): void {
  clearOverlayGuard()
  overlay.value = ''
  fade = null
  pendingLift = false
}

function onBaseLoad(): void {
  baseReady.value = true
  // A promoted image reaches the bottom layer after its own reveal already ran:
  // its load is the signal that the cover above can go.
  if (pendingLift) liftOverlay()
}

/**
 * Cross-fade the incoming layer over the one already on screen: opacity 0→1,
 * blur 10px→0, scale 1.06→1. The outgoing image stays put underneath, so the
 * stage never shows a gap. The blur is animated as a custom property because CSS
 * keeps ownership of `filter` (and a blur radius cannot be tweened as one string).
 */
function playCrossfade(): void {
  const img = overlayImage.value
  if (!img) {
    promoteOverlay()
    return
  }
  fade = motion(img, {
    opacity: [0, 1],
    scale: [1.06, 1],
    '--wp-blur': ['10px', '0px'],
    duration: FADE_MS,
    ease: 'outExpo',
    onComplete: promoteOverlay,
  })
  // Unreachable while `reduced` gates rotation, but the stage must not depend on
  // an animation that was refused.
  if (!fade) promoteOverlay()
}

/**
 * Promote the finished layer: the bottom image takes the new `src` while it is
 * still covered, and the cover is lifted only after the browser has had time to
 * paint it — which is why the visible image is never the one being swapped.
 */
function promoteOverlay(): void {
  const done = overlay.value
  fade = null
  // The fade settled, so the stall watchdog has nothing left to guard.
  clearOverlayGuard()
  if (!done || pendingLift) return
  pendingLift = true
  // The layer has to stay opaque for the two frames between the end of the fade
  // and the lift below, so its end state is written rather than left to the
  // animation's own cleanup.
  const cover = overlayImage.value
  if (cover) utils.set(cover, { opacity: 1, scale: 1, '--wp-blur': '0px' })
  shown.value = done
  requestAnimationFrame(() => requestAnimationFrame(liftOverlay))
}

function liftOverlay(): void {
  if (!pendingLift) return
  // The cover leaves together with the request that produced it.
  dropOverlay()
  // Self-heal: another rotation may have been asked for while this one faded.
  if (current.value && current.value !== shown.value) startCrossfade(current.value)
}

function onBaseError(): void {
  failures += 1
  // Rotate to another candidate rather than showing a broken image; when the
  // whole pool is unusable, fall back to the plain site background.
  if (failures <= pool.value.length) rotate()
  else broken.value = true
}

function onOverlayError(): void {
  failures += 1
  // Drop the unusable candidate. The bottom layer is untouched, so the stage
  // stays on the wallpaper it already has while the next one is fetched.
  overlay.value = ''
  fade = null
  pendingLift = false
  if (failures <= pool.value.length) rotate()
  else broken.value = true
}

/* -------------------------------------------------------------------------- */
/* Parallax                                                                   */
/* -------------------------------------------------------------------------- */

let targetX = 0
let targetY = 0
let valueX = 0
let valueY = 0
let frame = 0

function paint(): void {
  const el = stage.value
  if (!el) return
  el.style.setProperty('--wp-x', `${valueX.toFixed(2)}px`)
  el.style.setProperty('--wp-y', `${valueY.toFixed(2)}px`)
  el.style.setProperty('--wp-scroll', `${(-window.scrollY * 0.06).toFixed(2)}px`)
}

function tick(): void {
  frame = 0
  const dx = targetX - valueX
  const dy = targetY - valueY

  // Ease toward the pointer instead of snapping to it: the lag is what makes
  // the layer read as depth rather than as a cursor attachment.
  valueX += dx * 0.08
  valueY += dy * 0.08
  paint()

  if (Math.abs(dx) > 0.2 || Math.abs(dy) > 0.2) schedule()
}

function schedule(): void {
  if (!frame) frame = requestAnimationFrame(tick)
}

function onPointerMove(event: PointerEvent): void {
  targetX = (event.clientX / window.innerWidth - 0.5) * -24
  targetY = (event.clientY / window.innerHeight - 0.5) * -16
  schedule()
}

function onScroll(): void {
  schedule()
}

onMounted(() => {
  if (reduced) return
  window.addEventListener('pointermove', onPointerMove, { passive: true })
  window.addEventListener('scroll', onScroll, { passive: true })
  // Rotating a background nobody is looking at is pure battery, and a timer
  // that survives a hidden tab fires in a burst on return.
  document.addEventListener('visibilitychange', onVisibilityChange)
  startRotation()
})

onBeforeUnmount(() => {
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('scroll', onScroll)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  stopRotation()
  clearOverlayGuard()
  fade?.cancel()
  fade = null
  if (frame) cancelAnimationFrame(frame)
  frame = 0
})
</script>

<template>
  <div v-if="hasWallpaper && !broken" ref="stage" class="wallpaper" aria-hidden="true">
    <div class="wallpaper__scroll">
      <div class="wallpaper__drift">
        <img
          v-if="shown"
          class="wallpaper__image"
          :class="{ 'is-ready': baseReady }"
          :src="baseUrl"
          alt=""
          decoding="async"
          fetchpriority="high"
          draggable="false"
          @load="onBaseLoad"
          @error="onBaseError"
        />
        <img
          v-if="overlay"
          ref="overlayImage"
          class="wallpaper__image wallpaper__image--incoming"
          :src="overlayUrl"
          alt=""
          decoding="async"
          fetchpriority="low"
          draggable="false"
          @load="playCrossfade"
          @error="onOverlayError"
        />
      </div>
    </div>

    <span class="wallpaper__sweep" />
    <span class="wallpaper__scrim" />
  </div>
</template>

<style scoped>
.wallpaper {
  position: fixed;
  inset: 0;
  /* Below the shader backdrop (-1) and the page content (0), above the canvas
     background that the propagated body colour paints. */
  z-index: -2;
  overflow: hidden;
  pointer-events: none;
  contain: strict;
}

.wallpaper__scroll {
  position: absolute;
  /* Overscan: the parallax shift must never expose an edge. */
  inset: -4%;
  transform: translate3d(var(--wp-x, 0px), calc(var(--wp-y, 0px) + var(--wp-scroll, 0px)), 0);
  will-change: transform;
}

.wallpaper__drift {
  position: absolute;
  inset: 0;
  animation: wallpaper-drift 38s var(--ease-in-out, ease-in-out) infinite alternate;
  transform-origin: 50% 45%;
}

.wallpaper__image {
  /* Absolute, so the bottom and incoming layers occupy the same box and stack in
     DOM order — the fading layer is always the one on top. */
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0;
}

.wallpaper__image.is-ready {
  animation: wallpaper-reveal 1300ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
}

/* The incoming layer is hidden until anime.js cross-fades it in. It only ever
   exists while a rotation is running (JS adds it), so a page without JS, or with
   reduced motion, never renders an invisible image. `filter` stays in CSS while
   the blur radius is animated as a custom property. */
.wallpaper__image--incoming {
  opacity: 0;
  filter: blur(var(--wp-blur, 0px));
}

@keyframes wallpaper-reveal {
  from {
    opacity: 0;
    transform: scale(1.12);
    filter: blur(18px) saturate(1.15);
  }
  to {
    opacity: 1;
    transform: scale(1);
    filter: blur(0) saturate(1);
  }
}

@keyframes wallpaper-drift {
  from {
    transform: scale(1.05) translate3d(0, 0, 0);
  }
  to {
    transform: scale(1.13) translate3d(-1.1%, -0.9%, 0);
  }
}

/* A slow diagonal light sweep: the same trick a camera pass adds, and the only
   motion on the layer once the reveal has finished. */
.wallpaper__sweep {
  position: absolute;
  inset: 0;
  overflow: hidden;
}

.wallpaper__sweep::before {
  content: '';
  position: absolute;
  top: -25%;
  left: -55%;
  width: 38%;
  height: 150%;
  background: linear-gradient(105deg, transparent, rgb(255 255 255 / 14%), transparent);
  transform: rotate(9deg);
  animation: wallpaper-sweep 11s ease-in-out 1.4s infinite;
}

@keyframes wallpaper-sweep {
  0% {
    transform: translate3d(0, 0, 0) rotate(9deg);
    opacity: 0;
  }
  10% {
    opacity: 1;
  }
  50% {
    opacity: 0;
  }
  100% {
    transform: translate3d(430%, 0, 0) rotate(9deg);
    opacity: 0;
  }
}

/* Contrast guard. Derived from the theme's own base colour, so light mode
   lightens the image and dark mode deepens it without a second palette. */
.wallpaper__scrim {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(
      130% 90% at 50% 0%,
      transparent 38%,
      color-mix(in srgb, var(--bg-base) 46%, transparent) 100%
    ),
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--bg-base) 62%, transparent) 0%,
      color-mix(in srgb, var(--bg-base) 28%, transparent) 34%,
      color-mix(in srgb, var(--bg-base) 70%, transparent) 100%
    );
}

@media (prefers-reduced-motion: reduce) {
  .wallpaper__drift,
  .wallpaper__image.is-ready,
  .wallpaper__sweep::before {
    animation: none;
  }

  .wallpaper__image.is-ready,
  .wallpaper__image--incoming {
    opacity: 1;
  }

  .wallpaper__image--incoming {
    filter: none;
  }
}
</style>
