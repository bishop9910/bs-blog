<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    /** Avatar URL taken from the site profile; empty falls back to a monogram. */
    src?: string | null
    /** Author name: drives the alt text and the fallback letter. */
    name?: string | null
    /** Rendered size in CSS pixels. */
    size?: number
    /** Soft accent halo, used by the hero and the about page. */
    ring?: boolean
  }>(),
  { src: '', name: '', size: 72, ring: false },
)

// A broken image URL should degrade to the monogram instead of a torn icon.
const failed = ref(false)

watch(
  () => props.src,
  () => {
    failed.value = false
  },
)

const url = computed(() => (props.src ?? '').trim())
const showImage = computed(() => url.value.length > 0 && !failed.value)

const initial = computed(() => {
  const first = Array.from((props.name ?? '').trim())[0]
  if (!first) return 'B'
  return /[a-z]/i.test(first) ? first.toUpperCase() : first
})

const alt = computed(() => (props.name ? `${props.name} 的头像` : '作者头像'))
</script>

<template>
  <span
    class="avatar"
    :class="{ 'avatar--ring': ring }"
    :style="{ '--avatar-size': `${size}px`, '--avatar-font': `${Math.round(size * 0.42)}px` }"
  >
    <img
      v-if="showImage"
      class="avatar__img"
      :src="url"
      :alt="alt"
      loading="lazy"
      decoding="async"
      @error="failed = true"
    />
    <span v-else class="avatar__monogram" aria-hidden="true">{{ initial }}</span>
  </span>
</template>

<style scoped>
.avatar {
  position: relative;
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: var(--avatar-size);
  height: var(--avatar-size);
  overflow: hidden;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-full);
  background: var(--bg-inset);
}

.avatar--ring {
  border-color: var(--accent-border);
  box-shadow:
    0 0 0 4px color-mix(in srgb, var(--accent) 12%, transparent),
    0 0 28px var(--glow-accent);
}

.avatar__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar__monogram {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  background: linear-gradient(140deg, var(--accent-subtle), transparent 72%);
  color: var(--accent);
  font-family: var(--font-display);
  font-size: var(--avatar-font);
  line-height: 1;
  user-select: none;
}
</style>
