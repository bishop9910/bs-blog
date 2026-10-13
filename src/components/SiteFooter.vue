<script setup lang="ts">
import { computed } from 'vue'
import { PhArrowUp } from '@phosphor-icons/vue'
import { useSite } from '@/composables/useSite'
import { formatCount } from '@/utils/format'
import { resetScroll } from '@/composables/useSmoothScroll'
import SocialLinks from './SocialLinks.vue'

const { site } = useSite()

const year = new Date().getFullYear()

const stats = computed(() => {
  const data = site.value?.stats
  if (!data) return []
  return [
    { label: '篇文章', value: formatCount(data.postCount) },
    { label: '个标签', value: formatCount(data.tagCount) },
    { label: '字', value: formatCount(data.wordCount) },
  ]
})

function backToTop() {
  resetScroll(false)
}
</script>

<template>
  <footer class="footer">
    <div class="footer__inner">
      <div class="footer__brand">
        <p class="footer__title">{{ site?.title ?? 'bs-blog' }}</p>
        <p class="footer__note">
          {{ site?.footerNote || site?.description || '一个人的写作空间。' }}
        </p>
        <SocialLinks v-if="site && site.socials.length > 0" :socials="site.socials" />
      </div>

      <dl v-if="stats.length > 0" class="footer__stats">
        <div v-for="item in stats" :key="item.label" class="footer__stat">
          <dt>{{ item.label }}</dt>
          <dd>{{ item.value }}</dd>
        </div>
      </dl>
    </div>

    <div class="footer__bottom">
      <span>© {{ year }} {{ site?.author || site?.title || 'bs-blog' }}</span>
      <span class="footer__built">Vue 3 · Go · SQLite</span>
      <button class="footer__top" type="button" @click="backToTop">
        <PhArrowUp :size="14" weight="bold" />
        回到顶部
      </button>
    </div>
  </footer>
</template>

<style scoped>
.footer {
  margin-top: var(--space-4xl);
  border-top: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--bg-subtle) 60%, transparent);
}

.footer__inner {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2xl);
  align-items: flex-start;
  justify-content: space-between;
  max-width: var(--container-xl);
  margin: 0 auto;
  padding: var(--space-3xl) var(--space-lg) var(--space-xl);
}

.footer__brand {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  max-width: 42ch;
}

.footer__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-2xl);
}

.footer__note {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
}

.footer__stats {
  display: flex;
  gap: var(--space-2xl);
  margin: 0;
}

.footer__stat {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
}

.footer__stat dt {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-caps);
  text-transform: uppercase;
}

.footer__stat dd {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-variant-numeric: tabular-nums;
}

.footer__bottom {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-md);
  max-width: var(--container-xl);
  margin: 0 auto;
  padding: var(--space-md) var(--space-lg) var(--space-2xl);
  border-top: 1px solid var(--border-subtle);
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.footer__built {
  margin-left: auto;
  letter-spacing: var(--tracking-wide);
}

.footer__top {
  display: inline-flex;
  align-items: center;
  gap: var(--space-xs);
  min-height: var(--hit-min);
  padding: 0 var(--space-sm);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  font: inherit;
  cursor: pointer;
  transition:
    color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.footer__top:hover {
  color: var(--accent);
  background: var(--bg-hover);
}

@media (max-width: 640px) {
  .footer__stats {
    gap: var(--space-xl);
  }

  .footer__built {
    display: none;
  }
}
</style>
