<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { damp, reduceMotion, utils } from '@/utils/anime'

/**
 * Reading progress for the article body.
 *
 * Unlike a document-level scroll bar this tracks the *article*: 0 while the
 * body is still below the header line, 1 once its last line has reached the
 * bottom of the viewport. Everything above (title, meta) and below (author
 * card, prev/next) is chrome the reader is not "reading", so counting it would
 * make the bar report progress that has not happened yet.
 */

/**
 * How hard the painted value chases the measured one, for anime.js's frame-rate
 * independent damped lerp. ~0.22 settles in about 200ms: it follows the finger
 * without trailing a fast flick.
 */
const FOLLOW = 0.22

/** Closer than this to either end the bar is decoration without information. */
const EDGE = 0.004

const props = defineProps<{
  /**
   * Resolves the article body element whose progress the bar reports.
   *
   * A getter rather than an element on purpose: the parent's template ref is
   * filled in the same patch that mounts the article, so an element handed over
   * as a prop is still `null` when this component receives it — and assigning a
   * template ref never re-renders the parent, so it would stay `null` forever.
   * Reading it lazily also survives client-side navigation to the next post.
   */
  resolveTarget: () => HTMLElement | null
}>()

/** What the DOM shows, and what the last measurement asked for. */
let painted = 0
let goal = 0
/** Pending frame, 0 when the value has settled — there is no idle rAF loop. */
let frame = 0
let lastTick = 0
let active = false
let snap = false

const bar = ref<HTMLElement | null>(null)

/**
 * The line a heading has to reach to count as "read": the bottom edge of the
 * sticky header plus a small gap. Read from the tokens so the bar, the TOC and
 * the browser's own hash jumps cannot drift apart when the header is resized.
 */
function readingLine(): number {
  const raw = getComputedStyle(document.documentElement).getPropertyValue('--header-h')
  const header = Number.parseFloat(raw)
  return (Number.isFinite(header) ? header : 64) + 16
}

/** Re-measure the article against the viewport, then let the loop smooth it. */
function measure() {
  const target = props.resolveTarget()
  if (!target) {
    goal = 0
    active = false
    kick()
    return
  }

  const line = readingLine()
  const rect = target.getBoundingClientRect()
  // Travel = how far the body scrolls between "top just past the line" and
  // "bottom just left the viewport". A body shorter than the viewport has no
  // travel at all: it is either fully visible (1) or still below the line (0).
  const travel = rect.height + line - window.innerHeight
  const value = travel > 0 ? (line - rect.top) / travel : line >= rect.top ? 1 : 0

  goal = Math.min(1, Math.max(0, value))
  // A bar parked at either end carries no information, so it fades out instead
  // of leaving an empty (or permanently full) track lit for the whole page.
  active = goal > EDGE && goal < 1 - EDGE
  retarget()
  kick()
}

function paint() {
  const element = bar.value
  if (!element) return
  // scaleX (compositor only) instead of width (layout on every frame), written
  // through the toolkit so it shares anime.js's transform bookkeeping.
  utils.set(element, { scaleX: painted, opacity: active ? 1 : 0 })
}

function tick(now: number) {
  frame = 0
  // Clamp the step: a tab that was backgrounded for a second must not make the
  // bar lurch across the page on the frame that wakes it up.
  const delta = Math.min(64, now - lastTick)
  lastTick = now

  // Reduced motion still gets the readout, just without the easing.
  painted = snap ? goal : damp(painted, goal, delta, FOLLOW)
  if (Math.abs(goal - painted) < 0.0008) painted = goal
  paint()
  if (painted !== goal) kick()
}

/** Run the loop only while there is something to smooth. */
function kick() {
  if (frame !== 0) return
  lastTick = performance.now()
  frame = window.requestAnimationFrame(tick)
}

let observer: ResizeObserver | null = null
let watched: HTMLElement | null = null

/**
 * Watch the article's own box: images and webfonts change its height with no
 * scroll event to hear it. The document is watched too, because the article
 * element is replaced on every navigation and that swap is what tells us to
 * point the observer at the new one.
 */
function retarget() {
  if (!observer) return
  const target = props.resolveTarget()
  if (target === watched) return
  if (watched) observer.unobserve(watched)
  watched = target
  if (target) observer.observe(target)
}

onMounted(() => {
  snap = reduceMotion()
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => measure())
    observer.observe(document.body)
  }
  window.addEventListener('scroll', measure, { passive: true })
  window.addEventListener('resize', measure, { passive: true })
  measure()
})

onBeforeUnmount(() => {
  if (frame !== 0) window.cancelAnimationFrame(frame)
  frame = 0
  window.removeEventListener('scroll', measure)
  window.removeEventListener('resize', measure)
  observer?.disconnect()
  observer = null
  watched = null
})
</script>

<template>
  <!-- Pure decoration: the reading position is already conveyed by the page's
       own scrollbar, so this stays out of the a11y tree and the tab order. -->
  <div class="progress" aria-hidden="true">
    <div class="progress__track">
      <span ref="bar" class="progress__bar" />
    </div>
  </div>
</template>

<style scoped>
.progress {
  position: fixed;
  right: 0;
  left: 0;
  /* On the header's bottom edge, and under it (and under any drawer/popup that
     opens at --z-overlay) so nothing else's layering has to change. */
  top: var(--header-h);
  z-index: calc(var(--z-sticky) - 1);
  pointer-events: none;
  /* `ch` resolves against this element's own font, so matching .prose's size is
     what makes the track exactly as wide as the article column. */
  font-size: var(--text-lg);
}

.progress__track {
  max-width: var(--measure);
  margin: 0 auto;
  padding: 0 var(--space-lg);
}

.progress__bar {
  display: block;
  height: 2px;
  border-radius: var(--radius-full);
  background: var(--accent);
  /* The same halo the rest of the site gives accent surfaces. */
  box-shadow: var(--glow-accent);
  transform: scaleX(0);
  transform-origin: left center;
  opacity: 0;
  transition: opacity var(--dur-normal) var(--ease-out);
  will-change: transform, opacity;
}
</style>
