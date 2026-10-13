<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { listPublished, listTags } from '@/api/posts'
import type { PostSummary, Tag } from '@/types/blog'
import EmptyState from '@/components/EmptyState.vue'
import PostCard from '@/components/PostCard.vue'
import TagPill from '@/components/TagPill.vue'

const route = useRoute()
const router = useRouter()

const PAGE_SIZE = 9

const posts = ref<PostSummary[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(true)
const error = ref('')
const tags = ref<Tag[]>([])

const activeTag = computed(() =>
  typeof route.params.slug === 'string' ? route.params.slug : '',
)

const activeTagName = computed(
  () => tags.value.find((item) => item.slug === activeTag.value)?.name ?? activeTag.value,
)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))

const title = computed(() => (activeTag.value ? `# ${activeTagName.value}` : '文章'))

async function fetchPage(target: number) {
  loading.value = true
  error.value = ''
  try {
    const result = await listPublished({
      page: target,
      pageSize: PAGE_SIZE,
      tag: activeTag.value || undefined,
      sort: 'published',
      order: 'desc',
    })
    posts.value = result.items
    total.value = result.total
    page.value = result.page
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : '加载失败'
    posts.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function requestedPage(): number {
  const raw = Number(route.query.page ?? 1)
  return Number.isFinite(raw) && raw > 0 ? Math.floor(raw) : 1
}

function goTo(target: number) {
  const clamped = Math.min(Math.max(1, target), totalPages.value)
  if (clamped === page.value) return
  void router.push({ query: { ...route.query, page: String(clamped) } })
}

watch(
  () => route.fullPath,
  () => {
    void fetchPage(requestedPage())
  },
  { immediate: true },
)

onMounted(async () => {
  try {
    const result = await listTags()
    tags.value = result.tags
  } catch {
    // The tag strip is a nicety; the list itself still works without it.
  }
})
</script>

<template>
  <div class="page">
    <header class="page__head">
      <p class="eyebrow">{{ activeTag ? '标签' : '归档于时间线之外' }}</p>
      <h1 class="page__title">{{ title }}</h1>
      <p v-if="!loading && !error" class="page__count">{{ total }} 篇文章</p>
    </header>

    <nav v-if="tags.length > 0" class="strip" aria-label="按标签筛选">
      <RouterLink class="strip__all" :class="{ 'is-active': !activeTag }" :to="{ name: 'posts' }">
        全部
      </RouterLink>
      <TagPill
        v-for="tag in tags"
        :key="tag.id"
        :tag="tag"
        :count="tag.postCount"
        :active="tag.slug === activeTag"
      />
    </nav>

    <div v-if="loading" class="grid">
      <div v-for="n in 6" :key="n" class="skeleton" />
    </div>

    <EmptyState v-else-if="error" :title="error" description="请确认后端已启动，然后重试。" />

    <EmptyState
      v-else-if="posts.length === 0"
      :title="activeTag ? '这个标签下还没有文章' : '还没有文章'"
      description="换个标签，或者稍后再来。"
    />

    <div v-else class="grid">
      <PostCard v-for="post in posts" :key="post.id" :post="post" />
    </div>

    <nav v-if="!loading && totalPages > 1" class="pager" aria-label="分页">
      <button class="btn btn--sm" type="button" :disabled="page <= 1" @click="goTo(page - 1)">
        上一页
      </button>
      <span class="pager__status">{{ page }} / {{ totalPages }}</span>
      <button
        class="btn btn--sm"
        type="button"
        :disabled="page >= totalPages"
        @click="goTo(page + 1)"
      >
        下一页
      </button>
    </nav>
  </div>
</template>

<style scoped>
.strip {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-sm);
  margin-bottom: var(--space-xl);
}

.strip__all {
  padding: 0.3em 0.7em;
  border: 1px solid transparent;
  border-radius: var(--radius-full);
  color: var(--text-secondary);
  font-size: var(--text-xs);
  text-decoration: none;
}

.strip__all:hover {
  color: var(--accent);
}

.strip__all.is-active {
  border-color: var(--accent);
  background: var(--accent-subtle);
  color: var(--accent);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: var(--space-lg);
}

.skeleton {
  height: 260px;
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
