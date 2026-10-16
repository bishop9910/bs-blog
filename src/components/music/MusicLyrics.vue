<script setup lang="ts">
/**
 * Lyrics panel.
 *
 * Timed LRC lines scroll to keep the active one centred; untimed lyrics still
 * render, with the offset hint dropped, because a lyrics file without
 * timestamps is still more useful than an empty box.
 *
 * Lines are never truncated — a wrapped lyric beats an ellipsis, and the row
 * grows to fit rather than clipping.
 *
 * A bilingual file gives one line a translation as well as its original; the
 * pair travels as one clickable row, with the translation set smaller, dimmer
 * and one step quieter while the active pair is highlighted together.
 */
import { computed, nextTick, ref, watch } from 'vue'
import { PhMusicNotes } from '@phosphor-icons/vue'
import { useMusic } from '@/composables/useMusic'

const { lyrics, plainLyrics, lyricsLoading, activeLyricIndex, current, seek } = useMusic()

const scroller = ref<HTMLElement | null>(null)

const hasTimed = computed(() => lyrics.value.length > 0)
const hasPlain = computed(() => lyrics.value.length === 0 && plainLyrics.value.length > 0)
const isEmpty = computed(() => !hasTimed.value && !hasPlain.value)

const reduced =
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

function scrollToActive(): void {
  const el = scroller.value
  if (!el || activeLyricIndex.value < 0) return

  const line = el.querySelector<HTMLElement>(`[data-line="${activeLyricIndex.value}"]`)
  if (!line) return

  // Scroll the container rather than calling scrollIntoView: the latter walks up
  // and scrolls ancestors too, which would jerk the whole page.
  const top = line.offsetTop - el.clientHeight / 2 + line.offsetHeight / 2
  el.scrollTo({ top: Math.max(0, top), behavior: reduced ? 'auto' : 'smooth' })
}

watch(activeLyricIndex, () => {
  void nextTick(scrollToActive)
})

// A new track starts from the top, not from wherever the last one ended.
watch(current, () => {
  const el = scroller.value
  if (el) el.scrollTop = 0
})

function jumpTo(index: number): void {
  const line = lyrics.value[index]
  if (!line) return
  seek(line.time / 1000)
}
</script>

<template>
  <div ref="scroller" class="lyrics" :class="{ 'lyrics--plain': hasPlain }">
    <template v-if="hasTimed">
      <button
        v-for="(line, index) in lyrics"
        :key="`${line.time}-${index}`"
        type="button"
        class="lyrics__line"
        :class="{
          'is-active': index === activeLyricIndex,
          'is-past': index < activeLyricIndex,
        }"
        :data-line="index"
        :aria-current="index === activeLyricIndex ? 'true' : undefined"
        @click="jumpTo(index)"
      >
        <span class="lyrics__text">{{ line.text }}</span>
        <span v-if="line.translation" class="lyrics__translation">
          <!--
            A bilingual file can stack several translations on one timestamp;
            the parser keeps them apart with newlines, so each one is its own
            row here instead of relying on whitespace handling.
          -->
          <span
            v-for="(part, partIndex) in line.translation.split('\n')"
            :key="partIndex"
            class="lyrics__translation-line"
          >{{ part }}</span>
        </span>
      </button>
    </template>

    <p v-else-if="hasPlain" v-for="(line, index) in plainLyrics" :key="index" class="lyrics__plain">
      {{ line }}
    </p>

    <div v-else-if="lyricsLoading" class="lyrics__empty">
      <span class="lyrics__skeleton" />
      <span class="lyrics__skeleton lyrics__skeleton--short" />
      <span class="lyrics__skeleton" />
    </div>

    <div v-else-if="isEmpty" class="lyrics__empty lyrics__empty--message">
      <PhMusicNotes :size="22" weight="duotone" />
      <p>暂无歌词</p>
      <small>把同名 .lrc 文件放在音频旁边即可显示</small>
    </div>
  </div>
</template>

<style scoped>
.lyrics {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
  /* Generous end padding lets the first and last lines reach the centre line. */
  padding-block: 40%;
  mask-image: linear-gradient(180deg, transparent, #000 14%, #000 86%, transparent);
}

.lyrics__line {
  display: flex;
  flex-direction: column;
  justify-content: center;
  /* Every row is a seek target, so it keeps a full touch height even when it
     holds a single short line. */
  min-height: var(--hit-min);
  width: 100%;
  padding: var(--space-xs) var(--space-sm);
  border: 0;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--text-muted);
  font-size: var(--text-sm);
  line-height: var(--leading-normal);
  text-align: left;
  cursor: pointer;
  transition:
    color var(--dur-normal) var(--ease-out),
    transform var(--dur-normal) var(--ease-out),
    opacity var(--dur-normal) var(--ease-out);
}

/* No ellipsis, ever: a long lyric or a long translation wraps and the row grows
   to fit instead of clipping the words the reader came here for. */
.lyrics__text,
.lyrics__translation-line {
  display: block;
  white-space: normal;
  overflow-wrap: anywhere;
}

.lyrics__translation {
  display: block;
  margin-top: 2px;
  color: var(--text-muted);
  font-size: var(--text-2xs);
  line-height: var(--leading-normal);
}

.lyrics__line:hover {
  color: var(--text-secondary);
}

.lyrics__line:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.lyrics__line.is-past {
  opacity: 0.55;
}

.lyrics__line.is-active {
  transform: translateX(2px);
}

/* The original carries the emphasis; the translation is highlighted in step
   with it, but smaller and without the glow, so the pair reads as one line. */
.lyrics__line.is-active .lyrics__text {
  color: var(--text-primary);
  font-size: var(--text-base);
  font-weight: 600;
  text-shadow: 0 0 22px color-mix(in srgb, var(--accent) 45%, transparent);
}

.lyrics__line.is-active .lyrics__translation {
  color: var(--accent);
  font-weight: 500;
}

.lyrics__line.is-active .lyrics__translation-line {
  opacity: 0.9;
}

.lyrics__plain {
  padding: var(--space-2xs) var(--space-sm);
  color: var(--text-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
  overflow-wrap: anywhere;
}

.lyrics__empty {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
  padding: var(--space-sm);
}

.lyrics__empty--message {
  align-items: center;
  justify-content: center;
  gap: var(--space-2xs);
  min-height: 100%;
  color: var(--text-muted);
  text-align: center;
}

.lyrics__empty--message p {
  font-size: var(--text-sm);
}

.lyrics__empty--message small {
  font-size: var(--text-2xs);
  opacity: 0.75;
}

.lyrics__skeleton {
  height: 12px;
  border-radius: var(--radius-full);
  background: linear-gradient(
    90deg,
    var(--bg-hover),
    color-mix(in srgb, var(--accent) 24%, var(--bg-hover)),
    var(--bg-hover)
  );
  background-size: 200% 100%;
  animation: lyrics-shimmer 1.6s linear infinite;
}

.lyrics__skeleton--short {
  width: 62%;
}

@keyframes lyrics-shimmer {
  from {
    background-position: 200% 0;
  }
  to {
    background-position: -200% 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .lyrics__line,
  .lyrics__skeleton {
    transition: none;
    animation: none;
  }
}
</style>
