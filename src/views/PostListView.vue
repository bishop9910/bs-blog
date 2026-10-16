<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { listPublished, listTags } from '@/api/posts'
import type { PostSummary, Tag } from '@/types/blog'
import { motionStagger, reduceMotion, utils } from '@/utils/anime'
import EmptyState from '@/components/EmptyState.vue'
import PostCard from '@/components/PostCard.vue'
import TagPill from '@/components/TagPill.vue'
import { usePageReady } from '@/composables/usePageTransition'

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
  // Registered where the work starts, not in setup: paging keeps this
  // component alive, so the curtain would otherwise lift before the new page of
  // posts exists.
  const pageDone = usePageReady()
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
    pageDone()
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

const grid = ref<HTMLElement | null>(null)

/** Cards past this point appear with the others instead of trailing the wave. */
const CARD_WAVE_LIMIT = 12

let cardAnim: ReturnType<typeof motionStagger> = null

/**
 * Stagger in the current page of cards.
 *
 * The cards are taken over from `v-reveal` (which PostCard owns) by dropping
 * `.reveal`: that class is `opacity: 0` plus a CSS transition, and a CSS
 * transition on the same properties smears anime.js's per-frame writes. The
 * start state is then written by `utils.set` in the same frame — never from CSS
 * and only after the reduced-motion check, so if the animation never runs the
 * list is simply visible.
 */
function playCards() {
  // The previous wave may still be running on nodes the loading skeleton removed.
  cardAnim?.cancel()
  const cards = Array.from(grid.value?.querySelectorAll<HTMLElement>('[data-motion-card]') ?? [])
  for (const card of cards) {
    card.removeAttribute('data-motion-card')
    card.classList.remove('reveal')
  }
  if (cards.length === 0 || reduceMotion()) return

  const wave = cards.slice(0, CARD_WAVE_LIMIT)
  utils.set(wave, { opacity: 0, translateY: '22px' })

  const animation = motionStagger(
    wave,
    { opacity: [0, 1], translateY: [22, 0], scale: [0.985, 1] },
    { each: 55 },
  )
  cardAnim = animation
  if (!animation) return

  void animation.then(() => {
    // Clear the inline writes when the wave settles: an inline transform outranks
    // `.card:hover`, so leaving it would kill the hover lift.
    for (const card of wave) {
      card.style.removeProperty('opacity')
      card.style.removeProperty('transform')
    }
  })
}

onBeforeUnmount(() => {
  cardAnim?.cancel()
})

watch(
  () => route.fullPath,
  () => {
    void fetchPage(requestedPage())
  },
  { immediate: true },
)

// Every page turning over re-renders the grid, so the wave follows the data
// rather than mount: `flush: 'post'` guarantees the new cards are in the DOM.
watch(() => posts.value, () => playCards(), { flush: 'post' })

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

    <div v-else ref="grid" class="grid">
      <PostCard v-for="post in posts" :key="post.id" data-motion-card :post="post" />
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
