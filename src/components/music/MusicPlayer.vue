<script setup lang="ts">
/**
 * The site-wide music player.
 *
 * Rendered once from the public layout, so it survives route changes: the audio
 * element lives in `useMusic` and keeps playing while the reader navigates.
 *
 * Layout follows the brief — cover on the left, lyrics filling the space in
 * front, the spectrum and progress bar underneath, transport controls below
 * that. Collapsing it leaves a handle on the right edge with a tiny equaliser,
 * so playback stays visible without covering the page.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  PhCaretRight,
  PhListBullets,
  PhMusicNotes,
  PhPause,
  PhPlay,
  PhQueue,
  PhRepeat,
  PhRepeatOnce,
  PhShuffle,
  PhSkipBack,
  PhSkipForward,
  PhSpeakerHigh,
  PhSpeakerSlash,
  PhVinylRecord,
} from '@phosphor-icons/vue'
import { mediaUrl } from '@/api/music'
import { onMusicLevel, PLAY_MODE_LABEL, readSpectrum, useMusic, type PlayMode } from '@/composables/useMusic'
import { animate, createAnimatable, motionTimeline, reduceMotion, stagger, utils } from '@/utils/anime'
import { formatDuration } from '@/utils/time'
import MusicLyrics from './MusicLyrics.vue'
import MusicPlaylist from './MusicPlaylist.vue'
import MusicWaveform from './MusicWaveform.vue'

const music = useMusic()

/* -------------------------------------------------------------------------- */
/* Presentation state                                                         */
/* -------------------------------------------------------------------------- */

const coverFailed = ref(false)

// A track may advertise a cover that no longer decodes; fall back to the glyph
// rather than leaving a broken image in the panel.
watch(
  () => music.currentId.value,
  () => {
    coverFailed.value = false
  },
)

const coverSrc = computed(() => {
  const track = music.current.value
  if (!track || !track.hasCover || coverFailed.value) return ''
  return mediaUrl(track.coverUrl)
})

const hasTrack = computed(() => music.current.value !== null)

/**
 * A blog with no music in the library should not carry a permanent player
 * handle around, so the whole thing stays out of the DOM until there is
 * something to play — or until the library failed to load, which the owner
 * needs to be able to see.
 */
const visible = computed(
  () =>
    music.tracks.value.length > 0 ||
    music.current.value !== null ||
    music.loadError.value !== '',
)

const subtitle = computed(() => {
  const track = music.current.value
  if (!track) return '从播放列表里选一首开始'
  return [track.artist, track.album].filter(Boolean).join(' · ') || '未知歌手'
})

const statusLabel = computed(() => {
  if (!hasTrack.value) return '未播放'
  if (music.buffering.value) return '缓冲中'
  return music.playing.value ? '正在播放' : '已暂停'
})

const modeLabel = computed(() => PLAY_MODE_LABEL[music.mode.value])

const MODE_ICON: Record<PlayMode, typeof PhListBullets> = {
  order: PhListBullets,
  loop: PhRepeat,
  single: PhRepeatOnce,
  shuffle: PhShuffle,
}

const modeIcon = computed(() => MODE_ICON[music.mode.value])

const position = computed(() => music.position.value)
const seekMax = computed(() => Math.max(1, Math.floor(music.duration.value || 0)))

const playedStyle = computed(() => {
  const total = music.duration.value
  const percent = total > 0 ? Math.min(100, (position.value / total) * 100) : 0
  // Painting the filled portion with a gradient beats fighting the vendor
  // pseudo-elements of a range input.
  return `linear-gradient(to right, var(--accent) 0 ${percent}%, var(--border-strong) ${percent}% 100%)`
})

const volumeStyle = computed(() => {
  const level = music.muted.value ? 0 : music.volume.value
  return `linear-gradient(to right, var(--accent) 0 ${level * 100}%, var(--border-strong) ${level * 100}% 100%)`
})

/* -------------------------------------------------------------------------- */
/* Interaction                                                                */
/* -------------------------------------------------------------------------- */

function onScrub(event: Event): void {
  const value = Number((event.target as HTMLInputElement).value)
  if (Number.isFinite(value)) music.seek(value)
}

function onVolume(event: Event): void {
  const value = Number((event.target as HTMLInputElement).value)
  if (!Number.isFinite(value)) return
  if (music.muted.value && value > 0) music.toggleMute()
  music.setVolume(value)
}

function onKeydown(event: KeyboardEvent): void {
  const target = event.target as HTMLElement | null
  // Never hijack typing in the playlist search box.
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) {
    return
  }

  if (event.key === 'Escape' && music.expanded.value) {
    music.setExpanded(false)
    return
  }

  // Space toggles playback once the player has something loaded, matching the
  // convention of every other media surface.
  if (event.code === 'Space' && hasTrack.value && !event.ctrlKey && !event.metaKey && !event.altKey) {
    event.preventDefault()
    music.toggle()
  }
}

/* -------------------------------------------------------------------------- */
/* Motion                                                                     */
/* -------------------------------------------------------------------------- */

/**
 * Active JS motion in this panel, all of it through src/utils/anime.ts.
 *
 * Each element has exactly one owner per property: anime.js turns the cover,
 * breathes the halo, staggers the collapsed handle and plays the entrance — the
 * stylesheet keeps the colours and the rest state, never a keyframe over the
 * same property. Every entry point is a no-op under reduced motion, and nothing
 * is pre-hidden from CSS, so a client with no JS still shows the whole panel.
 */

/** The three timer calls this component needs. Structural, so no library type
 *  (the animation type is a union at the call site) has to be named here. */
interface MotionHandle {
  play: () => unknown
  pause: () => unknown
  revert: () => unknown
}

/** The fast setter pair `createAnimatable` hands back for the halo. */
interface ReactiveHandle {
  scale: (value: number) => unknown
  opacity: (value: number) => unknown
  revert: () => unknown
}

const coverEl = ref<HTMLElement | null>(null)
const ringEl = ref<HTMLElement | null>(null)
const vinylEl = ref<HTMLElement | null>(null)
const haloEl = ref<HTMLElement | null>(null)
const handleBarsEl = ref<HTMLElement | null>(null)
const controlsEl = ref<HTMLElement | null>(null)
const lyricsEl = ref<InstanceType<typeof MusicLyrics> | null>(null)
const waveformEl = ref<InstanceType<typeof MusicWaveform> | null>(null)

let spin: MotionHandle | null = null
let spinKey = ''
let handleEq: MotionHandle | null = null
let halo: ReactiveHandle | null = null
/** The node `halo` is bound to, so a v-if swap cannot leave it driving a
 *  detached element. */
let haloNode: HTMLElement | null = null
let entrance: ReturnType<typeof motionTimeline> = null

/** The root node of a child component, or undefined while it is not rendered. */
function rootOf(instance: { $el: unknown } | null): HTMLElement | undefined {
  const el = instance?.$el
  return el instanceof HTMLElement ? el : undefined
}

/**
 * The cover is a record: the dashed ring around the artwork turns once every
 * 14s, and a track with no art turns the vinyl glyph instead (the two are
 * swapped by v-if, so the ring can be joined by, or lose, the glyph).
 *
 * Rebuilds only when the set of target nodes actually changed, so toggling the
 * playlist — which re-runs the binder for other reasons — does not snap the
 * record back to the top of its turn.
 */
function buildCoverSpin(): void {
  const targets = [ringEl.value, vinylEl.value].filter((el): el is HTMLElement => el !== null)
  const key = (ringEl.value ? 'ring' : '') + (vinylEl.value ? '+vinyl' : '')

  if (spin && key === spinKey) return
  // Any running turn belongs to a node the last render replaced.
  spin?.revert()
  spin = null
  spinKey = key
  if (!music.playing.value || reduceMotion() || targets.length === 0) return

  spin = animate(targets, { rotate: 360, duration: 14000, loop: true, ease: 'linear' })
}

/**
 * Pause keeps the angle instead of rewinding, so resuming does not snap the
 * record back to the top of its turn.
 */
function syncCoverSpin(): void {
  if (!music.playing.value) {
    spin?.pause()
    return
  }
  if (!spin) buildCoverSpin()
  spin?.play()
}

/**
 * Audio-reactive halo around the cover.
 *
 * The panel's own glow already follows `--level` from the stylesheet, so this
 * deliberately drives a different element (the halo's scale and opacity) rather
 * than converting that one: one system per property, no double-driving.
 *
 * Builds the halo's setters once, or null when there is nothing to drive (not
 * mounted yet, or reduced motion). The library types `createAnimatable` as a
 * record of properties, so under noUncheckedIndexedAccess every member reads as
 * possibly undefined; wrapping the two setters this component drives keeps that
 * uncertainty in one place instead of sprinkling assertions through the loop.
 */
function createHalo(): ReactiveHandle | null {
  const el = haloEl.value
  if (!el || reduceMotion()) return null

  haloNode = el
  const animated = createAnimatable(el, { scale: 380, opacity: 380 })
  return {
    // The params above guarantee both members exist at runtime; the library's
    // index signature is the only reason TypeScript is unsure.
    scale: (value) => animated.scale!(value),
    opacity: (value) => animated.opacity!(value),
    revert: () => animated.revert(),
  }
}

function applyLevel(value: number): void {
  if (music.expanded.value) {
    // The halo lives only inside the expanded panel; there is nothing to breathe
    // onto while it is `v-show`n away, so it is not even built until then.
    if (!halo) halo = createHalo()
    const animated = halo
    if (!animated) return

    // Squared, because loudness is perceived logarithmically: a linear map leaves
    // the halo parked at its resting size for most of a track. The ceiling stays
    // low so the blurred spill never washes over the first lyric beside it.
    const energy = value * value
    animated.scale(1 + energy * 0.12)
    animated.opacity(0.22 + energy * 0.4)
    return
  }

  // Collapsed, the handle is the only surface on screen. Its bars are banded off
  // the spectrum rather than off the global level, so a kick and a hi-hat do not
  // move them identically. The idle wobble is paused while playing, which leaves
  // `scaleY` with a single writer.
  if (music.playing.value) setHandleBands(value)
}

/**
 * Writes the handle's bars straight from the analyser, splitting the bins into
 * as many bands as there are bars (low → high). `utils.set` is the same fast
 * setter the halo uses: it writes once per frame instead of starting a tween, so
 * nothing queues up behind the audio.
 */
function setHandleBands(level: number): void {
  const bars = handleBarsEl.value
  if (!bars) return

  const strips = bars.querySelectorAll<HTMLElement>('i')
  if (strips.length === 0) return

  const spectrum = readSpectrum()
  const bandSize = spectrum ? Math.floor(spectrum.length / strips.length) : 0

  strips.forEach((strip, index) => {
    let energy = level

    if (spectrum && bandSize > 0) {
      const start = index * bandSize
      let sum = 0
      for (let bin = start; bin < start + bandSize; bin += 1) sum += spectrum[bin] ?? 0
      energy = sum / bandSize / 255
    }

    utils.set(strip, { scaleY: 0.3 + energy * 0.85 })
  })
}

/**
 * The collapsed handle keeps a quiet idle wobble, so the handle reads as "there
 * is music here" before anyone presses play. anime.js owns `scaleY` outright —
 * the stylesheet only recolours the bars while playing.
 */
function syncHandleEq(): void {
  // While playing, the live spectrum owns the bars (see `setHandleBands`), so the
  // idle wobble is held here rather than left to fight over the same property.
  if (!visible.value || music.expanded.value || music.playing.value || reduceMotion()) {
    handleEq?.pause()
    return
  }

  if (!handleEq) {
    const bars = handleBarsEl.value
    const strips = bars ? Array.from(bars.querySelectorAll<HTMLElement>('i')) : []
    if (strips.length === 0) return
    handleEq = animate(strips, {
      scaleY: () => utils.random(0.35, 1),
      duration: () => utils.random(520, 900),
      loop: true,
      alternate: true,
      ease: 'inOutSine',
      delay: stagger(60),
    })
  }

  handleEq.play()
}

/**
 * Panel entrance: cover → lyrics → spectrum → controls, each block a beat
 * behind the one before it. The panel is `v-show`, so the nodes exist even
 * while collapsed and the timeline can replay on every expand.
 */
function playEntrance(): void {
  entrance?.revert()
  const timeline = motionTimeline()
  entrance = timeline
  if (!timeline) return

  const cover = coverEl.value
  if (cover) timeline.add(cover, { opacity: [0, 1], translateY: [14, 0], duration: 620 }, 0)

  const lyrics = rootOf(lyricsEl.value)
  if (lyrics) timeline.add(lyrics, { opacity: [0, 1], translateY: [12, 0], duration: 580 }, 70)

  const waveform = rootOf(waveformEl.value)
  if (waveform) timeline.add(waveform, { opacity: [0, 1], translateY: [10, 0], duration: 560 }, 140)

  const controls = controlsEl.value
  if (controls) timeline.add(controls, { opacity: [0, 1], translateY: [8, 0], duration: 520 }, 200)
}

/**
 * Re-targets every animation that points at a DOM node: called after any change
 * that can mount, swap or unmount one (panel visibility, playlist toggle, a
 * cover arriving, a failed cover falling back to the glyph).
 */
function rebindMotion(): void {
  // A cover can be swapped for the playlist and back, which unmounts the halo;
  // dropping the setter bound to the old node lets the next frame rebuild
  // against the element that is actually mounted.
  if (halo && haloNode !== haloEl.value) {
    halo.revert()
    halo = null
    haloNode = null
  }
  buildCoverSpin()
  syncHandleEq()
}

// The entrance belongs to the expand transition alone: replaying it whenever the
// playlist flips, or a cover decodes, would re-fade blocks the reader is
// already looking at. The handle's wobble is the other half — it pauses while
// the panel is open, so a hidden handle never keeps a loop alive.
watch(() => music.expanded.value, (expanded) => {
  if (expanded) void nextTick(playEntrance)
  syncHandleEq()
})

watch(
  [visible, () => music.showPlaylist.value, coverSrc],
  () => {
    void nextTick(rebindMotion)
  },
)

watch(() => music.playing.value, () => {
  syncCoverSpin()
  syncHandleEq()
})

let unsubscribeLevel: (() => void) | null = null

onMounted(() => {
  // Load the library lazily: the handle can show a count before first play.
  void music.load()
  window.addEventListener('keydown', onKeydown)

  // Fed from the player's own frame, so the halo never opens a second rAF loop.
  unsubscribeLevel = onMusicLevel(applyLevel)
  void nextTick(rebindMotion)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  unsubscribeLevel?.()
  unsubscribeLevel = null
  spin?.revert()
  handleEq?.revert()
  halo?.revert()
  entrance?.revert()
})
</script>

<template>
  <div v-if="visible" class="player-root">
    <!-- Collapsed: a handle on the right edge with a live equaliser. -->
    <Transition name="player-handle">
      <button
        v-show="!music.expanded.value"
        type="button"
        class="player-handle"
        :class="{ 'is-playing': music.playing.value }"
        aria-label="展开音乐播放器"
        title="展开音乐播放器"
        :aria-expanded="music.expanded.value"
        @click="music.toggleExpanded()"
      >
        <span class="player-handle__icon" aria-hidden="true">
          <PhMusicNotes :size="19" weight="bold" />
        </span>
        <span ref="handleBarsEl" class="player-handle__bars" aria-hidden="true"><i /><i /><i /></span>
      </button>
    </Transition>

    <!-- Expanded: the panel itself. -->
    <Transition name="player-panel">
      <aside v-show="music.expanded.value" class="player" aria-label="音乐播放器">
        <span class="player__glow" :style="{ '--level': music.level.value.toFixed(3) }" aria-hidden="true" />
        <span class="player__mesh" aria-hidden="true" />

        <header class="player__head">
          <div class="player__meta">
            <p class="player__status">
              <span class="player__pulse" :class="{ 'is-on': music.playing.value }" aria-hidden="true" />
              {{ statusLabel }}<template v-if="music.tracks.value.length"> · 曲库 {{ music.tracks.value.length }} 首</template>
            </p>
            <h2 class="player__title">{{ music.current.value?.title ?? '音乐播放器' }}</h2>
            <p class="player__artist">{{ subtitle }}</p>
          </div>

          <button
            type="button"
            class="player__icon player__collapse"
            aria-label="收起音乐播放器"
            title="收起音乐播放器"
            @click="music.setExpanded(false)"
          >
            <PhCaretRight :size="18" weight="bold" aria-hidden="true" />
          </button>
        </header>

        <p v-if="music.playError.value" class="player__error" role="alert">{{ music.playError.value }}</p>

        <div class="player__stage">
          <template v-if="!music.showPlaylist.value">
            <!-- Translation of the motion split: the halo is the audio-reactive
                 element (anime.js breathes its scale/opacity), the dashed ring
                 is what anime.js turns, and `--level` on .player__glow above
                 stays the stylesheet's job. One owner per property. -->
            <div ref="coverEl" class="player__cover">
              <span ref="haloEl" class="player__cover-halo" aria-hidden="true" />
              <span ref="ringEl" class="player__cover-ring" aria-hidden="true" />
              <div class="player__cover-art">
                <img
                  v-if="coverSrc"
                  :src="coverSrc"
                  :alt="`${music.current.value?.album || music.current.value?.title || ''} 封面`"
                  decoding="async"
                  loading="lazy"
                  @error="coverFailed = true"
                />
                <span v-else ref="vinylEl" class="player__cover-vinyl">
                  <PhVinylRecord :size="26" weight="duotone" aria-hidden="true" />
                </span>
              </div>
            </div>

            <MusicLyrics ref="lyricsEl" />
          </template>

          <MusicPlaylist v-else />
        </div>

        <MusicWaveform ref="waveformEl" />

        <div class="player__timeline">
          <span class="player__time">{{ formatDuration(position, '0:00') }}</span>
          <input
            class="player__range player__range--seek"
            type="range"
            min="0"
            :max="seekMax"
            step="1"
            :value="Math.floor(position)"
            :style="{ background: playedStyle }"
            aria-label="播放进度"
            :aria-valuetext="`${formatDuration(position, '0:00')} / ${formatDuration(music.duration.value)}`"
            @input="onScrub"
          />
          <span class="player__time">{{ formatDuration(music.duration.value) }}</span>
        </div>

        <div ref="controlsEl" class="player__controls">
          <button
            type="button"
            class="player__icon"
            :title="`播放模式：${modeLabel}`"
            :aria-label="`播放模式：${modeLabel}，点击切换`"
            @click="music.cycleMode()"
          >
            <component :is="modeIcon" :size="18" weight="bold" aria-hidden="true" />
          </button>

          <button
            type="button"
            class="player__icon"
            title="上一首"
            aria-label="上一首"
            :disabled="!hasTrack"
            @click="music.prev()"
          >
            <PhSkipBack :size="20" weight="fill" aria-hidden="true" />
          </button>

          <button
            type="button"
            class="player__play"
            :aria-label="music.playing.value ? '暂停' : '播放'"
            :title="music.playing.value ? '暂停' : '播放'"
            :disabled="!hasTrack && !music.tracks.value.length"
            @click="music.toggle()"
          >
            <PhPause v-if="music.playing.value" :size="21" weight="fill" aria-hidden="true" />
            <PhPlay v-else :size="21" weight="fill" aria-hidden="true" />
          </button>

          <button
            type="button"
            class="player__icon"
            title="下一首"
            aria-label="下一首"
            :disabled="!hasTrack"
            @click="music.next()"
          >
            <PhSkipForward :size="20" weight="fill" aria-hidden="true" />
          </button>

          <button
            type="button"
            class="player__icon"
            :class="{ 'is-on': music.showPlaylist.value }"
            title="播放列表"
            aria-label="播放列表"
            :aria-pressed="music.showPlaylist.value"
            @click="music.togglePlaylist()"
          >
            <PhQueue :size="18" weight="bold" aria-hidden="true" />
          </button>

          <span class="player__spacer" />

          <div class="player__volume">
            <button
              type="button"
              class="player__icon"
              :title="music.muted.value ? '取消静音' : '静音'"
              :aria-label="music.muted.value ? '取消静音' : '静音'"
              @click="music.toggleMute()"
            >
              <PhSpeakerSlash v-if="music.muted.value || music.volume.value === 0" :size="18" weight="bold" aria-hidden="true" />
              <PhSpeakerHigh v-else :size="18" weight="bold" aria-hidden="true" />
            </button>
            <input
              class="player__range player__range--volume"
              type="range"
              min="0"
              max="1"
              step="0.01"
              :value="music.muted.value ? 0 : music.volume.value"
              :style="{ background: volumeStyle }"
              aria-label="音量"
              @input="onVolume"
            />
          </div>
        </div>
      </aside>
    </Transition>
  </div>
</template>

<style scoped>
.player-root {
  /* Kept out of the layout flow: the player floats above the page. */
  position: relative;
  z-index: var(--z-overlay);
}

/* ------------------------------------------------------------------ handle */

.player-handle {
  position: fixed;
  top: 50%;
  right: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2xs);
  width: 44px;
  padding: var(--space-sm) 0;
  border: 1px solid var(--border-subtle);
  border-right: 0;
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
  background: color-mix(in srgb, var(--bg-elevated) 88%, transparent);
  color: var(--text-secondary);
  cursor: pointer;
  transform: translateY(-50%);
  backdrop-filter: blur(14px);
  box-shadow: var(--shadow-md);
  transition:
    color var(--dur-fast) var(--ease-out),
    border-color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out),
    transform var(--dur-fast) var(--ease-out);
}

.player-handle:hover {
  color: var(--accent);
  border-color: var(--accent-border);
}

.player-handle:active {
  transform: translateY(-50%) scale(0.97);
}

.player-handle:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.player-handle__icon {
  display: inline-flex;
}

.player-handle__bars {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 14px;
}

.player-handle__bars i {
  width: 2px;
  height: 100%;
  border-radius: var(--radius-full);
  background: var(--text-muted);
  /* The resting height, and the value everything falls back to without JS.
     anime.js owns scaleY from here (idle wobble, or a real spectrum during
     playback) — the stylesheet must never animate it too. */
  transform: scaleY(0.24);
  transform-origin: bottom;
}

.player-handle.is-playing .player-handle__bars i {
  /* Colour only: motion belongs to anime.js. */
  background: var(--accent);
}

/* ------------------------------------------------------------------- panel */

.player {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: var(--z-overlay);
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  width: min(400px, 100vw);
  padding: var(--space-md);
  padding-bottom: max(var(--space-md), env(safe-area-inset-bottom));
  overflow: hidden;
  border-left: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--bg-base) 92%, transparent);
  backdrop-filter: blur(22px) saturate(1.2);
  box-shadow: var(--shadow-lg);
}

.player__glow {
  position: absolute;
  inset: -25% -35%;
  z-index: -2;
  background:
    radial-gradient(42% 38% at 24% 18%, color-mix(in srgb, var(--accent) 40%, transparent), transparent 70%),
    radial-gradient(46% 42% at 78% 72%, color-mix(in srgb, var(--accent) 26%, transparent), transparent 72%),
    radial-gradient(38% 34% at 62% 22%, color-mix(in srgb, var(--accent) 18%, transparent), transparent 70%);
  filter: blur(34px);
  opacity: calc(0.34 + var(--level, 0) * 0.46);
  scale: calc(1 + var(--level, 0) * 0.14);
  translate: 0 0;
  animation: player-drift 18s ease-in-out infinite alternate;
  transition:
    opacity 140ms linear,
    scale 140ms linear;
  pointer-events: none;
}

.player__mesh {
  position: absolute;
  inset: 0;
  z-index: -1;
  background-image: radial-gradient(circle at 1px 1px, var(--grid-line) 1px, transparent 0);
  background-size: 22px 22px;
  mask-image: linear-gradient(180deg, transparent, #000 30%, #000 70%, transparent);
  opacity: 0.5;
  pointer-events: none;
}

@keyframes player-drift {
  from {
    translate: -4% -3%;
  }
  to {
    translate: 5% 4%;
  }
}

.player__head {
  display: flex;
  gap: var(--space-xs);
  align-items: flex-start;
  justify-content: space-between;
}

.player__meta {
  min-width: 0;
}

.player__status {
  display: flex;
  align-items: center;
  gap: var(--space-2xs);
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.player__pulse {
  width: 6px;
  height: 6px;
  border-radius: var(--radius-full);
  background: var(--text-muted);
}

.player__pulse.is-on {
  background: var(--accent);
  box-shadow: 0 0 0 0 var(--accent-subtle);
  animation: player-pulse 1.6s ease-out infinite;
}

@keyframes player-pulse {
  from {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--accent) 55%, transparent);
  }
  to {
    box-shadow: 0 0 0 10px transparent;
  }
}

.player__title {
  margin: 2px 0 0;
  overflow: hidden;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: 600;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.player__artist {
  overflow: hidden;
  color: var(--text-secondary);
  font-size: var(--text-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.player__error {
  padding: var(--space-2xs) var(--space-xs);
  border: 1px solid color-mix(in srgb, var(--danger) 35%, transparent);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--danger) 12%, transparent);
  color: var(--danger);
  font-size: var(--text-2xs);
}

/* ------------------------------------------------------------------- stage */

.player__stage {
  display: flex;
  flex: 1;
  gap: var(--space-sm);
  min-height: 0;
}

.player__cover {
  position: relative;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 96px;
  height: 96px;
  /* No clipping any more: the ring orbits just outside the artwork box and a
     clipped value would swallow it. Nothing here animates — anime.js owns the
     turn. */
  color: var(--text-muted);
}

/* The audio-reactive element: anime.js writes scale/opacity on it from the
   player's own frame. Rests at a barely-there glow, so a client without JS
   shows a static but complete cover. */
.player__cover-halo {
  position: absolute;
  /* Kept tight plus a low peak opacity (see the setter) so the blurred spill
     reads as a rim glow and never washes over the lyric column beside it. */
  inset: -6%;
  z-index: 0;
  border-radius: var(--radius-full);
  background: radial-gradient(50% 50% at 50% 50%, color-mix(in srgb, var(--accent) 32%, transparent), transparent 72%);
  filter: blur(11px);
  opacity: 0.22;
  pointer-events: none;
}

.player__cover-art {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-inset);
  box-shadow: var(--shadow-md);
}

.player__cover-art img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* A track without artwork turns this glyph in place of the rotating cover. */
.player__cover-vinyl {
  display: inline-flex;
}

.player__cover-ring {
  position: absolute;
  inset: -8%;
  z-index: 2;
  border-radius: var(--radius-full);
  border: 1px dashed color-mix(in srgb, var(--accent) 42%, transparent);
  opacity: 0.55;
  pointer-events: none;
}

/* -------------------------------------------------------------- timeline */

.player__timeline {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: var(--space-2xs);
}

.player__time {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  font-variant-numeric: tabular-nums;
}

.player__range {
  -webkit-appearance: none;
  appearance: none;
  height: 4px;
  border-radius: var(--radius-full);
  background: var(--border-strong);
  cursor: pointer;
}

.player__range::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 12px;
  height: 12px;
  border: 0;
  border-radius: var(--radius-full);
  background: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent);
  transition: transform var(--dur-fast) var(--ease-out);
}

.player__range::-moz-range-thumb {
  width: 12px;
  height: 12px;
  border: 0;
  border-radius: var(--radius-full);
  background: var(--accent);
}

.player__range:hover::-webkit-slider-thumb {
  transform: scale(1.15);
}

.player__range:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 4px;
}

.player__range--seek {
  width: 100%;
}

.player__range--volume {
  width: 72px;
}

/* -------------------------------------------------------------- controls */

.player__controls {
  display: flex;
  align-items: center;
  gap: var(--space-2xs);
}

.player__spacer {
  flex: 1;
}

.player__volume {
  display: flex;
  align-items: center;
  gap: var(--space-2xs);
}

.player__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  background: none;
  color: var(--text-secondary);
  cursor: pointer;
  transition:
    color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out),
    border-color var(--dur-fast) var(--ease-out),
    transform var(--dur-fast) var(--ease-out);
}

.player__icon:hover:not(:disabled) {
  border-color: var(--border-subtle);
  background: var(--bg-hover);
  color: var(--accent);
}

.player__icon:active:not(:disabled) {
  transform: scale(0.94);
}

.player__icon:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.player__icon.is-on {
  border-color: var(--accent-border);
  background: var(--accent-subtle);
  color: var(--accent);
}

.player__icon:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.player__play {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 46px;
  height: 46px;
  margin: 0 var(--space-2xs);
  border: 0;
  border-radius: var(--radius-full);
  background: var(--accent);
  color: var(--text-inverse);
  cursor: pointer;
  box-shadow: 0 6px 22px color-mix(in srgb, var(--accent) 42%, transparent);
  transition:
    transform var(--dur-fast) var(--ease-spring),
    background var(--dur-fast) var(--ease-out),
    box-shadow var(--dur-fast) var(--ease-out);
}

.player__play:hover:not(:disabled) {
  background: var(--accent-hover);
  transform: scale(1.06);
}

.player__play:active:not(:disabled) {
  transform: scale(0.95);
}

.player__play:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}

.player__play:disabled {
  opacity: 0.45;
  cursor: not-allowed;
  box-shadow: none;
}

.player__collapse {
  color: var(--text-muted);
}

/* --------------------------------------------------------- transitions */

.player-panel-enter-active,
.player-panel-leave-active {
  transition:
    transform var(--dur-slow) var(--ease-out),
    opacity var(--dur-normal) var(--ease-out);
}

.player-panel-enter-from,
.player-panel-leave-to {
  transform: translateX(102%);
  opacity: 0;
}

.player-handle-enter-active,
.player-handle-leave-active {
  transition:
    transform var(--dur-normal) var(--ease-out),
    opacity var(--dur-normal) var(--ease-out);
}

.player-handle-enter-from,
.player-handle-leave-to {
  transform: translate(102%, -50%);
  opacity: 0;
}

@media (max-width: 640px) {
  .player {
    top: auto;
    left: 0;
    width: 100%;
    height: min(78vh, 620px);
    border-left: 0;
    border-top: 1px solid var(--border-subtle);
    border-radius: var(--radius-xl) var(--radius-xl) 0 0;
  }

  .player-panel-enter-from,
  .player-panel-leave-to {
    transform: translateY(102%);
  }

  .player__cover {
    width: 76px;
    height: 76px;
  }

  .player__range--volume {
    display: none;
  }

  .player-handle {
    top: auto;
    bottom: 96px;
    transform: none;
    border-radius: var(--radius-lg) 0 0 var(--radius-lg);
  }

  .player-handle:active {
    transform: scale(0.97);
  }
}

@media (prefers-reduced-motion: reduce) {
  /* Only the stylesheet's own loops are silenced here; the cover turn, the
     halo breathing, the handle wobble and the entrance timeline are skipped by
     src/utils/anime.ts (reduceMotion) and never leave anything hidden. */
  .player__glow,
  .player__pulse {
    animation: none;
  }

  .player-panel-enter-active,
  .player-panel-leave-active,
  .player-handle-enter-active,
  .player-handle-leave-active {
    transition: opacity var(--dur-fast) linear;
  }

  .player-panel-enter-from,
  .player-panel-leave-to,
  .player-handle-enter-from,
  .player-handle-leave-to {
    transform: none;
  }
}
</style>
