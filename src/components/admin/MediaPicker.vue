<script setup lang="ts">
/**
 * Image picker over the media library.
 *
 * Shared by the avatar field and the wallpaper manager so "pick an existing
 * upload" behaves identically everywhere, and so neither has to send the author
 * off to the media page and back. Uploading from inside the picker is
 * deliberate: picking a picture you have not uploaded yet is the common case.
 */
import { computed, ref, watch } from 'vue'
import { NButton, NInput, NModal, NProgress, useMessage } from 'naive-ui'
import { PhCheck, PhUploadSimple } from '@phosphor-icons/vue'
import { listMedia, uploadMedia } from '@/api/media'
import type { MediaAsset } from '@/types/blog'
import { formatBytes } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    show: boolean
    multiple?: boolean
    title?: string
    hint?: string
  }>(),
  {
    multiple: false,
    title: '选择图片',
    hint: '',
  },
)

const emit = defineEmits<{
  'update:show': [value: boolean]
  select: [urls: string[]]
}>()

const PAGE_SIZE = 18
const message = useMessage()

const assets = ref<MediaAsset[]>([])
const total = ref(0)
const page = ref(1)
const keyword = ref('')
const loading = ref(false)
const uploading = ref(false)
const progress = ref(0)
const selected = ref<string[]>([])
const fileInput = ref<HTMLInputElement | null>(null)

const visible = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value),
})

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))

async function load(): Promise<void> {
  loading.value = true
  try {
    const result = await listMedia(page.value, PAGE_SIZE, keyword.value.trim() || undefined)
    assets.value = result.items
    total.value = result.total
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '加载媒体库失败')
    assets.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function search(): void {
  page.value = 1
  void load()
}

function onPage(next: number): void {
  page.value = next
  void load()
}

function isSelected(asset: MediaAsset): boolean {
  return selected.value.includes(asset.url)
}

function choose(asset: MediaAsset): void {
  if (!props.multiple) {
    emit('select', [asset.url])
    visible.value = false
    return
  }

  const index = selected.value.indexOf(asset.url)
  if (index >= 0) selected.value.splice(index, 1)
  else selected.value.push(asset.url)
}

function confirm(): void {
  if (selected.value.length === 0) return
  emit('select', [...selected.value])
  visible.value = false
}

async function handleFiles(files: FileList | null): Promise<void> {
  if (!files || files.length === 0) return

  uploading.value = true
  progress.value = 0
  try {
    for (const file of Array.from(files)) {
      const asset = await uploadMedia(file, (percent) => {
        progress.value = percent
      })
      // Newly uploaded images are almost always the ones being picked, so they
      // start selected instead of forcing a second click on the fresh tile.
      if (asset) selected.value.push(asset.url)
    }
    page.value = 1
    keyword.value = ''
    await load()
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '上传失败')
  } finally {
    uploading.value = false
    progress.value = 0
    if (fileInput.value) fileInput.value.value = ''
  }
}

watch(
  () => props.show,
  (open) => {
    if (!open) return
    selected.value = []
    page.value = 1
    void load()
  },
)
</script>

<template>
  <NModal
    v-model:show="visible"
    preset="card"
    class="picker"
    :title="title"
    :style="{ maxWidth: '920px' }"
    :bordered="false"
    size="small"
  >
    <div class="picker__body">
      <header class="picker__bar">
        <NInput
          v-model:value="keyword"
          clearable
          placeholder="按文件名搜索"
          @keydown.enter="search"
          @clear="search"
        />
        <NButton @click="search">搜索</NButton>
        <input
          ref="fileInput"
          class="picker__input"
          type="file"
          accept="image/*"
          multiple
          @change="handleFiles(($event.target as HTMLInputElement).files)"
        />
        <NButton secondary :loading="uploading" @click="fileInput?.click()">
          <template #icon><PhUploadSimple :size="16" /></template>
          上传
        </NButton>
      </header>

      <NProgress v-if="uploading" type="line" :percentage="progress" :show-indicator="false" processing />

      <p v-if="hint" class="picker__hint">{{ hint }}</p>
      <p v-if="loading" class="picker__state">正在加载…</p>
      <p v-else-if="assets.length === 0" class="picker__state">媒体库里还没有图片，先上传一张吧。</p>

      <div v-else class="picker__grid">
        <button
          v-for="asset in assets"
          :key="asset.id"
          type="button"
          class="picker__tile"
          :class="{ 'is-selected': isSelected(asset) }"
          :title="`${asset.filename} · ${formatBytes(asset.size)}`"
          @click="choose(asset)"
        >
          <img :src="asset.url" :alt="asset.filename" loading="lazy" decoding="async" />
          <span v-if="isSelected(asset)" class="picker__check" aria-hidden="true">
            <PhCheck :size="13" weight="bold" />
          </span>
          <span class="picker__name">{{ asset.filename }}</span>
        </button>
      </div>

      <nav v-if="pageCount > 1" class="picker__pager">
        <NButton size="small" :disabled="page <= 1" @click="onPage(page - 1)">上一页</NButton>
        <span class="picker__page">{{ page }} / {{ pageCount }}</span>
        <NButton size="small" :disabled="page >= pageCount" @click="onPage(page + 1)">下一页</NButton>
      </nav>
    </div>

    <template v-if="multiple" #footer>
      <div class="picker__footer">
        <span class="picker__count">已选 {{ selected.length }} 张</span>
        <NButton type="primary" :disabled="selected.length === 0" @click="confirm">确定</NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.picker__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.picker__bar {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
}

.picker__bar > :first-child {
  flex: 1;
  min-width: 180px;
}

.picker__input {
  display: none;
}

.picker__hint,
.picker__state {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.picker__state {
  padding: var(--space-xl);
  text-align: center;
}

.picker__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: var(--space-sm);
  max-height: 52vh;
  overflow-y: auto;
  padding: 2px;
}

.picker__tile {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-inset);
  color: var(--text-secondary);
  cursor: pointer;
  transition:
    border-color var(--dur-fast) var(--ease-out),
    transform var(--dur-fast) var(--ease-out);
}

.picker__tile:hover {
  border-color: var(--accent-border);
  transform: translateY(-1px);
}

.picker__tile.is-selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-subtle);
}

.picker__tile img {
  width: 100%;
  aspect-ratio: 4 / 3;
  object-fit: cover;
  background: var(--bg-subtle);
}

.picker__check {
  position: absolute;
  top: 4px;
  right: 4px;
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  border-radius: var(--radius-full);
  background: var(--accent);
  color: var(--text-inverse);
}

.picker__name {
  padding: 0 var(--space-2xs) var(--space-2xs);
  overflow: hidden;
  font-size: var(--text-2xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.picker__pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-sm);
}

.picker__page {
  color: var(--text-muted);
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
}

.picker__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.picker__count {
  color: var(--text-muted);
  font-size: var(--text-xs);
}
</style>
