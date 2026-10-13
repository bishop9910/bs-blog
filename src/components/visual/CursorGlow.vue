<script setup lang="ts">
/**
 * CursorGlow — a soft accent spotlight that trails the pointer.
 *
 * Pure CSS transform on a fixed radial gradient: no canvas, no dependency.
 * Disabled on coarse pointers (touch) and under prefers-reduced-motion.
 */
import { onBeforeUnmount, onMounted, ref } from 'vue'

const el = ref<HTMLElement | null>(null)
const active = ref(false)

let targetX = 0
let targetY = 0
let currentX = 0
let currentY = 0
let raf = 0
let enabled = false

function onMove(event: PointerEvent): void {
  targetX = event.clientX
  targetY = event.clientY
  if (!active.value) {
    active.value = true
    currentX = targetX
    currentY = targetY
  }
}

function tick(): void {
  // Critically-damped-ish lerp: follows without feeling attached.
  currentX += (targetX - currentX) * 0.12
  currentY += (targetY - currentY) * 0.12
  const node = el.value
  if (node) {
    node.style.transform = `translate3d(${currentX}px, ${currentY}px, 0)`
  }
  raf = requestAnimationFrame(tick)
}

function onLeave(): void {
  active.value = false
}

onMounted(() => {
  const coarse = window.matchMedia('(pointer: coarse)').matches
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (coarse || reduced) return

  enabled = true
  window.addEventListener('pointermove', onMove, { passive: true })
  document.addEventListener('pointerleave', onLeave)
  raf = requestAnimationFrame(tick)
})

onBeforeUnmount(() => {
  if (!enabled) return
  cancelAnimationFrame(raf)
  window.removeEventListener('pointermove', onMove)
  document.removeEventListener('pointerleave', onLeave)
})
</script>

<template>
  <div ref="el" class="glow" :class="{ 'is-active': active }" aria-hidden="true" />
</template>

<style scoped>
.glow {
  position: fixed;
  top: 0;
  left: 0;
  z-index: var(--z-overlay);
  width: 480px;
  height: 480px;
  margin: -240px 0 0 -240px;
  background: radial-gradient(circle closest-side, var(--accent-subtle) 0%, transparent 70%);
  opacity: 0;
  pointer-events: none;
  transition: opacity var(--dur-slow) var(--ease-out);
  will-change: transform;
}

.glow.is-active {
  opacity: 1;
}
</style>
