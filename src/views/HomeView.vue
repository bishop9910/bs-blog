<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { PhArrowRight, PhWarningCircle } from '@phosphor-icons/vue'
import { listPublished, listTags } from '@/api/posts'
import type { PostSummary, Tag } from '@/types/blog'
import { useSite } from '@/composables/useSite'
import { useTypewriter } from '@/composables/useTypewriter'
import { usePageReady } from '@/composables/usePageTransition'
import { formatCount } from '@/utils/format'
import {
  countUp,
  motion,
  motionStagger,
  motionTimeline,
  reduceMotion,
  utils,
} from '@/utils/anime'
import AuthorAvatar from '@/components/AuthorAvatar.vue'
import EmptyState from '@/components/EmptyState.vue'
import PostCard from '@/components/PostCard.vue'
import TagPill from '@/components/TagPill.vue'

const { site } = useSite()

const authorName = computed(() => site.value?.author?.trim() || site.value?.title?.trim() || 'bs-blog')

const HERO_LINES = ['代码、文字，和一些“自我感想”。', '个人观点，有啥说啥', '欢迎随便逛逛。']
const { text: heroText, start: startHero } = useTypewriter(HERO_LINES, { speed: 58, hold: 2000 })

const posts = ref<PostSummary[]>([])
const tags = ref<Tag[]>([])
const loading = ref(true)
const error = ref('')

const lead = computed(() => posts.value[0])
const rest = computed(() => posts.value.slice(1))

interface HeroStat {
  label: string
  /** What the row shows at rest, already formatted (e.g. `1.2 万`). */
  text: string
  /** Numeric source for the count-up, or `null` when the value is not a number. */
  count: number | null
}

/**
 * `count` is kept apart from `text` on purpose: `formatCount` turns large values
 * into unit-bearing strings ("1.2 万"), which cannot be counted up. Those render
 * as-is and are skipped by {@link playStats}.
 */
const statItems = computed<HeroStat[]>(() => {
  const stats = site.value?.stats
  if (!stats) return []
  const items: Array<{ label: string; value: number }> = [
    { label: '文章', value: stats.postCount },
    { label: '标签', value: stats.tagCount },
    { label: '字数', value: stats.wordCount },
  ]
  return items.map((item) => ({
    label: item.label,
    text: formatCount(item.value),
    count: Number.isFinite(item.value) ? item.value : null,
  }))
})

const heroId = ref<HTMLElement | null>(null)
const heroTitle = ref<HTMLElement | null>(null)
const heroTagline = ref<HTMLElement | null>(null)
const heroStatus = ref<HTMLElement | null>(null)
const heroActions = ref<HTMLElement | null>(null)
const heroStats = ref<HTMLElement | null>(null)
const bento = ref<HTMLElement | null>(null)

/** Cards past this point appear with the others instead of trailing the wave. */
const CARD_WAVE_LIMIT = 12

let heroAnim: ReturnType<typeof motionTimeline> = null
let statsAnim: ReturnType<typeof motion> = null
let cardAnim: ReturnType<typeof motionStagger> = null
const countAnims: Array<ReturnType<typeof countUp>> = []

/**
 * Hero entrance: the blocks fade up in reading order so the eye is led into the
 * page instead of the whole section arriving at once. anime.js renders the first
 * keyframe itself, on the same frame as mount — nothing is hidden from CSS, so a
 * page with JS disabled (or reduced motion) shows the full hero immediately.
 */
function playHero() {
  const timeline = motionTimeline()
  if (!timeline) return

  const blocks = [
    heroId.value,
    heroTitle.value,
    heroTagline.value,
    heroStatus.value,
    heroActions.value,
  ]
  let added = 0
  for (const block of blocks) {
    if (!block) continue
    timeline.add(
      block,
      { opacity: [0, 1], translateY: [18, 0], duration: 660, ease: 'outExpo' },
      added === 0 ? 0 : '+=70',
    )
    added += 1
  }
  if (added === 0) return
  heroAnim = timeline
}

/**
 * The stats row is `v-if`-ed on the async profile, so it arrives after the hero
 * timeline has run: it gets its own entrance and one count-up per number.
 */
function playStats() {
  const el = heroStats.value
  if (!el) return

  for (const running of countAnims.splice(0)) running?.cancel()
  statsAnim?.cancel()

  for (const node of el.querySelectorAll<HTMLElement>('[data-count]')) {
    const value = Number(node.dataset.count)
    if (!Number.isFinite(value)) continue
    countAnims.push(countUp(node, value, { format: formatCount }))
  }

  statsAnim = motion(el, {
    opacity: [0, 1],
    translateY: [18, 0],
    duration: 660,
    ease: 'outExpo',
  })
}

/**
 * Take the cards over from `v-reveal`. PostCard owns that directive, so the
 * hand-over happens from the outside: `.reveal` carries `opacity: 0` *and* a CSS
 * transition, which would both hide the cards in a no-JS page and smear
 * anime.js's inline writes across frames. Removing it puts the animation under
 * one owner.
 */
function takeoverCards(): HTMLElement[] {
  const cards = Array.from(bento.value?.querySelectorAll<HTMLElement>('[data-motion-card]') ?? [])
  for (const card of cards) {
    card.removeAttribute('data-motion-card')
    card.classList.remove('reveal')
  }
  return cards
}

/**
 * Staggered card entrance. The start state is written by `utils.set` in the same
 * frame as the take-over — never from CSS — and only after the reduced-motion
 * check, so the worst case (animation throws, or JS never runs) is a visible list.
 */
function playCards() {
  // A reload of the feed can hand over nodes while the previous wave still runs.
  cardAnim?.cancel()
  const cards = takeoverCards()
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
    // Drop the inline writes once the wave settles: an inline transform outranks
    // `.card:hover`, so leaving it would kill the hover lift for good.
    for (const card of wave) {
      card.style.removeProperty('opacity')
      card.style.removeProperty('transform')
    }
  })
}

onBeforeUnmount(() => {
  heroAnim?.cancel()
  statsAnim?.cancel()
  cardAnim?.cancel()
  for (const running of countAnims.splice(0)) running?.cancel()
})

// Cards are rendered by the async list, so the wave follows the data, not mount.
watch(() => posts.value, () => playCards(), { flush: 'post' })
// The profile lands later and brings the stats row with it.
watch(statItems, () => playStats(), { flush: 'post' })

onMounted(async () => {
  playHero()
  startHero()
  // The headline and the typewriter are worth arriving to, so the blade waits
  // for the posts as well — held to the ceiling if the API is having a day.
  const pageDone = usePageReady()
  // When the profile was already cached the stats row exists at mount, and the
  // watcher above will not fire — the count-up still has to run.
  void nextTick(playStats)
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
    pageDone()
  }
})
</script>

<template>
  <div class="home">
    <section class="hero">
      <div ref="heroId" class="hero__id">
        <AuthorAvatar :src="site?.avatarUrl" :name="authorName" :size="88" ring />
        <div class="hero__id-text">
          <p class="eyebrow">个人博客</p>
          <p v-if="site?.author" class="hero__author">{{ site.author }}</p>
        </div>
      </div>

      <h1 ref="heroTitle" class="hero__title">{{ site?.title ?? 'bs-blog' }}</h1>
      <p ref="heroTagline" class="hero__tagline">{{ site?.tagline || '一个人的写作空间。' }}</p>

      <p ref="heroStatus" class="hero__status">
        <span class="hero__prompt" aria-hidden="true">❯</span>
        <span class="hero__typed">{{ heroText }}</span>
        <span class="hero__caret" aria-hidden="true" />
      </p>

      <div ref="heroActions" class="hero__actions">
        <RouterLink class="btn btn--primary" :to="{ name: 'posts' }">
          开始阅读
          <PhArrowRight :size="16" weight="bold" />
        </RouterLink>
        <RouterLink class="btn" :to="{ name: 'archive' }">按时间浏览</RouterLink>
      </div>

      <dl v-if="statItems.length > 0" ref="heroStats" class="hero__stats">
        <div v-for="item in statItems" :key="item.label" class="hero__stat">
          <dt>{{ item.label }}</dt>
          <dd :data-count="item.count ?? undefined">{{ item.text }}</dd>
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

      <div v-else ref="bento" class="bento">
        <PostCard data-motion-card class="bento__lead" :post="lead" featured />
        <PostCard
          v-for="post in rest"
          :key="post.id"
          data-motion-card
          class="bento__item"
          :post="post"
        />
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

.hero__id {
  display: flex;
  align-items: center;
  gap: var(--space-md);
}

.hero__id-text {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
  min-width: 0;
}

.hero__id-text .eyebrow {
  margin: 0;
}

.hero__author {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-xl);
  letter-spacing: var(--tracking-tight);
  line-height: var(--leading-snug);
}

.hero__status {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  min-height: 1.5em;
  margin: 0;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: var(--text-sm);
}

.hero__prompt {
  color: var(--accent);
}

.hero__caret {
  display: inline-block;
  width: 8px;
  height: 1.05em;
  background: var(--accent);
  opacity: 0.85;
  animation: hero-caret 1.05s steps(1, end) infinite;
}

@keyframes hero-caret {
  0%,
  49% {
    opacity: 0.85;
  }

  50%,
  100% {
    opacity: 0;
  }
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

  .hero__caret {
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
