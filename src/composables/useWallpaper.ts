/**
 * Wallpaper selection.
 *
 * Module-level state, for the same reason the site profile is a singleton: a
 * client-side route change must not re-roll the background, so a visitor gets
 * one random wallpaper per page load ("每次进入随机显示") and keeps it while
 * browsing. `rotate()` re-rolls on demand for the admin preview.
 */
import { computed, readonly, ref, watch } from 'vue'
import { useSite } from '@/composables/useSite'

const pool = ref<string[]>([])
const current = ref('')
/** Bumped on every successful roll so the stage can replay its entrance. */
const revision = ref(0)

/** Recently shown URLs, so consecutive rolls cannot bounce between two images. */
let recent: string[] = []

function pickRandom(list: string[]): string {
  const fresh = list.filter((url) => !recent.includes(url))
  const source = fresh.length > 0 ? fresh : list
  return source[Math.floor(Math.random() * source.length)] ?? ''
}

function roll(force = false): void {
  const list = pool.value
  if (list.length === 0) {
    current.value = ''
    return
  }

  // Keep what is already painted unless the caller asked for a new one or the
  // image left the pool (owner removed it, or a rescan rewrote the list).
  if (!force && current.value && list.includes(current.value)) return

  const next = pickRandom(list)
  if (!next) return

  current.value = next
  revision.value += 1
  // Remember enough history to avoid an immediate repeat, but never so much
  // that a small pool has no "fresh" candidates left.
  const window = Math.max(1, Math.min(3, list.length - 1))
  recent = [next, ...recent.filter((url) => url !== next)].slice(0, window)
}

const { site } = useSite()

watch(
  () => site.value?.wallpapers ?? [],
  (list) => {
    // A new array identity every evaluation is harmless here: the getter only
    // depends on the site ref, so this fires when the profile arrives or is
    // saved, not on every render.
    pool.value = [...list]
    roll()
  },
  { immediate: true },
)

export function useWallpaper() {
  return {
    pool: readonly(pool),
    current: readonly(current),
    revision: readonly(revision),
    hasWallpaper: computed(() => current.value !== ''),
    /** Pick a different wallpaper from the pool (crossfades in the stage). */
    rotate: () => roll(true),
  }
}
