<script setup lang="ts">
import { onMounted } from 'vue'
import { useTypewriter } from '@/composables/useTypewriter'

const LINES = [
  '这个地址什么都没有。',
  '可能是打错字了。',
  '也可能是我删掉了。',
  '要不回到首页看看?',
]

const { text, start } = useTypewriter(LINES, { speed: 80 })

onMounted(() => {
  start()
})
</script>

<template>
  <div class="not-found">
    <p class="not-found__code" aria-hidden="true">404</p>
    <p class="not-found__line">
      <span class="caret">{{ text }}</span>
    </p>
    <div class="not-found__actions">
      <RouterLink class="btn btn--primary" :to="{ name: 'home' }">回到首页</RouterLink>
      <RouterLink class="btn" :to="{ name: 'posts' }">浏览文章</RouterLink>
    </div>
  </div>
</template>

<style scoped>
.not-found {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-md);
  min-height: 60vh;
  padding: var(--space-3xl) var(--space-lg);
  text-align: center;
}

.not-found__code {
  margin: 0;
  color: var(--accent);
  font-family: var(--font-display);
  font-size: clamp(5rem, 18vw, 11rem);
  font-weight: 600;
  letter-spacing: var(--tracking-tight);
  line-height: 1;
  opacity: 0.16;
}

.not-found__line {
  min-height: 1.6em;
  margin: calc(var(--space-lg) * -1) 0 0;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: var(--text-base);
}

.not-found__actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  justify-content: center;
  margin-top: var(--space-lg);
}
</style>
