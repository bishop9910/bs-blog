<script setup lang="ts">
/**
 * Playlist drawer for the music player.
 *
 * The full library is the queue by default; picking a row plays that track
 * against the currently visible order so "next" follows what the reader sees.
 * Rows stay cheap (no covers, no per-row observers) because a personal library
 * can easily run into the thousands of files.
 */
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { PhArrowClockwise, PhMagnifyingGlass, PhShuffle, PhVinylRecord } from '@phosphor-icons/vue'
import { useMusic } from '@/composables/useMusic'
import { motionStagger } from '@/utils/anime'
import { formatDurationMs } from '@/utils/time'
import type { MusicTrack } from '@/types/music'

const music = useMusic()

const keyword = ref('')

const list = ref<HTMLElement | null>(null)

/**
 * Ids that have already arrived on screen. Typing in the search box re-renders
 * the same keyed rows, so this is what keeps the entrance from replaying on
 * every keystroke: only a row nobody has seen yet gets the stagger.
 */
const introduced = new Set<string>()

const visible = computed<MusicTrack[]>(() => {
  const needle = keyword.value.trim().toLowerCase()
  const all = music.tracks.value
  if (!needle) return all

  return all.filter((track) =>
    `${track.title} ${track.artist} ${track.album}`.toLowerCase().includes(needle),
  )
})

function onSelect(track: MusicTrack): void {
  // Play within the filtered order so "next" matches the list on screen.
  music.playTrack(track, visible.value.map((item) => item.id))
}

function retry(): void {
  void music.load(true)
}

/**
 * Rows slide in when the drawer opens and when a filter reveals tracks that
 * were not on screen. `motionStagger` is a no-op under reduced motion and never
 * pre-hides anything from CSS, so the list is fully readable either way.
 */
function revealNewRows(): void {
  const root = list.value
  if (!root) return

  const fresh: HTMLElement[] = []
  for (const row of root.querySelectorAll<HTMLElement>('[data-track-id]')) {
    const id = row.dataset.trackId
    if (!id || introduced.has(id)) continue
    introduced.add(id)
    fresh.push(row)
  }

  if (fresh.length === 0) return
  motionStagger(fresh, { opacity: [0, 1], translateY: [10, 0] }, { each: 28 })
}

onMounted(() => {
  void nextTick(revealNewRows)
})

watch(visible, () => {
  void nextTick(revealNewRows)
})
</script>

<template>
  <div class="playlist">
    <div class="playlist__tools">
      <label class="playlist__search">
        <PhMagnifyingGlass :size="14" weight="bold" aria-hidden="true" />
        <span class="sr-only">搜索曲目</span>
        <input v-model="keyword" type="search" placeholder="搜索标题、歌手或专辑" spellcheck="false" />
      </label>

      <button
        type="button"
        class="playlist__shuffle"
        title="随机播放全部"
        aria-label="随机播放全部"
        :disabled="music.tracks.value.length === 0"
        @click="music.shuffleAll()"
      >
        <PhShuffle :size="16" weight="bold" aria-hidden="true" />
      </button>
    </div>

    <p class="playlist__meta">
      <span>{{ visible.length }} / {{ music.tracks.value.length }} 首</span>
      <span v-if="keyword.trim()" class="playlist__meta-hint">已筛选</span>
    </p>

    <div v-if="music.loading.value && !music.loaded.value" class="playlist__state">正在读取曲库…</div>

    <div v-else-if="music.loadError.value" class="playlist__state playlist__state--error">
      <p>{{ music.loadError.value }}</p>
      <button type="button" class="playlist__retry" @click="retry">
        <PhArrowClockwise :size="14" weight="bold" aria-hidden="true" />
        重试
      </button>
    </div>

    <div v-else-if="music.tracks.value.length === 0" class="playlist__state">
      <PhVinylRecord :size="22" weight="duotone" aria-hidden="true" />
      <p>曲库是空的</p>
      <small>把音频放进音乐目录，或在后台的音乐页面上传</small>
    </div>

    <div v-else-if="visible.length === 0" class="playlist__state">没有匹配的曲目</div>

    <ul v-else ref="list" class="playlist__list">
      <li v-for="(track, index) in visible" :key="track.id">
        <button
          type="button"
          class="playlist__row"
          :data-track-id="track.id"
          :class="{ 'is-current': track.id === music.currentId.value }"
          :aria-current="track.id === music.currentId.value ? 'true' : undefined"
          @click="onSelect(track)"
        >
          <span class="playlist__marker" aria-hidden="true">
            <span v-if="track.id === music.currentId.value && music.playing.value" class="playlist__eq">
              <i /><i /><i />
            </span>
            <span v-else>{{ index + 1 }}</span>
          </span>

          <span class="playlist__text">
            <span class="playlist__title">{{ track.title }}</span>
            <span class="playlist__sub">{{ track.artist || '未知歌手' }}<template v-if="track.album"> · {{ track.album }}</template></span>
          </span>

          <span class="playlist__time">{{ formatDurationMs(track.durationMs, '--:--') }}</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.playlist {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
  min-height: 0;
  flex: 1;
}

.playlist__tools {
  display: flex;
  gap: var(--space-2xs);
}

.playlist__search {
  display: flex;
  flex: 1;
  align-items: center;
  gap: var(--space-2xs);
  padding: 0 var(--space-xs);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-inset);
  color: var(--text-muted);
}

.playlist__search:focus-within {
  border-color: var(--accent-border);
  box-shadow: 0 0 0 2px var(--accent-subtle);
}

.playlist__search input {
  flex: 1;
  min-width: 0;
  height: 34px;
  border: 0;
  background: none;
  color: var(--text-primary);
  font: inherit;
  font-size: var(--text-xs);
}

.playlist__search input:focus {
  outline: none;
}

.playlist__shuffle,
.playlist__retry {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2xs);
  min-width: 34px;
  height: 34px;
  padding: 0 var(--space-xs);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-inset);
  color: var(--text-secondary);
  font-size: var(--text-xs);
  cursor: pointer;
  transition:
    color var(--dur-fast) var(--ease-out),
    border-color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.playlist__shuffle:hover:not(:disabled),
.playlist__retry:hover {
  border-color: var(--accent-border);
  color: var(--accent);
}

.playlist__shuffle:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.playlist__meta {
  display: flex;
  justify-content: space-between;
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: 0.04em;
}

.playlist__meta-hint {
  color: var(--accent);
}

.playlist__state {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-2xs);
  padding: var(--space-lg);
  color: var(--text-muted);
  font-size: var(--text-xs);
  text-align: center;
}

.playlist__state--error {
  color: var(--danger);
}

.playlist__state small {
  font-size: var(--text-2xs);
  opacity: 0.8;
}

.playlist__list {
  flex: 1;
  min-height: 0;
  margin: 0;
  padding: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  list-style: none;
  scrollbar-width: thin;
}

.playlist__row {
  display: grid;
  grid-template-columns: 22px 1fr auto;
  align-items: center;
  gap: var(--space-xs);
  width: 100%;
  padding: var(--space-2xs) var(--space-xs);
  border: 0;
  border-radius: var(--radius-sm);
  background: none;
  color: inherit;
  text-align: left;
  cursor: pointer;
  /* Long libraries: skip layout for rows that are far off-screen. */
  content-visibility: auto;
  contain-intrinsic-size: auto 46px;
  transition: background var(--dur-fast) var(--ease-out);
}

.playlist__row:hover {
  background: var(--bg-hover);
}

.playlist__row:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

.playlist__row.is-current {
  background: color-mix(in srgb, var(--accent) 12%, transparent);
}

.playlist__marker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 18px;
  color: var(--text-muted);
  font-size: var(--text-2xs);
  font-variant-numeric: tabular-nums;
}

.playlist__row.is-current .playlist__marker {
  color: var(--accent);
}

.playlist__eq {
  display: inline-flex;
  align-items: flex-end;
  gap: 2px;
  height: 14px;
}

.playlist__eq i {
  width: 2px;
  height: 100%;
  border-radius: var(--radius-full);
  background: var(--accent);
  transform-origin: bottom;
  animation: playlist-eq 0.9s ease-in-out infinite;
}

.playlist__eq i:nth-child(2) {
  animation-delay: 0.18s;
}

.playlist__eq i:nth-child(3) {
  animation-delay: 0.36s;
}

@keyframes playlist-eq {
  0%,
  100% {
    transform: scaleY(0.3);
  }
  50% {
    transform: scaleY(1);
  }
}

.playlist__text {
  min-width: 0;
}

.playlist__title {
  display: block;
  overflow: hidden;
  color: var(--text-primary);
  font-size: var(--text-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.playlist__row.is-current .playlist__title {
  color: var(--accent);
  font-weight: 600;
}

.playlist__sub {
  display: block;
  overflow: hidden;
  color: var(--text-muted);
  font-size: var(--text-2xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.playlist__time {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  font-variant-numeric: tabular-nums;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
  border: 0;
}

@media (prefers-reduced-motion: reduce) {
  .playlist__eq i {
    animation: none;
  }
}
</style>
