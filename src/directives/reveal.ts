/**
 * `v-reveal` — scroll-triggered entrance.
 *
 * A Vue directive rather than a component so it can be dropped on any element
 * without changing the markup shape. Uses one shared IntersectionObserver.
 * Elements start hidden ONLY once JS has confirmed IO support, so a no-JS or
 * failed-hydration render still shows all content.
 *
 * Usage:
 *   <div v-reveal>...</div>
 *   <div v-reveal="{ delay: 120, distance: 24 }">...</div>
 */

import type { Directive, DirectiveBinding } from 'vue'

interface RevealOptions {
  /** Stagger in ms. */
  delay?: number
  /** Travel distance in px. */
  distance?: number
  /** Only animate once (default) or every time it enters. */
  repeat?: boolean
}

const supported = typeof IntersectionObserver !== 'undefined'
const reduced =
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

let observer: IntersectionObserver | null = null

function ensureObserver(): IntersectionObserver {
  observer ??= new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-revealed')
          if (entry.target.getAttribute('data-reveal-repeat') !== 'true') {
            observer?.unobserve(entry.target)
          }
        } else if (entry.target.getAttribute('data-reveal-repeat') === 'true') {
          entry.target.classList.remove('is-revealed')
        }
      }
    },
    { rootMargin: '0px 0px -10% 0px', threshold: 0.05 },
  )
  return observer
}

export const reveal: Directive<HTMLElement, RevealOptions | undefined> = {
  mounted(el: HTMLElement, binding: DirectiveBinding<RevealOptions | undefined>) {
    // Respect the user's motion preference: never hide content.
    if (!supported || reduced) {
      el.classList.add('is-revealed')
      return
    }

    const { delay = 0, distance = 18, repeat = false } = binding.value ?? {}

    el.classList.add('reveal')
    el.style.setProperty('--reveal-delay', `${delay}ms`)
    el.style.setProperty('--reveal-distance', `${distance}px`)
    if (repeat) el.setAttribute('data-reveal-repeat', 'true')

    ensureObserver().observe(el)
  },

  unmounted(el: HTMLElement) {
    observer?.unobserve(el)
  },
}
