<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getArchive } from '@/api/posts'
import type { ArchiveYear } from '@/types/blog'
import { formatDateShort } from '@/utils/format'
import EmptyState from '@/components/EmptyState.vue'
import { usePageReady } from '@/composables/usePageTransition'

const years = ref<ArchiveYear[]>([])
const loading = ref(true)
const error = ref('')

const total = computed(() => years.value.reduce((sum, year) => sum + year.count, 0))

const MONTH_LABEL = [
  '',
  '一月',
  '二月',
  '三月',
  '四月',
  '五月',
  '六月',
  '七月',
  '八月',
  '九月',
  '十月',
  '十一月',
  '十二月',
]

function monthLabel(month: number): string {
  return MONTH_LABEL[month] ?? `${month} 月`
}

onMounted(async () => {
  // Nothing behind the blade until the timeline is actually there.
  const pageDone = usePageReady()
  try {
    const result = await getArchive()
    years.value = result.years
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : '加载失败'
  } finally {
    loading.value = false
    pageDone()
  }
})
</script>

<template>
  <div class="page page--narrow">
    <header class="page__head">
      <p class="eyebrow">时间线</p>
      <h1 class="page__title">归档</h1>
      <p v-if="!loading && !error" class="page__count">
        {{ years.length }} 年 · {{ total }} 篇文章
      </p>
    </header>

    <div v-if="loading" class="rail">
      <div v-for="n in 4" :key="n" class="skeleton" />
    </div>

    <EmptyState v-else-if="error" :title="error" description="请确认后端已启动，然后重试。" />

    <EmptyState
      v-else-if="years.length === 0"
      title="还没有归档"
      description="发布第一篇文章后，它会按年月出现在这里。"
    />

    <div v-else class="rail">
      <section v-for="year in years" :key="year.year" class="year" v-reveal>
        <h2 class="year__label">
          <span class="year__number">{{ year.year }}</span>
          <span class="year__count">{{ year.count }} 篇</span>
        </h2>

        <div v-for="month in year.months" :key="month.month" class="month">
          <h3 class="month__label">{{ monthLabel(month.month) }}</h3>
          <ul class="entries">
            <li v-for="post in month.posts" :key="post.slug" class="entry">
              <span class="entry__date">
                {{ formatDateShort(post.publishedAt ?? post.createdAt) }}
              </span>
              <RouterLink
                class="entry__link"
                :to="{ name: 'post', params: { slug: post.slug } }"
              >
                {{ post.title }}
              </RouterLink>
            </li>
          </ul>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.rail {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xl);
}

.year {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: var(--space-lg);
  padding-top: var(--space-lg);
  border-top: 1px solid var(--border-subtle);
}

.year__label {
  position: sticky;
  top: calc(var(--header-h) + var(--space-lg));
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
  align-self: start;
  margin: 0;
}

.year__number {
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  font-variant-numeric: tabular-nums;
}

.year__count {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-caps);
  text-transform: uppercase;
}

.month + .month {
  margin-top: var(--space-lg);
}

.month__label {
  margin: 0 0 var(--space-xs);
  color: var(--text-muted);
  font-size: var(--text-2xs);
  font-weight: 500;
  letter-spacing: var(--tracking-caps);
  text-transform: uppercase;
}

.entries {
  margin: 0;
  padding: 0;
  list-style: none;
}

.entry {
  display: flex;
  align-items: baseline;
  gap: var(--space-md);
  padding: var(--space-xs) 0;
  border-bottom: 1px solid transparent;
}

.entry__date {
  flex-shrink: 0;
  width: 56px;
  color: var(--text-muted);
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
}

.entry__link {
  color: var(--text-primary);
  font-size: var(--text-base);
  text-decoration: none;
  transition: color var(--dur-fast) var(--ease-out);
}

.entry__link:hover {
  color: var(--accent);
}

.skeleton {
  height: 120px;
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

@media (max-width: 720px) {
  .year {
    grid-template-columns: minmax(0, 1fr);
  }

  .year__label {
    position: static;
    flex-direction: row;
    align-items: baseline;
    gap: var(--space-md);
  }
}
</style>
