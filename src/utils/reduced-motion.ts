/**
 * Whether the visitor asked the OS for less motion.
 *
 * This lives on its own, free of both anime.js and lenis, because the router's
 * navigation guard needs it: importing it from either of those would drag a
 * whole library into the entry bundle to answer a one-line media query.
 */
export const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)'

/** True when the visitor asked the OS for less motion. */
export function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  return window.matchMedia(REDUCED_MOTION_QUERY).matches
}
