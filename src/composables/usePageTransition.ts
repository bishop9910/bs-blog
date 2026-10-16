import { computed, nextTick, ref } from 'vue'
import type { RouteLocationNormalized, RouteMeta } from 'vue-router'
import { prefersReducedMotion } from '@/utils/reduced-motion'

/**
 * The route curtain.
 *
 * Why this is a navigation guard and not a `<Transition>`: transition hooks run
 * *after* the lazy chunk has been fetched, the component mounted and its data
 * requested — by the time they fire, the stall an animation was supposed to
 * cover has already been seen. `router.beforeEach` runs at click time, so the
 * blade closes before any of that work starts, and the work then happens behind
 * an opaque panel instead of in full view.
 *
 * The second half of the job is knowing when to let go again. Waiting for the
 * paint covers mount and layout; waiting for the page's own data covers the
 * difference between "the DOM is there" and "there is something to read".
 */

export type CurtainPhase = 'idle' | 'covering' | 'holding' | 'revealing'

/** Blade sweeping in, and sweeping away again. */
const COVER_MS = 240
const REVEAL_MS = 380
/** Time spent fully covered even when the page is ready instantly. */
const HOLD_MIN_MS = 110
/** Hard ceiling: a page that never reports back cannot hold the site hostage. */
const HOLD_MAX_MS = 1200
/** Only after this does the curtain admit it is waiting. */
const SLOW_MS = 520

/** Exposed so the layout can pace the page's own arrival against the blade. */
export const CURTAIN_TIMING = {
  cover: COVER_MS,
  reveal: REVEAL_MS,
  hold: HOLD_MIN_MS,
  max: HOLD_MAX_MS,
} as const

const phase = ref<CurtainPhase>('idle')
const direction = ref(1)
const waiting = ref(false)

/** When the blade finished closing — the minimum hold is measured from here. */
let coveredAt = 0
/** The page behind the blade has been laid out at least once. */
let painted = false
/** The page asked to be waited for. */
let obligated = false
/** …and has since reported that its content is there. */
let ready = false

let slowTimer = 0
let holdTimer = 0
let maxTimer = 0
let coverResolve: (() => void) | null = null
/** Bumped per covered navigation, so a load can tell whose navigation it belongs to. */
let generation = 0

/** Where a route sits in the site's reading order, for the direction of travel. */
const orderOf = (meta: RouteMeta): number => (typeof meta.order === 'number' ? meta.order : 0)

function clearTimers(): void {
  window.clearTimeout(slowTimer)
  window.clearTimeout(holdTimer)
  window.clearTimeout(maxTimer)
  slowTimer = 0
  holdTimer = 0
  maxTimer = 0
}

/** Lets the guard that is awaiting the blade continue. */
function releaseCover(): void {
  const resolve = coverResolve
  coverResolve = null
  resolve?.()
}

/**
 * Everything except the navigations there is nothing to cover for: the first
 * load (which has no page to leave), the admin area (which keeps its own plain
 * fade), and navigations that only move an anchor within a page.
 */
function coveredByCurtain(to: RouteLocationNormalized, from: RouteLocationNormalized): boolean {
  if (prefersReducedMotion()) return false
  if (from.name === undefined) return false
  if (to.path.startsWith('/admin') || from.path.startsWith('/admin')) return false
  if (to.path === from.path && JSON.stringify(to.query) === JSON.stringify(from.query)) return false
  return true
}

/**
 * Called from `router.beforeEach`. Returns a promise when the navigation has to
 * wait for the blade to close first — which is the whole point: the chunk, the
 * mount and the fetch stay behind a covered screen, and the closing animation
 * gets an idle main thread instead of competing with the page it is hiding.
 */
export function beginRouteTransition(
  to: RouteLocationNormalized,
  from: RouteLocationNormalized,
): Promise<void> | void {
  if (!coveredByCurtain(to, from)) return

  direction.value = orderOf(to.meta) < orderOf(from.meta) ? -1 : 1
  generation += 1
  ready = false
  obligated = false
  painted = false
  waiting.value = false

  clearTimers()
  // Whatever happens next — a redirect, a slow disk, a page that never answers —
  // the blade lifts on its own.
  maxTimer = window.setTimeout(reveal, HOLD_MAX_MS)
  slowTimer = window.setTimeout(() => {
    if (phase.value !== 'idle') waiting.value = true
  }, SLOW_MS)

  // A second click while the blade is closing or already closed: stay closed,
  // and let the new destination own the deadlines that were just re-armed.
  if (phase.value === 'covering' || phase.value === 'holding') return

  phase.value = 'covering'
  return new Promise<void>((resolve) => {
    coverResolve = resolve
    // Navigation must never depend on an animation completing.
    window.setTimeout(() => {
      if (coverResolve === resolve) releaseCover()
    }, COVER_MS + 420)
  })
}

/** The blade has finished closing; a navigation waiting on it may now proceed. */
export function markCurtainClosed(): void {
  if (phase.value !== 'covering') return
  phase.value = 'holding'
  coveredAt = performance.now()
  releaseCover()
  holdTimer = window.setTimeout(maybeReveal, HOLD_MIN_MS + 20)
}

/** The route is committed; the new component is in the DOM after this tick. */
export async function markRouteCommitted(): Promise<void> {
  if (phase.value === 'idle') return
  await nextTick()
  // One frame later the browser has had the chance to lay the page out, so the
  // blade lifts over something that is actually there — not over an empty box.
  await new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
  painted = true
  // The phase may well have moved on during those two ticks — abandoning the
  // navigation, or revealing without ever needing this page. `maybeReveal` only
  // acts while the curtain is holding, so it is the one that decides.
  maybeReveal()
}

function maybeReveal(): void {
  if (phase.value !== 'holding') return
  if (!painted) return
  // A page that asked to be waited for gets waited for — up to HOLD_MAX, which
  // is the timer's job, because this function cannot run on its own.
  if (obligated && !ready) return
  if (performance.now() - coveredAt < HOLD_MIN_MS) return
  reveal()
}

/** Lift the blade. Safe to call at any time; `markCurtainOpen` finishes the job. */
export function reveal(): void {
  if (phase.value === 'idle' || phase.value === 'revealing') return
  clearTimers()
  phase.value = 'revealing'
}

/** The blade is out of the way: nothing is pending any more. */
export function markCurtainOpen(): void {
  if (phase.value === 'idle') return
  clearTimers()
  phase.value = 'idle'
  waiting.value = false
}

/**
 * Drop the curtain without sweeping it away. Used when a navigation is
 * abandoned (`afterEach` failure, `onError`) and when the shell owning the
 * curtain is torn down mid-flight — going into the admin area, for instance,
 * where there is no blade left to sweep.
 */
export function abortRouteTransition(): void {
  clearTimers()
  if (phase.value === 'idle') return
  phase.value = 'idle'
  waiting.value = false
  releaseCover()
}

/** A failed navigation must not leave the blade hanging over the page. */
export function cancelRouteTransition(): void {
  if (phase.value === 'covering') {
    // Never got closed: drop it rather than sweep across a page the visitor was
    // not sent to.
    abortRouteTransition()
  } else {
    reveal()
  }
}

/**
 * A page calls this where its loading work starts — not only in setup — to say
 * "wait for me", and then calls the returned function once the content is
 * actually there.
 *
 * Registering it at the start of the work is deliberate: a list keeps its
 * component alive across a page turn and a detail view can reload in place, so
 * an obligation taken in setup would not be there for the second load. Calling
 * it again simply re-registers for whichever navigation is pending.
 *
 * A page that never asks is revealed as soon as it has been painted, so this
 * cannot become a way to hang the site.
 */
export function usePageReady(): () => void {
  const registered = generation
  obligated = phase.value !== 'idle'
  return () => {
    // A load that belongs to an earlier navigation — the page the visitor just
    // left, still finishing in the background — must not lift the blade that is
    // covering the one they are going to.
    if (registered !== generation) return
    ready = true
    maybeReveal()
  }
}

/**
 * Read-only view of the curtain for the components that draw it. Separate
 * computeds rather than `readonly()` so every consumer sees plain refs.
 */
export function useRouteCurtain() {
  return {
    phase: computed(() => phase.value),
    direction: computed(() => direction.value),
    waiting: computed(() => waiting.value),
    active: computed(() => phase.value !== 'idle'),
  }
}
