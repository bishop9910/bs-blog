<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  PhArrowLeft,
  PhArrowRight,
  PhArrowUp,
  PhCheck,
  PhLink,
} from '@phosphor-icons/vue'
import { useRoute } from 'vue-router'
import { getPublished } from '@/api/posts'
import { ApiError } from '@/api/http'
import type { PostDetail } from '@/types/blog'
import { useHighlightedHtml } from '@/composables/useHighlight'
import { useSite } from '@/composables/useSite'
import { resetScroll } from '@/composables/useSmoothScroll'
import { sanitizeHtml } from '@/utils/sanitize'
import { formatCount, formatDateLong } from '@/utils/format'
import AuthorAvatar from '@/components/AuthorAvatar.vue'
import EmptyState from '@/components/EmptyState.vue'
import SocialLinks from '@/components/SocialLinks.vue'
import TagPill from '@/components/TagPill.vue'

const route = useRoute()
const { site } = useSite()

const authorName = computed(() => site.value?.author?.trim() || site.value?.title?.trim() || '')

const detail = ref<PostDetail | null>(null)
const loading = ref(true)
const notFound = ref(false)
const error = ref('')
const progress = ref(0)
const copied = ref(false)

const post = computed(() => detail.value?.post ?? null)
const nav = computed(() => detail.value?.nav ?? {})

const content = useHighlightedHtml(() => sanitizeHtml(post.value?.contentHtml ?? ''))

function onScroll() {
  const root = document.documentElement
  const max = root.scrollHeight - root.clientHeight
  progress.value = max > 0 ? Math.min(1, root.scrollTop / max) : 0
}

async function load(slug: string) {
  loading.value = true
  notFound.value = false
  error.value = ''
  try {
    const result = await getPublished(slug)
    detail.value = result
    document.title = `${result.post.title} · bs-blog`
  } catch (caught) {
    detail.value = null
    if (caught instanceof ApiError && caught.status === 404) {
      notFound.value = true
    } else {
      error.value = caught instanceof Error ? caught.message : '加载失败'
    }
  } finally {
    loading.value = false
  }
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(window.location.href)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1800)
  } catch {
    // Clipboard access needs a secure context and permission; nothing to do.
  }
}

watch(
  () => route.params.slug,
  (slug) => {
    if (typeof slug === 'string' && slug) void load(slug)
  },
  { immediate: true },
)

onMounted(() => {
  onScroll()
  window.addEventListener('scroll', onScroll, { passive: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll)
})
</script>

<template>
  <article class="post">
    <div class="progress" aria-hidden="true">
      <span class="progress__bar" :style="{ transform: `scaleX(${progress})` }" />
    </div>

    <div v-if="loading" class="post__inner">
      <div class="skeleton skeleton--title" />
      <div class="skeleton skeleton--line" />
      <div class="skeleton skeleton--body" />
    </div>

    <EmptyState
      v-else-if="notFound"
      title="这篇文章不存在"
      description="它可能已被删除，或者还没有发布。"
    >
      <RouterLink class="btn btn--primary" :to="{ name: 'posts' }">看看其他文章</RouterLink>
    </EmptyState>

    <EmptyState v-else-if="error" :title="error" description="请确认后端已启动，然后重试。" />

    <template v-else-if="post">
      <header class="post__head">
        <div v-if="post.tags.length > 0" class="post__tags">
          <TagPill v-for="tag in post.tags" :key="tag.id" :tag="tag" />
        </div>

        <h1 class="post__title">{{ post.title }}</h1>

        <p v-if="post.summary" class="post__summary">{{ post.summary }}</p>

        <div class="post__meta">
          <RouterLink v-if="authorName" class="post__author" :to="{ name: 'about' }">
            <AuthorAvatar :src="site?.avatarUrl" :name="authorName" :size="22" />
            <span>{{ authorName }}</span>
          </RouterLink>
          <span v-if="authorName" aria-hidden="true">·</span>
          <time :datetime="post.publishedAt ?? post.createdAt">
            {{ formatDateLong(post.publishedAt ?? post.createdAt) }}
          </time>
          <span aria-hidden="true">·</span>
          <span>{{ post.readingMinutes }} 分钟阅读</span>
          <span aria-hidden="true">·</span>
          <span>{{ formatCount(post.wordCount) }} 字</span>
          <span v-if="post.viewCount > 0" aria-hidden="true">·</span>
          <span v-if="post.viewCount > 0">{{ formatCount(post.viewCount) }} 次浏览</span>
        </div>
      </header>

      <!-- Sanitised on the server, again in sanitizeHtml, then re-highlighted. -->
      <div class="prose post__body" v-html="content" />

      <footer class="post__foot">
        <section v-if="authorName" class="post__author-card">
          <AuthorAvatar :src="site?.avatarUrl" :name="authorName" :size="56" />
          <div class="post__author-body">
            <p class="post__author-name">{{ authorName }}</p>
            <p v-if="site?.tagline" class="post__author-note">{{ site.tagline }}</p>
          </div>
          <SocialLinks
            v-if="site && site.socials.length > 0"
            class="post__author-socials"
            :socials="site.socials"
            :size="18"
          />
        </section>

        <div class="post__actions">
          <button class="btn btn--sm" type="button" @click="copyLink">
            <component :is="copied ? PhCheck : PhLink" :size="16" />
            {{ copied ? '已复制' : '复制链接' }}
          </button>
          <button class="btn btn--sm" type="button" @click="resetScroll(false)">
            <PhArrowUp :size="16" />
            回到顶部
          </button>
        </div>

        <nav v-if="nav.prev || nav.next" class="post__nav" aria-label="相邻文章">
          <RouterLink
            v-if="nav.prev"
            class="post__nav-link post__nav-link--prev"
            :to="{ name: 'post', params: { slug: nav.prev.slug } }"
          >
            <span class="post__nav-label"><PhArrowLeft :size="14" /> 上一篇</span>
            <span class="post__nav-title">{{ nav.prev.title }}</span>
          </RouterLink>
          <span v-else class="post__nav-spacer" />

          <RouterLink
            v-if="nav.next"
            class="post__nav-link post__nav-link--next"
            :to="{ name: 'post', params: { slug: nav.next.slug } }"
          >
            <span class="post__nav-label">下一篇 <PhArrowRight :size="14" /></span>
            <span class="post__nav-title">{{ nav.next.title }}</span>
          </RouterLink>
        </nav>
      </footer>
    </template>
  </article>
</template>

<style scoped>
.progress {
  position: fixed;
  top: 0;
  left: 0;
  z-index: var(--z-overlay);
  width: 100%;
  height: 2px;
  pointer-events: none;
}

.progress__bar {
  display: block;
  width: 100%;
  height: 100%;
  background: var(--accent);
  box-shadow: 0 0 10px var(--glow-accent);
  transform: scaleX(0);
  transform-origin: left center;
}

.post__inner,
.post__head,
.post__body,
.post__foot {
  max-width: var(--measure);
  margin: 0 auto;
  padding: 0 var(--space-lg);
}

.post__inner {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  padding-top: var(--space-3xl);
}

.post__head {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  padding-top: var(--space-3xl);
  padding-bottom: var(--space-2xl);
}

.post__tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-xs);
}

.post__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-4xl);
  font-weight: 600;
  letter-spacing: var(--tracking-tight);
  line-height: var(--leading-tight);
}

.post__summary {
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--text-lg);
  line-height: var(--leading-relaxed);
}

.post__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-xs);
  padding-top: var(--space-sm);
  border-top: 1px solid var(--border-subtle);
  color: var(--text-muted);
  font-size: var(--text-xs);
  letter-spacing: var(--tracking-wide);
}

.post__author {
  display: inline-flex;
  align-items: center;
  gap: var(--space-xs);
  color: var(--text-secondary);
  text-decoration: none;
  transition: color var(--dur-fast) var(--ease-out);
}

.post__author:hover {
  color: var(--accent);
}

.post__author-card {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-lg);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.post__author-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
  min-width: 0;
}

.post__author-name {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-base);
}

.post__author-note {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-sm);
}

.post__author-socials {
  margin-left: auto;
}

.post__body {
  padding-bottom: var(--space-2xl);
}

.post__foot {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xl);
  padding-bottom: var(--space-3xl);
  border-top: 1px solid var(--border-subtle);
  padding-top: var(--space-lg);
}

.post__actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
}

.post__nav {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md);
}

.post__nav-link {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
  padding: var(--space-md);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-card);
  text-decoration: none;
  transition:
    border-color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.post__nav-link:hover {
  border-color: var(--border-accent);
  background: var(--bg-hover);
}

.post__nav-link--next {
  align-items: flex-end;
  text-align: right;
}

.post__nav-label {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2xs);
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-caps);
  text-transform: uppercase;
}

.post__nav-title {
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-base);
  line-height: var(--leading-snug);
}

.post__nav-spacer {
  display: block;
}

.skeleton {
  border-radius: var(--radius-md);
  background: linear-gradient(
    90deg,
    var(--bg-subtle) 25%,
    var(--bg-hover) 37%,
    var(--bg-subtle) 63%
  );
  background-size: 400% 100%;
  animation: skeleton 1.4s ease-in-out infinite;
}

.skeleton--title {
  height: 48px;
}

.skeleton--line {
  height: 20px;
  width: 60%;
}

.skeleton--body {
  height: 320px;
  margin-top: var(--space-lg);
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

@media (max-width: 640px) {
  .post__title {
    font-size: var(--text-3xl);
  }

  .post__author-card {
    flex-wrap: wrap;
  }

  .post__author-socials {
    margin-left: 0;
  }

  .post__nav {
    grid-template-columns: minmax(0, 1fr);
  }

  .post__nav-spacer {
    display: none;
  }
}
</style>
