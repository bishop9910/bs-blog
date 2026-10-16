<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NButton, NInput, NPopconfirm, NProgress, useMessage } from 'naive-ui'
import { PhCopy, PhTrash, PhUploadSimple } from '@phosphor-icons/vue'
import { deleteMedia, listMedia, uploadMedia } from '@/api/media'
import { useLimits } from '@/composables/useLimits'
import type { MediaAsset } from '@/types/blog'
import { formatBytes, formatDate } from '@/utils/format'
import { describeImageTypes, describeSizeLimit } from '@/utils/limits'

const PAGE_SIZE = 24

const message = useMessage()

// The accepted types and the size cap come from GET /admin/limits, so editing
// config.yaml changes what this page promises as well as what the server does.
const { limits, load: loadLimits } = useLimits()
const acceptAttr = computed(() => limits.value.media.allowedTypes.join(','))
const dropHint = computed(() => {
  const types = describeImageTypes(limits.value.media.allowedTypes)
  const size = describeSizeLimit(limits.value.media.maxUploadBytes)
  return types ? `支持 ${types}，${size}` : size
})

const assets = ref<MediaAsset[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const keyword = ref('')
const uploading = ref(false)
const progress = ref(0)
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

async function load() {
  loading.value = true
  try {
    const result = await listMedia(page.value, PAGE_SIZE, keyword.value.trim() || undefined)
    assets.value = result.items
    total.value = result.total
    page.value = result.page
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '加载失败')
    assets.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

async function handleFiles(files: FileList | null) {
  if (!files || files.length === 0) return
  uploading.value = true
  progress.value = 0
  let uploaded = 0
  try {
    for (const file of Array.from(files)) {
      // Reject locally what the server would reject anyway: uploading a 60 MB
      // photo just to read "图片不能超过 10 MB" wastes the whole transfer.
      if (file.size > limits.value.media.maxUploadBytes) {
        message.warning(
          `${file.name} 超过 ${describeSizeLimit(limits.value.media.maxUploadBytes)}，已跳过`,
        )
        continue
      }
      const asset = await uploadMedia(file, (percent) => {
        progress.value = percent
      })
      if (asset) uploaded += 1
    }
    if (uploaded > 0) {
      message.success(`已上传 ${uploaded} 个文件`)
      page.value = 1
      await load()
    }
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '上传失败')
  } finally {
    uploading.value = false
    progress.value = 0
    if (fileInput.value) fileInput.value.value = ''
  }
}

function onDrop(event: DragEvent) {
  dragOver.value = false
  void handleFiles(event.dataTransfer?.files ?? null)
}

async function copyUrl(asset: MediaAsset) {
  const absolute = new URL(asset.url, window.location.origin).href
  try {
    await navigator.clipboard.writeText(absolute)
    message.success('地址已复制')
  } catch {
    message.warning('浏览器拒绝了剪贴板访问，请手动复制')
  }
}

async function remove(asset: MediaAsset) {
  try {
    await deleteMedia(asset.id)
    message.success('已删除')
    if (assets.value.length === 1 && page.value > 1) page.value -= 1
    await load()
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '删除失败')
  }
}

onMounted(() => {
  void load()
  void loadLimits()
})
</script>

<template>
  <div class="media">
    <header class="toolbar">
      <div class="toolbar__search">
        <NInput
          v-model:value="keyword"
          clearable
          placeholder="按文件名搜索"
          @keydown.enter="search"
          @clear="search"
        />
        <NButton @click="search">搜索</NButton>
      </div>
      <span class="toolbar__count">共 {{ total }} 个文件</span>
    </header>

    <div
      class="drop"
      :class="{ 'is-over': dragOver, 'is-busy': uploading }"
      @dragover.prevent="dragOver = true"
      @dragleave.prevent="dragOver = false"
      @drop.prevent="onDrop"
    >
      <input
        ref="fileInput"
        class="drop__input"
        type="file"
        :accept="acceptAttr"
        multiple
        @change="handleFiles(($event.target as HTMLInputElement).files)"
      />
      <PhUploadSimple :size="28" weight="bold" />
      <p class="drop__text">把图片拖到这里，或<button class="drop__pick" type="button" @click="fileInput?.click()">选择文件</button></p>
      <p class="drop__hint">{{ dropHint }}</p>
      <NProgress
        v-if="uploading"
        class="drop__progress"
        type="line"
        :percentage="progress"
        :show-indicator="false"
        processing
      />
    </div>

    <p v-if="loading" class="state">正在加载…</p>
    <p v-else-if="assets.length === 0" class="state">还没有上传任何文件。</p>

    <div v-else class="grid">
      <figure v-for="asset in assets" :key="asset.id" class="tile">
        <div class="tile__thumb">
          <img :src="asset.url" :alt="asset.filename" loading="lazy" decoding="async" />
        </div>
        <figcaption class="tile__meta">
          <span class="tile__name" :title="asset.filename">{{ asset.filename }}</span>
          <span class="tile__sub">
            {{ formatBytes(asset.size) }}
            <template v-if="asset.width"> · {{ asset.width }}×{{ asset.height }}</template>
          </span>
          <span class="tile__sub">{{ formatDate(asset.createdAt) }}</span>
        </figcaption>
        <div class="tile__actions">
          <NButton size="tiny" secondary @click="copyUrl(asset)">
            <template #icon><PhCopy :size="14" /></template>
            复制地址
          </NButton>
          <NPopconfirm @positive-click="remove(asset)">
            <template #trigger>
              <NButton size="tiny" quaternary type="error" title="删除">
                <template #icon><PhTrash :size="14" /></template>
              </NButton>
            </template>
            删除这张图片?文章里引用它的地方会变成空图。
          </NPopconfirm>
        </div>
      </figure>
    </div>

    <nav v-if="!loading && total > PAGE_SIZE" class="pager">
      <button class="btn btn--sm" type="button" :disabled="page <= 1" @click="page -= 1; load()">
        上一页
      </button>
      <span class="pager__status">{{ page }} / {{ Math.ceil(total / PAGE_SIZE) }}</span>
      <button
        class="btn btn--sm"
        type="button"
        :disabled="page >= Math.ceil(total / PAGE_SIZE)"
        @click="page += 1; load()"
      >
        下一页
      </button>
    </nav>
  </div>
</template>

<style scoped>
.media {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-md);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.toolbar__search {
  display: flex;
  gap: var(--space-sm);
  min-width: 280px;
}

.toolbar__count {
  margin-left: auto;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.drop {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-2xl) var(--space-lg);
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius-lg);
  background: var(--bg-subtle);
  color: var(--text-muted);
  text-align: center;
  transition:
    border-color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.drop.is-over {
  border-color: var(--accent);
  background: var(--accent-subtle);
  color: var(--accent);
}

.drop.is-busy {
  pointer-events: none;
  opacity: 0.8;
}

.drop__input {
  display: none;
}

.drop__text {
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--text-sm);
}

.drop__pick {
  padding: 0;
  border: none;
  background: none;
  color: var(--accent);
  font: inherit;
  text-decoration: underline;
  cursor: pointer;
}

.drop__hint {
  margin: 0;
  font-size: var(--text-2xs);
}

.drop__progress {
  width: min(320px, 100%);
  margin-top: var(--space-sm);
}

.state {
  padding: var(--space-2xl);
  color: var(--text-muted);
  font-size: var(--text-sm);
  text-align: center;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: var(--space-md);
}

.tile {
  display: flex;
  flex-direction: column;
  margin: 0;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  overflow: hidden;
}

.tile__thumb {
  aspect-ratio: 4 / 3;
  background:
    repeating-conic-gradient(var(--bg-inset) 0% 25%, var(--bg-subtle) 0% 50%) 50% / 16px 16px;
}

.tile__thumb img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.tile__meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: var(--space-sm);
  min-width: 0;
}

.tile__name {
  color: var(--text-primary);
  font-size: var(--text-xs);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tile__sub {
  color: var(--text-muted);
  font-size: var(--text-2xs);
}

.tile__actions {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-sm);
  border-top: 1px solid var(--border-subtle);
}
</style>
