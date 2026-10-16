/**
 * LRC lyrics parsing.
 *
 * Deliberately forgiving: sidecar files in the wild mix `[mm:ss]`,
 * `[mm:ss.xx]` and the older `[mm:ss:xx]` decimal form, carry a leading
 * metadata block, and often repeat a timestamp to put one line under two
 * moments. Anything unrecognised is skipped rather than failing the parse —
 * worse lyrics are better than no player.
 */

export interface LrcLine {
  /** Milliseconds from the start of the track. */
  time: number
  /** The original lyric. */
  text: string
  /**
   * The translation of `text`, when the file is bilingual. Multiline when the
   * file stacked several translations on the same timestamp.
   */
  translation?: string
}

export interface ParsedLyrics {
  /** Timed lines, sorted, empty when the file carries no timestamps. */
  lines: LrcLine[]
  /** Untimed fallback: the file was plain text rather than LRC. */
  plain: string[]
}

/** `[00:12.34]`, `[00:12]`, `[00:12:34]`, `[0:12.3]` */
const TIME_TAG = /\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]/g

/** `[offset:+250]` — a whole-file correction, in milliseconds. */
const OFFSET_TAG = /\[offset:\s*([+-]?\d+)\s*\]/i

export function parseLyrics(raw: string): ParsedLyrics {
  const text = raw.replace(/\r\n?/g, '\n')
  if (!text.trim()) return { lines: [], plain: [] }

  const offsetMatch = OFFSET_TAG.exec(text)
  const offset = offsetMatch?.[1] ? Number(offsetMatch[1]) : 0

  /**
   * One entry per timestamp, in file order. The first row that claims a moment
   * is the original lyric; every further row on that same timestamp is read as
   * a translation of it, which is how a bilingual sidecar is written:
   *
   *   [00:12.00]夜に駆ける
   *   [00:12.00]奔向黑夜
   */
  const byTime = new Map<number, { text: string; translations: string[] }>()
  const plain: string[] = []

  for (const row of text.split('\n')) {
    TIME_TAG.lastIndex = 0

    let match: RegExpExecArray | null
    const stamps: number[] = []

    while ((match = TIME_TAG.exec(row)) !== null) {
      const minutes = Number(match[1])
      const seconds = Number(match[2])
      // A one- or two-digit fraction means centiseconds, three means
      // milliseconds. Padding to three digits makes `[00:01.5]` and
      // `[00:01.500]` agree instead of landing 500ms apart.
      const fraction = match[3] ? Number(match[3].padEnd(3, '0')) : 0
      stamps.push(minutes * 60_000 + seconds * 1000 + fraction)
    }

    // Strip every recognised tag so the lyric text itself is what remains.
    const body = row.replace(TIME_TAG, '').replace(OFFSET_TAG, '').trim()

    if (stamps.length === 0) {
      // A metadata row such as [ti:…] or a plain-text lyric line.
      if (body && !/^\[[a-z#]+:.*\]$/i.test(row.trim())) plain.push(body)
      continue
    }

    // Empty timed rows are karaoke gaps; showing a blank highlight is worse
    // than skipping the row and letting the previous line linger.
    if (!body) continue

    // A row may carry several tags (a repeated chorus); each one points at the
    // same words. A set collapses a row that repeats one timestamp so it cannot
    // become its own translation.
    for (const time of new Set(stamps)) {
      const at = Math.max(0, time + offset)
      const entry = byTime.get(at)

      if (!entry) {
        byTime.set(at, { text: body, translations: [] })
      } else if (entry.text !== body && !entry.translations.includes(body)) {
        entry.translations.push(body)
      }
    }
  }

  if (byTime.size === 0) return { lines: [], plain }

  const lines: LrcLine[] = [...byTime.entries()]
    .map(([time, entry]) =>
      entry.translations.length > 0
        ? { time, text: entry.text, translation: entry.translations.join('\n') }
        : { time, text: entry.text },
    )
    .sort((a, b) => a.time - b.time)

  return { lines, plain: [] }
}

/**
 * Index of the line that should be highlighted at `timeMs`, or -1 before the
 * first line starts.
 *
 * Binary search rather than a scan: this runs on every animation frame while
 * the track plays, and lyrics files can run to a few hundred rows.
 */
export function findActiveLine(lines: LrcLine[], timeMs: number): number {
  if (lines.length === 0) return -1

  let low = 0
  let high = lines.length - 1
  let found = -1

  while (low <= high) {
    const mid = (low + high) >> 1
    const line = lines[mid]
    if (!line) break
    if (line.time <= timeMs) {
      found = mid
      low = mid + 1
    } else {
      high = mid - 1
    }
  }

  return found
}
