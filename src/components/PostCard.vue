<script setup lang="ts">
import { computed } from 'vue'
import { PhArrowUpRight } from '@phosphor-icons/vue'
import type { PostSummary } from '@/types/blog'
import { formatDate } from '@/utils/format'
import TagPill from './TagPill.vue'

const props = withDefaults(
  defineProps<{
    post: PostSummary
    /** Larger treatment for the lead item of a grid. */
    featured?: boolean
  }>(),
  { featured: false },
)

const target = computed(() => ({ name: 'post', params: { slug: props.post.slug } }))
const isDraft = computed(() => props.post.status === 'draft')
</script>

<template>
  <article class="card" :class="{ 'card--featured': featured }" v-reveal>
    <RouterLink class="card__link" :to="target">
      <div v-if="post.coverUrl" class="card__cover">
        <img :src="post.coverUrl" :alt="post.title" loading="lazy" decoding="async" />
      </div>

      <div class="card__body">
        <div class="card__meta">
          <time :datetime="post.publishedAt ?? post.createdAt">
            {{ formatDate(post.publishedAt ?? post.createdAt) }}
          </time>
          <span class="card__dot" aria-hidden="true">·</span>
          <span>{{ post.readingMinutes }} 分钟</span>
          <span v-if="isDraft" class="card__draft">草稿</span>
        </div>

        <h3 class="card__title">
          <span>{{ post.title }}</span>
          <PhArrowUpRight class="card__arrow" :size="18" weight="bold" />
        </h3>

        <p v-if="post.summary" class="card__summary">{{ post.summary }}</p>

        <div v-if="post.tags.length > 0" class="card__tags">
          <TagPill v-for="tag in post.tags" :key="tag.id" :tag="tag" :link="false" />
        </div>
      </div>
    </RouterLink>
  </article>
</template>

<style scoped>
.card {
  position: relative;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  overflow: hidden;
  transition:
    border-color var(--dur-normal) var(--ease-out),
    transform var(--dur-normal) var(--ease-out),
    box-shadow var(--dur-normal) var(--ease-out);
}

.card:hover {
  border-color: var(--border-accent);
  transform: translateY(-4px);
  box-shadow: var(--shadow-lg);
}

.card__link {
  display: flex;
  flex-direction: column;
  height: 100%;
  color: inherit;
  text-decoration: none;
}

.card__cover {
  aspect-ratio: 16 / 9;
  overflow: hidden;
  background: var(--bg-inset);
}

.card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform var(--dur-slower) var(--ease-out);
}

.card:hover .card__cover img {
  transform: scale(1.04);
}

.card__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-lg);
}

.card--featured .card__body {
  padding: var(--space-xl);
}

.card__meta {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-wide);
  text-transform: uppercase;
}

.card__draft {
  padding: 0.1em 0.5em;
  border: 1px solid var(--warning);
  border-radius: var(--radius-full);
  color: var(--warning);
}

.card__title {
  display: flex;
  align-items: baseline;
  gap: var(--space-xs);
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-xl);
  line-height: var(--leading-snug);
}

.card--featured .card__title {
  font-size: var(--text-3xl);
}

.card__arrow {
  flex-shrink: 0;
  color: var(--accent);
  opacity: 0;
  transform: translate(-4px, 4px);
  transition:
    opacity var(--dur-fast) var(--ease-out),
    transform var(--dur-fast) var(--ease-out);
}

.card:hover .card__arrow {
  opacity: 1;
  transform: translate(0, 0);
}

.card__summary {
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-xs);
  margin-top: auto;
  padding-top: var(--space-sm);
}
</style>
