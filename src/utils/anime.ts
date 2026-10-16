/**
 * anime.js v4 — the site's one motion toolkit.
 *
 * Everything animated in JS goes through here so that three rules cannot be
 * forgotten at a call site:
 *
 *   1. `prefers-reduced-motion` is checked FIRST. Every helper is a no-op when
 *      the visitor asked for less motion, and the caller's markup is expected
 *      to render in its natural (finished) state — the same contract `v-reveal`
 *      uses, so a failed animation never leaves content hidden.
 *   2. Easing/duration defaults are the design tokens' equivalents, not an
 *      anime.js default, so a JS animation and a CSS transition feel alike.
 *   3. anime.js is imported once. The library's API moved a lot between v3 and
 *      v4; keeping the surface here means upgrading it touches one file.
 *
 * Raw primitives are re-exported at the bottom for the few places that need
 * them (audio-reactive setters, timelines built from several steps).
 */

import {
  animate,
  createAnimatable,
  createSpring,
  createTimeline,
  onScroll,
  scrambleText,
  stagger,
  utils,
  type AnimationParams,
  type EasingParam,
  type JSAnimation,
  type ScrollObserverParams,
  type StaggerParams,
  type TargetsParam,
  type Timeline,
  type TimelineParams,
} from 'animejs'

const REDUCE_QUERY = '(prefers-reduced-motion: reduce)'

/** True when the visitor asked the OS for less motion. */
export function reduceMotion(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  return window.matchMedia(REDUCE_QUERY).matches
}

/**
 * The site's own "out-expo" curve, matching `--ease-out` in tokens.css. Used as
 * the default so JS motion does not read as a different material than CSS.
 */
export const EASE_OUT: EasingParam = 'outExpo'

/**
 * `animate()` that refuses to run under reduced motion. Returns `null` in that
 * case — callers must therefore treat the DOM's natural state as the end state.
 */
export function motion(targets: TargetsParam, params: AnimationParams): JSAnimation | null {
  if (reduceMotion()) return null
  return animate(targets, params)
}

/** Timeline counterpart of {@link motion}. */
export function motionTimeline(params?: TimelineParams): Timeline | null {
  if (reduceMotion()) return null
  return createTimeline({ defaults: { ease: EASE_OUT }, ...params })
}

export interface StaggerEntranceOptions {
  /** Gap between neighbours in ms. */
  each?: number
  /** Where the wave starts. */
  from?: StaggerParams['from']
}

/**
 * Staggered entrance for a list of siblings. The wave starts from the first
 * card (a grid reads top-left first), unlike anime.js's `center` default.
 */
export function motionStagger(
  targets: TargetsParam,
  params: AnimationParams,
  { each = 55, from = 'first' }: StaggerEntranceOptions = {},
): JSAnimation | null {
  if (reduceMotion()) return null
  return animate(targets, {
    duration: 620,
    ease: EASE_OUT,
    delay: stagger(each, { from }),
    ...params,
  })
}

export interface EnterOptions {
  /** How far into the viewport the element must be, e.g. `'80%'` or `0`. */
  enter?: ScrollObserverParams['enter']
  /** Replay every time it scrolls back into view. */
  repeat?: boolean
  /** Link playback to scroll progress instead of firing once. */
  sync?: ScrollObserverParams['sync']
}

/**
 * Scroll-triggered entrance: the animation waits until the element is in view.
 * Elements are never pre-hidden from CSS — the caller animates FROM an offset,
 * so a page that never gets this far still shows everything.
 */
export function motionOnEnter(
  targets: TargetsParam,
  params: AnimationParams,
  { enter = '80%', repeat = false, sync = false }: EnterOptions = {},
): JSAnimation | null {
  if (reduceMotion()) return null
  return animate(targets, {
    duration: 720,
    ease: EASE_OUT,
    ...params,
    autoplay: onScroll({ target: targets, enter, repeat, sync }),
  })
}

export interface ScrambleOptions {
  /** Delay before the reveal wave starts, in ms. */
  delay?: number
  /** Character set, in anime.js range syntax (`'A-Z0-9'`) or a named set. */
  chars?: string
  /** Where the reveal wave starts. */
  from?: 'left' | 'center' | 'right' | 'random' | 'auto'
  /** Show a caret character at the leading edge of the wave. */
  cursor?: boolean
}

/**
 * Scramble/reveal text: characters shuffle before settling on the final string.
 *
 * anime.js writes this tween to `innerHTML` — the value is a string, and the
 * library's OBJECT writer is what lands it (`text`/`textContent` would be
 * written as an *attribute* on an HTML element and never show up).
 */
export function scrambleTo(
  targets: TargetsParam,
  text: string,
  { delay = 0, chars = 'A-Z0-9!%#_', from = 'left', cursor = false }: ScrambleOptions = {},
): JSAnimation | null {
  const nodes = utils.$(targets) as unknown as HTMLElement[]
  if (reduceMotion()) {
    for (const node of nodes) node.innerHTML = text
    return null
  }
  return animate(targets, {
    innerHTML: scrambleText({ text, chars, from, delay, cursor }),
  })
}

export interface CountUpOptions {
  /** Start value. */
  from?: number
  /** Length of the count in ms. */
  duration?: number
  /** Formats each frame; defaults to a grouped integer (1,024). */
  format?: (value: number) => string
}

const defaultCountFormat = (value: number): string => Math.round(value).toLocaleString('zh-CN')

/**
 * Count a number up in a DOM node. anime.js animates a plain object and we
 * write the frame ourselves, which keeps the formatting (thousands separators,
 * units) out of the animation definition.
 */
export function countUp(
  target: HTMLElement,
  value: number,
  { from = 0, duration = 1100, format = defaultCountFormat }: CountUpOptions = {},
): JSAnimation | null {
  if (reduceMotion()) {
    target.textContent = format(value)
    return null
  }
  const proxy = { value: from }
  return animate(proxy, {
    value,
    duration,
    ease: EASE_OUT,
    onUpdate: () => {
      target.textContent = format(proxy.value)
    },
    onComplete: () => {
      target.textContent = format(value)
    },
  })
}

/** A physical spring, for gestures and panel snaps where a curve feels dead. */
export function springEase(stiffness = 120, damping = 14, mass = 1): EasingParam {
  return createSpring({ stiffness, damping, mass })
}

/** Settle a value with the library's damped lerp (used by the cursor/parallax). */
export const damp = utils.damp

export { animate, createAnimatable, createTimeline, onScroll, scrambleText, stagger, utils }
