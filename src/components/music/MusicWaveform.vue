<script setup lang="ts">
/**
 * Spectrum canvas that sits directly above the progress bar.
 *
 * Two sources feed the same meter: live frequency data from the player's Web
 * Audio graph, and a synthetic field used before the first play — or when the
 * browser refuses an AudioContext — so the bar is never a dead rectangle.
 *
 * The canvas is decorative (the progress bar under it is the seek control), so
 * it is hidden from assistive technology and ignores pointer events.
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { readSpectrum } from '@/composables/useMusic'
import { useTheme } from '@/composables/useTheme'

/** Enough bars to read as a spectrum, few enough to stay crisp at 320px wide. */
const BARS = 44

const canvas = ref<HTMLCanvasElement | null>(null)
const { effective } = useTheme()

let ctx: CanvasRenderingContext2D | null = null
let frame = 0
let observer: ResizeObserver | null = null
let width = 0
let height = 0

const bars = new Array<number>(BARS).fill(0)
let idlePhase = 0
let gradient: CanvasGradient | null = null
// Resolved from the stylesheet in readPalette(); nothing is drawn before then.
let colorFrom = 'transparent'
let colorTo = 'transparent'

const reduced =
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

/* -------------------------------------------------------------------------- */
/* Sizing and palette                                                         */
/* -------------------------------------------------------------------------- */

function measure(): void {
  const el = canvas.value
  if (!el) return

  const rect = el.getBoundingClientRect()
  width = rect.width
  height = rect.height
  if (width === 0 || height === 0) return

  // Cap the device pixel ratio: a 3x phone would quadruple the fill cost for
  // detail nobody can see on bars this thin.
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  el.width = Math.max(1, Math.round(width * dpr))
  el.height = Math.max(1, Math.round(height * dpr))

  ctx = el.getContext('2d')
  ctx?.setTransform(dpr, 0, 0, dpr, 0, 0)
  gradient = null
}

function readPalette(): void {
  const styles = getComputedStyle(document.documentElement)
  // Semantic first, palette second: the second lookup only ever fires if a
  // stylesheet override dropped the semantic token, and it keeps the values in
  // tokens.css instead of duplicating them here.
  colorFrom =
    styles.getPropertyValue('--accent').trim() ||
    styles.getPropertyValue('--p-pink-500').trim() ||
    'transparent'
  colorTo =
    styles.getPropertyValue('--accent-hover').trim() ||
    styles.getPropertyValue('--p-pink-400').trim() ||
    colorFrom
  gradient = null
}

/* -------------------------------------------------------------------------- */
/* Rendering                                                                  */
/* -------------------------------------------------------------------------- */

function rounded(context: CanvasRenderingContext2D, x: number, y: number, w: number, h: number): void {
  const radius = Math.min(2.5, w / 2, h / 2)
  context.beginPath()
  context.moveTo(x + radius, y)
  context.arcTo(x + w, y, x + w, y + h, radius)
  context.arcTo(x + w, y + h, x, y + h, radius)
  context.arcTo(x, y + h, x, y, radius)
  context.arcTo(x, y, x + w, y, radius)
  context.closePath()
}

function targets(): void {
  const spectrum = readSpectrum()
  idlePhase += 0.012

  for (let i = 0; i < BARS; i += 1) {
    let target: number

    if (spectrum && spectrum.length > 4) {
      // Bin 0 is DC and the top of the range is mostly empty for music, so the
      // audible middle is what gets spread across the bars.
      const span = Math.max(1, Math.floor(spectrum.length * 0.62))
      const start = 1 + Math.floor((i / BARS) * span)
      const end = Math.max(start + 1, 1 + Math.floor(((i + 1) / BARS) * span))

      let sum = 0
      let count = 0
      for (let bin = start; bin < end && bin < spectrum.length; bin += 1) {
        sum += spectrum[bin] ?? 0
        count += 1
      }

      const raw = count > 0 ? sum / count / 255 : 0
      // A little compression: raw FFT magnitudes leave the meter looking asleep
      // at comfortable listening volumes.
      target = Math.min(1, raw ** 0.72 * 1.25)
    } else {
      target = 0.09 + 0.06 * Math.sin(idlePhase + i * 0.42) + 0.035 * Math.sin(idlePhase * 1.7 + i * 0.9)
    }

    const previous = bars[i] ?? 0
    // Fast attack, slow release: the familiar meter response, and the reason a
    // beat reads as a beat instead of a blur.
    bars[i] = previous + (target - previous) * (target > previous ? 0.55 : 0.1)
  }
}

function paint(): void {
  if (!ctx || width === 0 || height === 0) return

  ctx.clearRect(0, 0, width, height)

  if (!gradient) {
    gradient = ctx.createLinearGradient(0, 0, width, 0)
    gradient.addColorStop(0, colorFrom)
    gradient.addColorStop(1, colorTo)
  }

  const gap = Math.max(1, (width / BARS) * 0.22)
  const barWidth = (width - gap * (BARS - 1)) / BARS
  ctx.fillStyle = gradient

  for (let i = 0; i < BARS; i += 1) {
    const value = Math.max(0.015, bars[i] ?? 0)
    const barHeight = Math.max(2, value * height * 0.92)
    const x = i * (barWidth + gap)
    rounded(ctx, x, (height - barHeight) / 2, barWidth, barHeight)
    ctx.fill()
  }
}

function loop(): void {
  frame = requestAnimationFrame(loop)
  // A hidden tab keeps its audio but never needs canvas work.
  if (document.hidden) return
  targets()
  paint()
}

/* -------------------------------------------------------------------------- */

onMounted(() => {
  measure()
  readPalette()

  const el = canvas.value
  if (el && typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => {
      measure()
    })
    observer.observe(el)
  }

  if (reduced) {
    // Still paints one static frame so the bar has a shape, but no animation.
    targets()
    paint()
    return
  }

  frame = requestAnimationFrame(loop)
})

onBeforeUnmount(() => {
  if (frame) cancelAnimationFrame(frame)
  frame = 0
  observer?.disconnect()
  observer = null
})

watch(effective, () => {
  readPalette()
})
</script>

<template>
  <canvas ref="canvas" class="waveform" aria-hidden="true" />
</template>

<style scoped>
.waveform {
  display: block;
  width: 100%;
  height: 52px;
  pointer-events: none;
}

@media (max-width: 640px) {
  .waveform {
    height: 40px;
  }
}
</style>
