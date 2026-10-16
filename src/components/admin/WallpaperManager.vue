<script setup lang="ts">
/**
 * Wallpaper list editor for the site settings page.
 *
 * The wallpapers themselves live in the media library (or anywhere reachable by
 * URL); what is stored on the site profile is just the ordered list of image
 * URLs. Every public page then picks one at random on each load.
 *
 * Ordering is exposed as explicit move buttons rather than drag and drop: it
 * stays keyboard reachable and needs no library.
 */
import { computed, ref } from 'vue'
import { NButton, NInput, NModal, useMessage } from 'naive-ui'
import {
  PhArrowLeft,
  PhArrowRight,
  PhEye,
  PhImage,
  PhLinkSimple,
  PhPlus,
  PhTrash,
} from '@phosphor-icons/vue'
import MediaPicker from './MediaPicker.vue'
import { useLimits } from '@/composables/useLimits'

const props = defineProps<{ modelValue: string[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()

const message = useMessage()

// site.maxWallpapers in config.yaml, read back through GET /admin/limits so the
// counter and the server agree; the fallback covers the request in flight.
const { limits, load: loadLimits } = useLimits()
const maxWallpapers = computed(() => limits.value.site.maxWallpapers)

const pickerOpen = ref(false)
const previewOpen = ref(false)
const previewUrl = ref('')
const draft = ref('')

const list = computed(() => props.modelValue)
const full = computed(() => list.value.length >= maxWallpapers.value)

void loadLimits()

function commit(next: string[]): void {
  // De-duplicate on every write: the same image twice in the rotation is a
  // silent bug the author would only notice as "why does it never change".
  emit('update:modelValue', Array.from(new Set(next)))
}

function addFromLibrary(urls: string[]): void {
  const room = maxWallpapers.value - list.value.length
  if (urls.length > room) {
    message.warning(`最多只能添加 ${maxWallpapers.value} 张壁纸，多出的部分没有加入`)
  }
  commit([...list.value, ...urls.slice(0, Math.max(0, room))])
}

function addDraft(): void {
  const url = draft.value.trim()
  if (!url) return
  if (full.value) {
    message.warning(`最多只能添加 ${maxWallpapers.value} 张壁纸`)
    return
  }
  if (!/^(https?:\/\/|\/)/i.test(url)) {
    message.warning('请填写 /uploads/… 这样的站内路径，或 http(s) 链接')
    return
  }
  commit([...list.value, url])
  draft.value = ''
}

function remove(index: number): void {
  const next = [...list.value]
  next.splice(index, 1)
  commit(next)
}

function move(index: number, delta: number): void {
  const target = index + delta
  if (target < 0 || target >= list.value.length) return
  const next = [...list.value]
  const [item] = next.splice(index, 1)
  if (item === undefined) return
  next.splice(target, 0, item)
  commit(next)
}

function preview(url: string): void {
  previewUrl.value = url
  previewOpen.value = true
}
</script>

<template>
  <div class="wallpapers">
    <p class="wallpapers__hint">
      每次打开站点会从下面随机挑一张作为整站背景。添加的图片同时会留在媒体库里。
      <strong>{{ list.length }} / {{ maxWallpapers }}</strong>
    </p>

    <div class="wallpapers__grid">
      <figure v-for="(url, index) in list" :key="`${url}-${index}`" class="wp">
        <button type="button" class="wp__thumb" :title="`预览第 ${index + 1} 张`" @click="preview(url)">
          <img :src="url" :alt="`壁纸 ${index + 1}`" loading="lazy" decoding="async" />
          <span class="wp__index">{{ index + 1 }}</span>
        </button>

        <figcaption class="wp__actions">
          <NButton size="tiny" quaternary :disabled="index === 0" title="前移" @click="move(index, -1)">
            <template #icon><PhArrowLeft :size="13" /></template>
          </NButton>
          <NButton
            size="tiny"
            quaternary
            :disabled="index === list.length - 1"
            title="后移"
            @click="move(index, 1)"
          >
            <template #icon><PhArrowRight :size="13" /></template>
          </NButton>
          <NButton size="tiny" quaternary title="预览" @click="preview(url)">
            <template #icon><PhEye :size="13" /></template>
          </NButton>
          <NButton size="tiny" quaternary type="error" title="移除" @click="remove(index)">
            <template #icon><PhTrash :size="13" /></template>
          </NButton>
        </figcaption>
      </figure>

      <button type="button" class="wp wp--add" :disabled="full" @click="pickerOpen = true">
        <PhPlus :size="20" weight="bold" aria-hidden="true" />
        <span>{{ full ? '已达上限' : '从媒体库添加' }}</span>
      </button>
    </div>

    <p v-if="list.length === 0" class="wallpapers__empty">
      <PhImage :size="16" weight="duotone" aria-hidden="true" />
      还没有壁纸，站点会继续使用默认的极光背景。
    </p>

    <div class="wallpapers__url">
      <NInput
        v-model:value="draft"
        :maxlength="limits.site.maxWallpaperLength"
        placeholder="/uploads/… 或 https://…"
        @keydown.enter="addDraft"
      >
        <template #prefix><PhLinkSimple :size="15" /></template>
      </NInput>
      <NButton secondary :disabled="full || !draft.trim()" @click="addDraft">添加网址</NButton>
    </div>

    <MediaPicker
      v-model:show="pickerOpen"
      multiple
      title="添加壁纸"
      hint="可以一次选多张，也可以直接上传新图片。"
      @select="addFromLibrary"
    />

    <NModal
      v-model:show="previewOpen"
      preset="card"
      class="wp-preview"
      title="壁纸预览"
      :style="{ maxWidth: '1080px', width: '92vw' }"
      :bordered="false"
      size="small"
    >
      <img v-if="previewUrl" class="wp-preview__img" :src="previewUrl" alt="壁纸预览" />
    </NModal>
  </div>
</template>

<style scoped>
.wallpapers {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.wallpapers__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.wallpapers__hint strong {
  color: var(--text-secondary);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.wallpapers__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: var(--space-sm);
}

.wp {
  display: flex;
  flex-direction: column;
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-inset);
}

.wp__thumb {
  position: relative;
  display: block;
  padding: 0;
  border: 0;
  background: none;
  cursor: zoom-in;
}

.wp__thumb img {
  width: 100%;
  aspect-ratio: 16 / 10;
  object-fit: cover;
  display: block;
  transition: transform var(--dur-slow) var(--ease-out);
}

.wp__thumb:hover img {
  transform: scale(1.03);
}

.wp__index {
  position: absolute;
  top: 6px;
  left: 6px;
  min-width: 20px;
  padding: 1px 5px;
  border-radius: var(--radius-full);
  background: var(--scrim);
  color: var(--text-on-scrim);
  font-size: var(--text-2xs);
  font-variant-numeric: tabular-nums;
  text-align: center;
}

.wp__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 2px;
  padding: 2px var(--space-2xs);
  border-top: 1px solid var(--border-subtle);
  background: var(--bg-card);
}

.wp--add {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-2xs);
  min-height: 132px;
  border: 1px dashed var(--border-strong);
  color: var(--text-muted);
  font-size: var(--text-xs);
  cursor: pointer;
  transition:
    border-color var(--dur-fast) var(--ease-out),
    color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.wp--add:hover:not(:disabled) {
  border-color: var(--accent);
  background: var(--accent-subtle);
  color: var(--accent);
}

.wp--add:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.wallpapers__empty {
  display: flex;
  align-items: center;
  gap: var(--space-2xs);
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.wallpapers__url {
  display: flex;
  gap: var(--space-sm);
}

.wallpapers__url > :first-child {
  flex: 1;
}

.wp-preview__img {
  width: 100%;
  max-height: 74vh;
  object-fit: contain;
  border-radius: var(--radius-md);
  background: var(--bg-inset);
}
</style>
