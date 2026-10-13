<script setup lang="ts">
import { computed } from 'vue'
import type { Tag } from '@/types/blog'

const props = withDefaults(
  defineProps<{
    tag: Pick<Tag, 'name' | 'slug'> & { color?: string }
    count?: number
    active?: boolean
    link?: boolean
  }>(),
  { count: undefined, active: false, link: true },
)

const style = computed(() =>
  props.tag.color ? { '--tag-color': props.tag.color } : undefined,
)
</script>

<template>
  <component
    :is="link ? 'RouterLink' : 'span'"
    class="tag-pill"
    :class="{ 'is-active': active }"
    :style="style"
    :to="link ? { name: 'tag', params: { slug: tag.slug } } : undefined"
  >
    <span class="tag-pill__hash" aria-hidden="true">#</span>
    <span class="tag-pill__name">{{ tag.name }}</span>
    <span v-if="count !== undefined" class="tag-pill__count">{{ count }}</span>
  </component>
</template>

<style scoped>
.tag-pill {
  --tag-color: var(--accent);
  display: inline-flex;
  align-items: center;
  gap: 0.3em;
  padding: 0.3em 0.7em;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-full);
  background: var(--bg-subtle);
  color: var(--text-secondary);
  font-size: var(--text-xs);
  line-height: 1.4;
  text-decoration: none;
  white-space: nowrap;
  transition:
    border-color var(--dur-fast) var(--ease-out),
    color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

a.tag-pill:hover {
  border-color: var(--tag-color);
  color: var(--tag-color);
  background: var(--bg-hover);
}

.tag-pill.is-active {
  border-color: var(--tag-color);
  background: var(--accent-subtle);
  color: var(--tag-color);
}

.tag-pill__hash {
  color: var(--tag-color);
  opacity: 0.8;
}

.tag-pill__count {
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}
</style>
