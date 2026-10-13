<script setup lang="ts">
import { computed } from 'vue'
import { PhDesktop, PhMoonStars, PhSunDim } from '@phosphor-icons/vue'
import { useTheme } from '@/composables/useTheme'

const { mode, toggle } = useTheme()

const label = computed(() => {
  if (mode.value === 'dark') return '深色模式'
  if (mode.value === 'light') return '浅色模式'
  return '跟随系统'
})

const icon = computed(() => {
  if (mode.value === 'dark') return PhMoonStars
  if (mode.value === 'light') return PhSunDim
  return PhDesktop
})
</script>

<template>
  <button class="theme-toggle" type="button" :title="label" :aria-label="label" @click="toggle()">
    <component :is="icon" :size="18" weight="bold" />
  </button>
</template>

<style scoped>
.theme-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--hit-min);
  height: var(--hit-min);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition:
    color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.theme-toggle:hover {
  color: var(--accent);
  background: var(--bg-hover);
}

.theme-toggle:active {
  transform: scale(0.94);
}
</style>
