<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { PhArrowRight, PhWarningCircle } from '@phosphor-icons/vue'
import { listPublished, listTags } from '@/api/posts'
import type { PostSummary, Tag } from '@/types/blog'
import { useSite } from '@/composables/useSite'
import { formatCount } from '@/utils/format'
import EmptyState from '@/components/EmptyState.vue'
import PostCard from '@/components/PostCard.vue'
import TagPill from '@/components/TagPill.vue'

const { site } = useSite()

const posts = ref<PostSummary[]>([])
const tags = ref<Tag[]>([])
const loading = ref(true)
const error = ref('')

const lead = computed(() => posts.value[0])
const rest = computed(() => posts.value.slice(1))

const statItems = computed(() => {
  const stats = site.value?.stats
  if (!stats) return []
  return [
    { label: '文章', value: formatCount(stats.postCount) },
    { label: '标签', value: formatCount(stats.tagCount) },
    { label: '字数', value: formatCount(stats.wordCount) },
  ]
})

onMounted(async () => {
  try {
    const [page, tagList] = await Promise.all([
      listPublished({ page: 1, pageSize: 7, sort: 'published', order: 'desc' }),
      listTags(),
    ])
    posts.value = page.items
    tags.value = tagList.tags
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : '加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="home">
    <section class="hero">
      <p class="eyebrow">个人博客</p>
      <h1 class="hero__title">{{ site?.title ?? 'bs-blog' }}</h1>
      <p class="hero__tagline">{{ site?.tagline || '一个人的写作空间。' }}</p>

      <div class="hero__actions">
        <RouterLink class="btn btn--primary" :to="{ name: 'posts' }">
          开始阅读
          <PhArrowRight :size="16" weight="bold" />
        </RouterLink>
        <RouterLink class="btn" :to="{ name: 'archive' }">按时间浏览</RouterLink>
      </div>

      <dl v-if="statItems.length > 0" class="hero__stats">
        <div v-for="item in statItems" :key="item.label" class="hero__stat">
          <dt>{{ item.label }}</dt>
          <dd>{{ item.value }}</dd>
        </div>
      </dl>
    </section>

    <section class="section">
      <header class="section__head">
        <h2 class="section__title">最新文章</h2>
        <RouterLink class="section__more" :to="{ name: 'posts' }">全部文章</RouterLink>
      </header>

      <div v-if="loading" class="skeleton-grid">
        <div v-for="n in 3" :key="n" class="skeleton" />
      </div>

      <EmptyState
        v-else-if="error"
        :title="error"
        description="看起来后端没有在运行。启动 `go run ./cmd/server` 后刷新即可。"
      >
        <button class="btn" type="button" @click="$router.go(0)">
          <PhWarningCircle :size="16" />
          重新加载
        </button>
      </EmptyState>

      <EmptyState
        v-else-if="!lead"
        title="还没有文章"
        description="第一篇文章一旦发布，就会出现在这里。"
      >
        <RouterLink class="btn btn--primary" :to="{ name: 'admin-post-new' }">
          去写第一篇
        </RouterLink>
      </EmptyState>

      <div v-else class="bento">
        <PostCard class="bento__lead" :post="lead" featured />
        <PostCard v-for="post in rest" :key="post.id" class="bento__item" :post="post" />
      </div>
    </section>

    <section v-if="tags.length > 0" class="section">
      <header class="section__head">
        <h2 class="section__title">标签</h2>
        <RouterLink class="section__more" :to="{ name: 'tags' }">全部标签</RouterLink>
      </header>
      <div class="tag-cloud">
        <TagPill
          v-for="tag in tags.slice(0, 16)"
          :key="tag.id"
          :tag="tag"
          :count="tag.postCount"
        />
      </div>
    </section>
  </div>
</template>

<style scoped>
.home {
  max-width: var(--container-xl);
  margin: 0 auto;
  padding: 0 var(--space-lg);
}

.hero {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-md);
  padding: var(--space-4xl) 0 var(--space-3xl);
}

.hero__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-5xl);
  font-weight: 600;
  letter-spacing: var(--tracking-tight);
  line-height: var(--leading-tight);
}

.hero__tagline {
  max-width: var(--measure-narrow);
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--text-lg);
  line-height: var(--leading-relaxed);
}

.hero__actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  margin-top: var(--space-sm);
}

.hero__stats {
  display: flex;
  gap: var(--space-3xl);
  margin: var(--space-xl) 0 0;
}

.hero__stat {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
}

.hero__stat dt {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-caps);
  text-transform: uppercase;
}

.hero__stat dd {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  font-variant-numeric: tabular-nums;
}

.section {
  margin-top: var(--space-3xl);
}

.section__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-lg);
  padding-bottom: var(--space-sm);
  border-bottom: 1px solid var(--border-subtle);
}

.section__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-2xl);
}

.section__more {
  color: var(--accent);
  font-size: var(--text-sm);
  text-decoration: none;
}

.section__more:hover {
  text-decoration: underline;
}

/* Asymmetric bento: the lead item spans both columns and both rows. */
.bento {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-lg);
}

.bento__lead {
  grid-column: 1 / -1;
}

.skeleton-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-lg);
}

.skeleton {
  height: 220px;
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

.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
}

@media (prefers-reduced-motion: reduce) {
  .skeleton {
    animation: none;
  }
}

@media (max-width: 900px) {
  .bento {
    grid-template-columns: minmax(0, 1fr);
  }

  .skeleton-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .hero {
    padding-top: var(--space-2xl);
  }

  .hero__stats {
    gap: var(--space-xl);
  }
}
</style>
