<script setup lang="ts">
import { computed, ref } from 'vue'
import { PhEyeSlash, PhFlower, PhSnowflake } from '@phosphor-icons/vue'
import petal1 from '@/assets/sakura/sakura_1.png'
import petal3 from '@/assets/sakura/sakura_3.png'
import petal5 from '@/assets/sakura/sakura_5.png'
import { useTheme } from '@/composables/useTheme'

/**
 * Ambient background effects (sakura / snow), rendered by tsParticles.
 *
 * The petals are the author's own drawings from the old site
 * (G:\bishop9910-website\src\assets\img\sakura). The whole layer sits behind the
 * page (z-index -1), never intercepts clicks, and the little switch in the
 * corner is remembered in localStorage under `bs-blog.effects`.
 *
 * `<VueParticles>` is registered globally by the plugin installed in main.ts.
 */

type EffectMode = 'sakura' | 'snow' | 'off'

interface Choice {
  value: EffectMode
  title: string
  label: string
}

const STORAGE_KEY = 'bs-blog.effects'
const STAGE_ID = 'ambient-weather'
const PETALS = [petal1, petal3, petal5]

const CHOICES: Choice[] = [
  { value: 'sakura', title: '樱花飘落', label: '樱花' },
  { value: 'snow', title: '雪花飘落', label: '雪花' },
  { value: 'off', title: '关闭背景动效', label: '关闭' },
]

const reduced =
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

const { effective } = useTheme()

function read(): EffectMode {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (raw === 'sakura' || raw === 'snow' || raw === 'off') return raw
  } catch {
    /* ignore */
  }
  // Nothing stored yet: don't animate for people who asked for less motion.
  return reduced ? 'off' : 'sakura'
}

const mode = ref<EffectMode>(read())

function setMode(next: EffectMode): void {
  mode.value = next
  try {
    window.localStorage.setItem(STORAGE_KEY, next)
  } catch {
    /* ignore */
  }
}

/**
 * Options shared by both effects; the canvas stays inside our own layer.
 *
 * No `pauseOnOutsideViewport`: this layer is fixed to the viewport, so it can
 * never scroll out of view, and the flag left the container permanently paused
 * (mounted canvas, zero frames drawn) whenever the window did not report
 * itself as focused. Pausing in background tabs is left to the browser's own
 * requestAnimationFrame throttling, which costs us nothing here.
 */
function base() {
  return {
    fullScreen: { enable: false },
    detectRetina: true,
    fpsLimit: 45,
  }
}

const options = computed(() => {
  if (mode.value === 'snow') {
    return {
      ...base(),
      particles: {
        number: { value: 64, density: { enable: true, width: 1600, height: 900 } },
        // Snow has to stay visible on both themes.
        color: { value: effective.value === 'light' ? '#7c8aa5' : '#ffffff' },
        shape: { type: 'circle' },
        size: { value: { min: 1, max: 3.6 } },
        opacity: { value: { min: 0.25, max: 0.9 } },
        move: {
          enable: true,
          speed: { min: 0.7, max: 2.4 },
          direction: 'bottom',
          straight: false,
          outModes: { default: 'out' },
        },
        wobble: { enable: true, distance: 12, speed: { min: -4, max: 4 } },
      },
    }
  }

  return {
    ...base(),
    particles: {
      number: { value: 18, density: { enable: true, width: 1600, height: 900 } },
      shape: {
        type: 'image',
        options: { image: PETALS.map((src) => ({ src, width: 48, height: 48 })) },
      },
      size: { value: { min: 10, max: 22 } },
      opacity: { value: { min: 0.35, max: 0.85 }, animation: { enable: true, speed: 0.8 } },
      move: {
        enable: true,
        speed: { min: 0.7, max: 1.9 },
        direction: 'bottom',
        straight: false,
        outModes: { default: 'out' },
      },
      rotate: { value: { min: 0, max: 360 }, animation: { enable: true, speed: 10 } },
      tilt: { value: { min: 0, max: 360 }, animation: { enable: true, speed: 8 } },
      wobble: { enable: true, distance: 22, speed: { min: -7, max: 7 } },
    },
  }
})
</script>

<template>
  <div class="weather" aria-hidden="true">
    <VueParticles
      v-if="mode !== 'off'"
      :id="STAGE_ID"
      class="weather__stage"
      :options="options"
    />

    <div class="weather__switch" role="group" aria-label="背景动效">
      <button
        v-for="choice in CHOICES"
        :key="choice.value"
        class="weather__btn"
        :class="{ 'is-active': mode === choice.value }"
        type="button"
        :title="choice.title"
        :aria-pressed="mode === choice.value"
        @click="setMode(choice.value)"
      >
        <PhFlower v-if="choice.value === 'sakura'" :size="14" weight="fill" />
        <PhSnowflake v-else-if="choice.value === 'snow'" :size="14" weight="fill" />
        <PhEyeSlash v-else :size="14" weight="fill" />
        <span class="weather__btn-label">{{ choice.label }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* No z-index here on purpose: the layer must not create a stacking context, so
   the canvas can go behind the page while the switch stays on top of it. */
.weather {
  position: fixed;
  inset: 0;
  pointer-events: none;
}

.weather__stage {
  position: fixed;
  inset: 0;
  z-index: -1;
  pointer-events: none;
}

.weather__stage :deep(canvas) {
  display: block;
  width: 100% !important;
  height: 100% !important;
}

.weather__switch {
  position: fixed;
  right: var(--space-md);
  bottom: var(--space-md);
  z-index: var(--z-sticky);
  display: flex;
  gap: 2px;
  padding: 4px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-full);
  background: color-mix(in srgb, var(--bg-elevated) 85%, transparent);
  backdrop-filter: blur(8px);
  opacity: 0.5;
  pointer-events: auto;
  transition: opacity var(--dur-fast) var(--ease-out);
}

.weather__switch:hover,
.weather__switch:focus-within {
  opacity: 1;
}

.weather__btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 10px;
  border: none;
  border-radius: var(--radius-full);
  background: none;
  color: var(--text-muted);
  font-size: var(--text-2xs);
  cursor: pointer;
  transition:
    background var(--dur-fast) var(--ease-out),
    color var(--dur-fast) var(--ease-out);
}

.weather__btn:hover {
  color: var(--text-primary);
}

.weather__btn.is-active {
  background: var(--accent-subtle);
  color: var(--accent);
}

@media (max-width: 640px) {
  .weather__btn-label {
    display: none;
  }

  .weather__btn {
    padding: 6px;
  }
}
</style>
