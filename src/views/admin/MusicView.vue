<script setup lang="ts">
/**
 * Music library admin page.
 *
 * The library is a disk scan, not a database: this page shows what the scanner
 * found, lets the author drop new files into the music directory, force a
 * re-scan, and remove a track (which deletes the file on disk, hence the
 * confirmation).
 */
import { computed, h, onMounted, ref } from 'vue'
import { NButton, NDataTable, NInput, NPopconfirm, NProgress, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  PhArrowsClockwise,
  PhHardDrives,
  PhMusicNotes,
  PhTrash,
  PhUploadSimple,
} from '@phosphor-icons/vue'
import { deleteMusic, getAdminMusic, rescanMusic, uploadMusic } from '@/api/music'
import type { MusicAdminLibrary, MusicTrack } from '@/types/music'
import { formatDurationMs } from '@/utils/time'
import { formatDate } from '@/utils/format'

const message = useMessage()

const library = ref<MusicAdminLibrary | null>(null)
const loading = ref(false)
const scanning = ref(false)
const uploading = ref(false)
const progress = ref(0)
const dragOver = ref(false)
const keyword = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const ACCEPT = '.mp3,.flac,.m4a,.m4b,.ogg,.opus,.wav,audio/*'

const rows = computed<MusicTrack[]>(() => {
  const needle = keyword.value.trim().toLowerCase()
  const all = library.value?.tracks ?? []
  if (!needle) return all
  return all.filter((track) =>
    `${track.title} ${track.artist} ${track.album}`.toLowerCase().includes(needle),
  )
})

async function load(): Promise<void> {
  loading.value = true
  try {
    library.value = await getAdminMusic()
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '读取音乐库失败')
  } finally {
    loading.value = false
  }
}

async function rescan(): Promise<void> {
  scanning.value = true
  try {
    library.value = await rescanMusic()
    message.success(`扫描完成，共 ${library.value.trackCount} 首`)
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '重新扫描失败')
  } finally {
    scanning.value = false
  }
}

async function handleFiles(files: FileList | null): Promise<void> {
  if (!files || files.length === 0) return

  uploading.value = true
  progress.value = 0
  let uploaded = 0
  try {
    for (const file of Array.from(files)) {
      await uploadMusic(file, (percent) => {
        progress.value = percent
      })
      uploaded += 1
    }
    message.success(`已上传 ${uploaded} 个文件`)
    await load()
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '上传失败')
  } finally {
    uploading.value = false
    progress.value = 0
    if (fileInput.value) fileInput.value.value = ''
  }
}

function onDrop(event: DragEvent): void {
  dragOver.value = false
  void handleFiles(event.dataTransfer?.files ?? null)
}

async function remove(track: MusicTrack): Promise<void> {
  try {
    await deleteMusic(track.id)
    message.success('已删除')
    await load()
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '删除失败')
  }
}

const columns: DataTableColumns<MusicTrack> = [
  {
    title: '曲目',
    key: 'title',
    render: (row) =>
      h('div', { class: 'cell-title' }, [
        h('span', { class: 'cell-title__main' }, row.title),
        h('span', { class: 'cell-title__sub' }, [row.hasCover ? '封面' : '', row.hasLyrics ? '歌词' : ''].filter(Boolean).join(' · ') || '无附加信息'),
      ]),
  },
  {
    title: '歌手',
    key: 'artist',
    width: 170,
    render: (row) => h('span', { class: 'cell-muted' }, row.artist || '未知'),
  },
  {
    title: '专辑',
    key: 'album',
    width: 190,
    render: (row) => h('span', { class: 'cell-muted' }, row.album || '—'),
  },
  {
    title: '时长',
    key: 'durationMs',
    width: 92,
    render: (row) => h('span', { class: 'cell-num' }, formatDurationMs(row.durationMs, '--:--')),
  },
  {
    title: '操作',
    key: 'actions',
    width: 76,
    render: (row) =>
      h(
        NPopconfirm,
        { onPositiveClick: () => void remove(row) },
        {
          trigger: () =>
            h(
              NButton,
              { size: 'tiny', quaternary: true, type: 'error', title: '删除' },
              { icon: () => h(PhTrash, { size: 14 }) },
            ),
          default: () => '删除会把磁盘上的音频文件一起删掉，确定吗?',
        },
      ),
  },
]

onMounted(load)
</script>

<template>
  <div class="music-admin">
    <section class="card">
      <header class="card__head">
        <h2 class="card__title">
          <PhHardDrives :size="18" weight="duotone" aria-hidden="true" />
          音乐库
        </h2>
        <p class="card__hint">
          后端直接扫描音乐目录，识别 ID3 / Vorbis / MP4 标签和同名 .lrc 歌词；放入新文件会自动出现在这里。
        </p>
      </header>

      <dl v-if="library" class="facts">
        <div class="facts__item">
          <dt>曲目</dt>
          <dd>{{ library.trackCount }}</dd>
        </div>
        <div class="facts__item">
          <dt>专辑</dt>
          <dd>{{ library.albumCount }}</dd>
        </div>
        <div class="facts__item">
          <dt>歌手</dt>
          <dd>{{ library.artistCount }}</dd>
        </div>
        <div class="facts__item">
          <dt>目录</dt>
          <dd class="facts__path" :title="library.dir">
            <span :class="{ 'is-missing': !library.exists }">{{ library.dir }}</span>
          </dd>
        </div>
        <div class="facts__item">
          <dt>自动监听</dt>
          <dd>{{ library.watch ? '已开启' : '已关闭' }}</dd>
        </div>
        <div class="facts__item">
          <dt>上次扫描</dt>
          <dd>{{ library.scannedAt ? formatDate(library.scannedAt) : '—' }}</dd>
        </div>
      </dl>

      <p v-if="library && !library.exists" class="warn">
        目录还不存在，上传第一个文件时会自动创建。
      </p>

      <div class="actions">
        <NButton secondary :loading="scanning" @click="rescan">
          <template #icon><PhArrowsClockwise :size="16" /></template>
          重新扫描
        </NButton>
        <span class="actions__hint">支持的格式：{{ library?.extensions.join('、') ?? '—' }}</span>
      </div>
    </section>

    <section
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
        :accept="ACCEPT"
        multiple
        @change="handleFiles(($event.target as HTMLInputElement).files)"
      />
      <PhUploadSimple :size="26" weight="bold" aria-hidden="true" />
      <p class="drop__text">
        把音频拖到这里，或<button class="drop__pick" type="button" @click="fileInput?.click()">选择文件</button>
      </p>
      <p class="drop__hint">文件会写入音乐目录，标签和封面由后端读取，不需要在前端填。</p>
      <NProgress v-if="uploading" class="drop__progress" type="line" :percentage="progress" :show-indicator="false" processing />
    </section>

    <section class="card">
      <header class="card__toolbar">
        <h2 class="card__title">
          <PhMusicNotes :size="18" weight="duotone" aria-hidden="true" />
          全部曲目
        </h2>
        <NInput v-model:value="keyword" clearable placeholder="搜索标题、歌手或专辑" class="card__search" />
      </header>

      <NDataTable
        remote
        :columns="columns"
        :data="rows"
        :loading="loading"
        :row-key="(row: MusicTrack) => row.id"
        :pagination="rows.length > 20 ? { pageSize: 20 } : false"
        :bordered="false"
        :single-line="false"
        size="small"
      />
      <p v-if="!loading && rows.length === 0" class="empty">
        {{ library?.trackCount ? '没有匹配的曲目。' : '音乐库还是空的，先上传几首歌。' }}
      </p>
    </section>
  </div>
</template>

<style scoped>
.music-admin {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  max-width: 1080px;
}

.card {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  padding: var(--space-lg);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.card__head {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
}

.card__toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
}

.card__title {
  display: flex;
  align-items: center;
  gap: var(--space-2xs);
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-lg);
}

.card__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.card__search {
  width: min(280px, 100%);
}

.facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: var(--space-md);
  margin: 0;
}

.facts__item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.facts__item dt {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.facts__item dd {
  margin: 0;
  color: var(--text-primary);
  font-size: var(--text-sm);
  font-variant-numeric: tabular-nums;
}

.facts__path span {
  display: block;
  overflow: hidden;
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.facts__path span.is-missing {
  color: var(--warning);
}

.warn {
  margin: 0;
  padding: var(--space-2xs) var(--space-xs);
  border: 1px solid color-mix(in srgb, var(--warning) 35%, transparent);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--warning) 12%, transparent);
  color: var(--warning);
  font-size: var(--text-xs);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-sm);
}

.actions__hint {
  color: var(--text-muted);
  font-size: var(--text-2xs);
}

.drop {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-xl) var(--space-lg);
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

.empty {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-sm);
  text-align: center;
}
</style>

<!--
  The table's cell renderers run inside NDataTable's own render context, so they
  never receive this component's scope id. Keep these rules global.
-->
<style>
.music-admin .cell-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.music-admin .cell-title__main {
  color: var(--text-primary);
  font-size: var(--text-sm);
}

.music-admin .cell-title__sub {
  color: var(--text-muted);
  font-size: var(--text-2xs);
}

.music-admin .cell-muted {
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.music-admin .cell-num {
  color: var(--text-secondary);
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
}
</style>
