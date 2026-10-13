import { onBeforeUnmount, ref } from 'vue'

interface TypewriterOptions {
  /** ms per character. */
  speed?: number
  /** ms to hold the finished line before restarting. */
  hold?: number
  /** Cursor pause after each line. */
  pause?: number
}

/**
 * A tiny terminal-style typewriter. Hand-rolled rather than pulled from npm:
 * the whole feature is ~40 lines and this keeps the dependency list small.
 * Under prefers-reduced-motion it renders the first line instantly.
 */
export function useTypewriter(lines: string[], options: TypewriterOptions = {}) {
  const { speed = 62, hold = 2200, pause = 420 } = options

  const text = ref('')
  const lineIndex = ref(0)

  const reduced =
    typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

  let timer: ReturnType<typeof setTimeout> | null = null
  let cursor = 0
  let stopped = false

  function clear(): void {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  function typeLine(): void {
    if (stopped) return
    const line = lines[lineIndex.value] ?? ''
    if (cursor <= line.length) {
      text.value = line.slice(0, cursor)
      cursor += 1
      timer = setTimeout(typeLine, speed)
      return
    }
    timer = setTimeout(eraseLine, hold)
  }

  function eraseLine(): void {
    if (stopped) return
    if (cursor > 0) {
      cursor -= 1
      text.value = (lines[lineIndex.value] ?? '').slice(0, cursor)
      timer = setTimeout(eraseLine, speed / 2)
      return
    }
    lineIndex.value = (lineIndex.value + 1) % Math.max(1, lines.length)
    timer = setTimeout(typeLine, pause)
  }

  function start(): void {
    if (reduced) {
      text.value = lines[0] ?? ''
      return
    }
    stopped = false
    clear()
    cursor = 0
    lineIndex.value = 0
    text.value = ''
    timer = setTimeout(typeLine, 320)
  }

  function stop(): void {
    stopped = true
    clear()
  }

  onBeforeUnmount(stop)

  return { text, lineIndex, start, stop }
}
