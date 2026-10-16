import Lenis from 'lenis'
import { prefersReducedMotion } from '@/utils/reduced-motion'

/**
 * Inertial scrolling. Kept in a module so the public layout can start and stop
 * it without every navigation building a new instance, and disabled outright
 * for anyone who asked for reduced motion — a hijacked scroll is exactly the
 * kind of movement that preference is about.
 */
let lenis: Lenis | null = null
let frame = 0

// Part of this module's public surface: layouts and views ask it before
// scheduling any scrolling of their own.
export { prefersReducedMotion }

export function startSmoothScroll() {
  if (lenis || typeof window === 'undefined' || prefersReducedMotion()) return
  lenis = new Lenis({
    duration: 1.05,
    easing: (t: number) => Math.min(1, 1.001 - Math.pow(2, -10 * t)),
    smoothWheel: true,
    // Leave touch scrolling to the platform: overriding it makes mobile feel
    // laggy and breaks native scroll-to-top on status-bar taps.
    syncTouch: false,
  })

  const loop = (time: number) => {
    lenis?.raf(time)
    frame = window.requestAnimationFrame(loop)
  }
  frame = window.requestAnimationFrame(loop)
}

export function stopSmoothScroll() {
  if (frame) window.cancelAnimationFrame(frame)
  frame = 0
  lenis?.destroy()
  lenis = null
}

/** Used by the router on navigation, so a new page always starts at the top. */
export function resetScroll(immediate = true) {
  if (lenis) {
    lenis.scrollTo(0, { immediate })
    return
  }
  window.scrollTo({ top: 0, behavior: immediate ? 'auto' : 'smooth' })
}

/** Lets in-page anchors (e.g. a "back to top" link) reuse the smooth engine. */
export function scrollToElement(target: HTMLElement | string, offset = 80) {
  const element =
    typeof target === 'string' ? document.querySelector<HTMLElement>(target) : target
  if (!element) return
  if (lenis) {
    lenis.scrollTo(element, { offset: -offset })
    return
  }
  const top = element.getBoundingClientRect().top + window.scrollY - offset
  window.scrollTo({ top, behavior: prefersReducedMotion() ? 'auto' : 'smooth' })
}
