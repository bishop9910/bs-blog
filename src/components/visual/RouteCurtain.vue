<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { motion } from '@/utils/anime'
import {
  CURTAIN_TIMING,
  abortRouteTransition,
  markCurtainClosed,
  markCurtainOpen,
  useRouteCurtain,
} from '@/composables/usePageTransition'

const { phase, direction, waiting } = useRouteCurtain()

const bladeEl = ref<HTMLElement | null>(null)

/**
 * How far the blade travels: further than its own width, so it starts and ends
 * completely clear of the viewport. These are percentages of the blade's own
 * width (150% of the viewport), which is why they look large.
 */
const TRAVEL = 176

const travel = (sign: number): string => `${sign * TRAVEL}%`

/**
 * The blade closes from the side the visitor came from: going forward it enters
 * from the right and sweeps left, going back it enters from the left. Its first
 * paint has to be off-screen on that side — the element is created in the same
 * frame the animation starts, and anything else flashes it into view.
 */
function close(): void {
  const el = bladeEl.value
  const anim = el
    ? motion(el, {
        translateX: [travel(direction.value > 0 ? 1 : -1), '0%'],
        duration: CURTAIN_TIMING.cover,
        ease: 'outExpo',
        onComplete: markCurtainClosed,
      })
    : null
  // No animation (reduced motion, or nothing to animate): the navigation is
  // still waiting on this, so it has to be released either way.
  if (!anim) markCurtainClosed()
}

/** …and leaves the way it was going, continuing the motion instead of reversing it. */
function open(): void {
  const el = bladeEl.value
  const anim = el
    ? motion(el, {
        translateX: ['0%', travel(direction.value > 0 ? -1 : 1)],
        duration: CURTAIN_TIMING.reveal,
        ease: 'inOutQuad',
        onComplete: markCurtainOpen,
      })
    : null
  if (!anim) markCurtainOpen()
}

// The component is created by `v-if` at the moment the phase turns to
// `covering`, so its own mount is the start of the closing animation.
onMounted(() => {
  if (phase.value === 'covering') close()
})

watch(phase, (next) => {
  if (next === 'revealing') open()
})

// Tearing the shell down mid-flight (public → admin) would otherwise leave the
// machine believing a blade is still closed that no longer exists.
onBeforeUnmount(abortRouteTransition)
</script>

<template>
  <div
    class="curtain"
    :class="{ 'is-revealing': phase === 'revealing' }"
    :data-dir="direction"
    aria-hidden="true"
  >
    <div ref="bladeEl" class="curtain__blade" />

    <!-- Only once the wait has gone on long enough to be worth explaining. -->
    <p class="curtain__wait" :class="{ 'is-on': waiting }">
      <span class="curtain__pulse" />
      正在载入
    </p>
  </div>
</template>

<style scoped>
.curtain {
  position: fixed;
  inset: 0;
  /*
    Above the page and its wallpaper, below the header: the header keeps
    working, and its active nav item updates the moment the route commits —
    which is the feedback that the click landed, delivered while the content is
    still covered.
  */
  z-index: calc(var(--z-sticky) - 1);
  overflow: hidden;
  pointer-events: auto;
}

/* On the way out the page underneath is already the destination: let it be used. */
.curtain.is-revealing {
  pointer-events: none;
}

.curtain__blade {
  position: absolute;
  /* Overhang: the leading edge is skewed, and this keeps its corners outside. */
  top: -8vh;
  bottom: -8vh;
  left: 0;
  width: 150%;
  will-change: transform;
  /*
    Four layers, top to bottom: the lit edge that leads the sweep, a faint
    diagonal weave so the panel reads as a surface rather than a fill, and the
    surface itself — the theme's own base colour, nearly opaque. That opacity is
    the point of the whole component: a half-rendered page must not be
    recognisable through it.
  */
  background-image:
    linear-gradient(
      90deg,
      var(--accent) 0%,
      color-mix(in srgb, var(--accent) 55%, transparent) 0.35%,
      color-mix(in srgb, var(--accent) 15%, transparent) 2.2%,
      transparent 9%
    ),
    repeating-linear-gradient(
      118deg,
      color-mix(in srgb, var(--text-primary) 5%, transparent) 0 1px,
      transparent 1px 7px
    ),
    linear-gradient(
      color-mix(in srgb, var(--bg-base) 90%, transparent),
      color-mix(in srgb, var(--bg-base) 90%, transparent)
    );
  backdrop-filter: blur(28px) saturate(1.15);
  clip-path: polygon(0 0, 100% 0, calc(100% - 6vw) 100%, -6vw 100%);
  /* Off-screen to the right: the side the blade enters from when going forward. */
  transform: translate3d(176%, 0, 0);
}

/* Going back, everything mirrors: the edge on the right, entering from the left. */
.curtain[data-dir='-1'] .curtain__blade {
  background-image:
    linear-gradient(
      270deg,
      var(--accent) 0%,
      color-mix(in srgb, var(--accent) 55%, transparent) 0.35%,
      color-mix(in srgb, var(--accent) 15%, transparent) 2.2%,
      transparent 9%
    ),
    repeating-linear-gradient(
      62deg,
      color-mix(in srgb, var(--text-primary) 5%, transparent) 0 1px,
      transparent 1px 7px
    ),
    linear-gradient(
      color-mix(in srgb, var(--bg-base) 90%, transparent),
      color-mix(in srgb, var(--bg-base) 90%, transparent)
    );
  clip-path: polygon(6vw 0, 100% 0, 100% 100%, 0 100%);
  transform: translate3d(-176%, 0, 0);
}

.curtain__wait {
  position: absolute;
  top: 50%;
  left: 50%;
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin: 0;
  padding: var(--space-sm) var(--space-md);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-full);
  background: color-mix(in srgb, var(--bg-elevated) 88%, transparent);
  color: var(--text-secondary);
  font-size: var(--text-sm);
  letter-spacing: 0.04em;
  transform: translate(-50%, -50%);
  opacity: 0;
  transition: opacity var(--dur-normal) var(--ease-out);
}

.curtain__wait.is-on {
  opacity: 1;
}

.curtain__pulse {
  width: 7px;
  height: 7px;
  border-radius: var(--radius-full);
  background: var(--accent);
  box-shadow: 0 0 0 0 var(--accent-subtle);
  animation: curtain-pulse 1400ms var(--ease-out) infinite;
}

@keyframes curtain-pulse {
  0% {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--accent) 55%, transparent);
  }
  70% {
    box-shadow: 0 0 0 9px transparent;
  }
  100% {
    box-shadow: 0 0 0 0 transparent;
  }
}

@media (prefers-reduced-motion: reduce) {
  .curtain__pulse {
    animation: none;
  }
}
</style>
