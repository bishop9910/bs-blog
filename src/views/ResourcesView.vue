<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { listPublicResources, resourceDownloadUrl } from '@/api/resources'
import type { ResourceFile } from '@/types/resource'
import { formatBytes, formatDate } from '@/utils/format'
import EmptyState from '@/components/EmptyState.vue'
import { usePageReady } from '@/composables/usePageTransition'

const PAGE_SIZE = 12

const items = ref<ResourceFile[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(true)
const error = ref('')
const keyword = ref('')

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))

async function fetchPage(target: number) {
  // Registered where the work starts so the route curtain stays down until the
  // files are actually there — the same contract the article list uses.
  const pageDone = usePageReady()
  loading.value = true
  error.value = ''
  try {
    const result = await listPublicResources({
      page: target,
      pageSize: PAGE_SIZE,
      keyword: keyword.value.trim() || undefined,
    })
    items.value = result.items
    total.value = result.total
    page.value = result.page
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : '加载失败'
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
    pageDone()
  }
}

function search() {
  void fetchPage(1)
}

function goTo(target: number) {
  const clamped = Math.min(Math.max(1, target), totalPages.value)
  if (clamped === page.value) return
  void fetchPage(clamped)
}

/** The extension badge reads as a label, so it is uppercased: "ZIP", "APK". */
function badge(item: ResourceFile): string {
  return item.extension ? item.extension.toUpperCase() : 'FILE'
}

onMounted(() => void fetchPage(1))
</script>

<template>
  <div class="page">
    <header class="page__head">
      <p class="eyebrow">可以直接拿走的东西</p>
      <h1 class="page__title">资源</h1>
      <p v-if="!loading && !error && total > 0" class="page__count">{{ total }} 个文件</p>
    </header>

    <form class="search" role="search" @submit.prevent="search">
      <input
        v-model="keyword"
        class="input"
        type="search"
        placeholder="按标题或文件名搜索"
        aria-label="搜索资源"
      />
      <button class="btn btn--sm" type="submit">搜索</button>
    </form>

    <div v-if="loading" class="list">
      <div v-for="n in 5" :key="n" class="skeleton" />
    </div>

    <EmptyState v-else-if="error" :title="error" description="请确认后端已启动，然后重试。" />

    <EmptyState
      v-else-if="items.length === 0"
      :title="keyword ? '没有匹配的文件' : '还没有可下载的文件'"
      description="上传的项目和笔记会出现在这里，稍后再来看看。"
    />

    <ul v-else class="list">
      <li
        v-for="(item, index) in items"
        :key="item.id"
        class="row"
        v-reveal="{ delay: Math.min(index, 8) * 45 }"
      >
        <img
          v-if="item.coverUrl"
          class="row__cover"
          :src="item.coverUrl"
          :alt="item.title"
          loading="lazy"
          decoding="async"
        />
        <span v-else class="row__badge" aria-hidden="true">{{ badge(item) }}</span>

        <div class="row__body">
          <h2 class="row__title">{{ item.title }}</h2>
          <p v-if="item.description" class="row__desc">{{ item.description }}</p>
          <p class="row__meta">
            <span class="row__file" :title="item.filename">{{ item.filename }}</span>
            <span>{{ formatBytes(item.size) }}</span>
            <span>{{ formatDate(item.createdAt) }}</span>
            <span>{{ item.downloadCount }} 次下载</span>
          </p>
        </div>

        <a class="btn btn--sm btn--primary row__get" :href="resourceDownloadUrl(item.id)">
          下载
        </a>
      </li>
    </ul>

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
.search {
  display: flex;
  gap: var(--space-sm);
  margin-bottom: var(--space-xl);
}

.search .input {
  flex: 1;
  max-width: 360px;
}

.list {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  margin: 0;
  padding: 0;
  list-style: none;
}

.row {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-md);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  transition:
    border-color var(--dur-fast) var(--ease-out),
    transform var(--dur-fast) var(--ease-out);
}

.row:hover {
  border-color: var(--border-strong);
  transform: translateY(-1px);
}

.row__cover,
.row__badge {
  flex: none;
  width: 56px;
  height: 56px;
  border-radius: var(--radius-md);
  background: var(--bg-subtle);
}

.row__cover {
  object-fit: cover;
}

.row__badge {
  display: grid;
  place-items: center;
  border: 1px solid var(--border-subtle);
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: var(--text-2xs);
  letter-spacing: 0.04em;
}

.row__body {
  min-width: 0;
  flex: 1;
}

.row__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-lg);
  line-height: 1.3;
}

.row__desc {
  margin: 0.25em 0 0;
  color: var(--text-secondary);
  font-size: var(--text-sm);
  line-height: 1.6;
}

.row__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: var(--space-sm);
  margin: 0.4em 0 0;
  color: var(--text-muted);
  font-size: var(--text-2xs);
  font-variant-numeric: tabular-nums;
}

.row__file {
  max-width: 22ch;
  overflow: hidden;
  font-family: var(--font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__get {
  flex: none;
  text-decoration: none;
}

.skeleton {
  height: 88px;
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

@media (max-width: 640px) {
  .row {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .row__get {
    margin-left: auto;
  }
}
</style>
