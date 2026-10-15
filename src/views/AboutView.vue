<script setup lang="ts">
import { computed } from 'vue'
import { useSite } from '@/composables/useSite'
import { formatCount, formatDate } from '@/utils/format'
import AuthorAvatar from '@/components/AuthorAvatar.vue'
import SocialLinks from '@/components/SocialLinks.vue'

const { site } = useSite()

const stats = computed(() => {
  const data = site.value?.stats
  if (!data) return []
  return [
    { label: '已发布', value: formatCount(data.postCount) },
    { label: '标签', value: formatCount(data.tagCount) },
    { label: '总字数', value: formatCount(data.wordCount) },
  ]
})

const timeline = computed(() => {
  const data = site.value?.stats
  if (!data?.firstPublishedAt) return ''
  const from = formatDate(data.firstPublishedAt)
  const to = data.lastPublishedAt ? formatDate(data.lastPublishedAt) : '至今'
  return `${from} — ${to}`
})

const STACK = [
  { name: 'Vue 3 + Vite', note: '前台与后台界面，TypeScript 严格模式' },
  { name: 'TipTap (ProseMirror)', note: '富文本编辑器，HTML 与 JSON 双份存储' },
  { name: 'Go + Kratos v2', note: '手写路由的 HTTP 服务，单文件二进制' },
  { name: 'SQLite (GORM)', note: 'WAL 模式，零运维的本地数据库' },
]
</script>

<template>
  <div class="page page--narrow">
    <header class="page__head about__head">
      <AuthorAvatar :src="site?.avatarUrl" :name="site?.author" :size="104" ring />
      <div class="about__intro">
        <p class="eyebrow">关于</p>
        <h1 class="page__title">{{ site?.author || site?.title || 'bs-blog' }}</h1>
        <p class="page__lead">{{ site?.description || '一个人的写作空间。' }}</p>
        <p v-if="site?.author && site?.title" class="about__site">{{ site.title }}</p>
      </div>
    </header>

    <section class="block">
      <h2 class="block__title">这里有什么</h2>
      <p class="block__text">
        {{ site?.tagline || '写点东西，然后留下来。' }}
      </p>
      <dl v-if="stats.length > 0" class="stats">
        <div v-for="item in stats" :key="item.label" class="stat">
          <dt>{{ item.label }}</dt>
          <dd>{{ item.value }}</dd>
        </div>
      </dl>
      <p v-if="timeline" class="block__timeline">写作区间 · {{ timeline }}</p>
    </section>

    <section class="block">
      <h2 class="block__title">怎么搭的</h2>
      <ul class="stack">
        <li v-for="item in STACK" :key="item.name" class="stack__item">
          <span class="stack__name">{{ item.name }}</span>
          <span class="stack__note">{{ item.note }}</span>
        </li>
      </ul>
    </section>

    <section v-if="site && site.socials.length > 0" class="block">
      <h2 class="block__title">找到我</h2>
      <SocialLinks :socials="site.socials" :size="22" />
    </section>
  </div>
</template>

<style scoped>
.about__head {
  display: flex;
  align-items: flex-start;
  gap: var(--space-xl);
}

.about__intro {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  min-width: 0;
}

.about__intro .eyebrow {
  margin: 0;
}

.about__site {
  margin: 0;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  letter-spacing: var(--tracking-wide);
}

.block {
  padding: var(--space-xl) 0;
  border-top: 1px solid var(--border-subtle);
}

.block__title {
  margin: 0 0 var(--space-md);
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-xl);
}

.block__text {
  max-width: var(--measure);
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--text-base);
  line-height: var(--leading-relaxed);
}

.block__timeline {
  margin: var(--space-lg) 0 0;
  color: var(--text-muted);
  font-size: var(--text-xs);
  letter-spacing: var(--tracking-wide);
}

.stats {
  display: flex;
  gap: var(--space-3xl);
  margin: var(--space-xl) 0 0;
}

.stat {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
}

.stat dt {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-caps);
  text-transform: uppercase;
}

.stat dd {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  font-variant-numeric: tabular-nums;
}

.stack {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  margin: 0;
  padding: 0;
  list-style: none;
}

.stack__item {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
  padding-left: var(--space-md);
  border-left: 2px solid var(--accent-border);
}

.stack__name {
  color: var(--text-primary);
  font-size: var(--text-base);
  font-weight: 500;
}

.stack__note {
  color: var(--text-muted);
  font-size: var(--text-sm);
}

@media (max-width: 640px) {
  .about__head {
    flex-direction: column;
    gap: var(--space-lg);
  }

  .stats {
    gap: var(--space-xl);
  }
}
</style>
