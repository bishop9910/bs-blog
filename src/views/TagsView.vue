<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { listTags } from '@/api/posts'
import type { Tag } from '@/types/blog'
import EmptyState from '@/components/EmptyState.vue'

const tags = ref<Tag[]>([])
const loading = ref(true)
const error = ref('')

const maxCount = computed(() => Math.max(1, ...tags.value.map((tag) => tag.postCount)))

/** Weight each tag by how much of the archive it covers, with a floor so a
 *  single-post tag is still comfortably tappable. */
function weight(tag: Tag): number {
  return 0.85 + (tag.postCount / maxCount.value) * 0.9
}
</script>

<template>
  <div class="page page--narrow">
    <header class="page__head">
      <p class="eyebrow">索引</p>
      <h1 class="page__title">标签</h1>
      <p v-if="!loading && !error" class="page__count">{{ tags.length }} 个标签</p>
    </header>

    <div v-if="loading" class="skeleton" />

    <EmptyState v-else-if="error" :title="error" description="请确认后端已启动，然后重试。" />

    <EmptyState
      v-else-if="tags.length === 0"
      title="还没有标签"
      description="给文章加上标签，它们会自动出现在这里。"
    />

    <div v-else class="cloud">
      <RouterLink
        v-for="tag in tags"
        :key="tag.id"
        class="cloud__item"
        :style="{
          fontSize: `${weight(tag)}rem`,
          '--tag-color': tag.color || 'var(--accent)',
        }"
        :to="{ name: 'tag', params: { slug: tag.slug } }"
      >
        <span class="cloud__name">{{ tag.name }}</span>
        <span class="cloud__count">{{ tag.postCount }}</span>
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
.cloud {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: var(--space-md) var(--space-lg);
  padding: var(--space-lg) 0;
}

.cloud__item {
  --tag-color: var(--accent);
  display: inline-flex;
  align-items: baseline;
  gap: 0.25em;
  color: var(--text-secondary);
  font-family: var(--font-display);
  line-height: 1.2;
  text-decoration: none;
  transition:
    color var(--dur-fast) var(--ease-out),
    transform var(--dur-fast) var(--ease-out);
}

.cloud__item:hover {
  color: var(--tag-color);
  transform: translateY(-2px);
}

.cloud__name {
  text-decoration: underline;
  text-decoration-color: transparent;
  text-underline-offset: 0.2em;
  transition: text-decoration-color var(--dur-fast) var(--ease-out);
}

.cloud__item:hover .cloud__name {
  text-decoration-color: var(--tag-color);
}

.cloud__count {
  color: var(--text-muted);
  font-family: var(--font-body);
  font-size: var(--text-2xs);
  font-variant-numeric: tabular-nums;
}

.skeleton {
  height: 200px;
  border-radius: var(--radius-lg);
  background: linear-gradient(
    90deg,
    var(--bg-subtle) 25%,
    var(--bg-hover) 37%,
    var(--bg-subtle) 63%
  );
  background-size: 400% 100%;
  animation: skeleton 1.4s ease-in-out infinite;
}

@keyframes skeleton {
  from {
    background-position: 100% 50%;
  }
  to {
    background-position: 0 50%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .skeleton {
    animation: none;
  }
}
</style>
