import { computed, readonly, ref, watch } from 'vue'

export type ThemeMode = 'dark' | 'light' | 'system'

const STORAGE_KEY = 'bs-blog.theme'

function readStored(): ThemeMode {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw === 'dark' || raw === 'light' || raw === 'system') return raw
  } catch {
    /* ignore */
  }
  return 'system'
}

const mode = ref<ThemeMode>(readStored())

const media = typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)') : null
const systemDark = ref(media ? media.matches : true)

media?.addEventListener('change', (event) => {
  systemDark.value = event.matches
})

/** The mode actually painted right now. */
function resolve(m: ThemeMode): 'dark' | 'light' {
  return m === 'system' ? (systemDark.value ? 'dark' : 'light') : m
}

function apply(): void {
  const effective = resolve(mode.value)
  document.documentElement.dataset.theme = effective
  // Keep mobile browser chrome in step with the page.
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', effective === 'dark' ? '#09090b' : '#fafafa')
}

watch([mode, systemDark], () => {
  apply()
  try {
    localStorage.setItem(STORAGE_KEY, mode.value)
  } catch {
    /* ignore */
  }
})

/** Called once from main.ts, before mount, to avoid a flash of the wrong theme. */
export function initTheme(): void {
  apply()
}

export function useTheme() {
  return {
    mode: readonly(mode),
    /** 'dark' | 'light' — never 'system'. */
    effective: computed(() => resolve(mode.value)),
    setMode: (next: ThemeMode) => {
      mode.value = next
    },
    /** Cycles dark -> light -> system, for a single-button toggle. */
    toggle: () => {
      const order: ThemeMode[] = ['dark', 'light', 'system']
      const next = order[(order.indexOf(mode.value) + 1) % order.length]
      mode.value = next ?? 'system'
    },
  }
}
