/**
 * Text helpers shared by the editor, the slug field and the post list.
 *
 * IMPORTANT: these must stay byte-for-byte identical in behaviour to
 * backend/internal/pkg/textutil/textutil.go. The server recomputes the slug,
 * the word count and the reading time when a post is saved, so any drift here
 * shows the author one value in the editor and stores a different one.
 *
 * The shared expectations live in scripts/text-cases.json and are asserted on
 * both sides (scripts/check-text-contract.mjs here, textutil_test.go in Go).
 */

/**
 * CJK code points that are counted per glyph rather than per whitespace-
 * delimited word: ext-A + unified ideographs + compatibility ideographs,
 * hiragana/katakana and hangul syllables.
 */
const CJK_PATTERN = /[\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\u3040-\u30ff\uac00-\ud7af]/g

/** Runs of Latin letters/digits. Matches the Go side's `[A-Za-z0-9]+`. */
const LATIN_PATTERN = /[A-Za-z0-9]+/g

const MAX_SLUG_SOURCE = 200
const MAX_SLUG_LENGTH = 80

/**
 * URL-safe slug. Keeps CJK characters rather than stripping them, because a
 * Chinese title would otherwise slugify to an empty string. The backend
 * percent-encodes these in the path.
 *
 * Punctuation becomes a separator (not nothing), so "a.b" and "a-b" agree
 * instead of "a.b" collapsing into "ab".
 */
export function slugify(input: string): string {
  const trimmed = Array.from(input.trim()).slice(0, MAX_SLUG_SOURCE).join('')
  if (!trimmed) return ''

  const decomposed = trimmed.toLowerCase().normalize('NFKD')

  // Combining accents left behind by NFKD are dropped; letters and digits
  // survive (this is what keeps 中文 slugs alive); everything else - spaces,
  // punctuation, symbols - becomes a separator.
  const separated = decomposed
    .replace(/\p{Mn}/gu, '')
    .replace(/[^\p{Letter}\p{Number}]/gu, '-')
    // Recompose. NFKD split Hangul syllables into jamo, so without this a
    // Korean title slugs to a run of jamo rather than readable syllables.
    .normalize('NFC')

  const slug = separated.replace(/-{2,}/g, '-').replace(/^-+|-+$/g, '')

  const points = Array.from(slug)
  if (points.length <= MAX_SLUG_LENGTH) return slug
  // Slice by code point (not UTF-16 unit) and re-trim: the cut can land on a
  // separator, and a slug should never end in a dash.
  return points.slice(0, MAX_SLUG_LENGTH).join('').replace(/^-+|-+$/g, '')
}

/** Plain text from an HTML string, for excerpts and counting. */
export function stripHtml(html: string): string {
  if (!html) return ''
  const withoutTags = html
    .replace(/<(script|style)[\s\S]*?<\/\1>/gi, ' ')
    .replace(/<[^>]+>/g, ' ')
  // Decode in a detached element so entities render as characters.
  if (typeof document !== 'undefined') {
    const el = document.createElement('textarea')
    el.innerHTML = withoutTags
    return el.value.replace(/\s+/g, ' ').trim()
  }
  return withoutTags.replace(/\s+/g, ' ').trim()
}

/**
 * Word count that treats each CJK ideograph as one word and runs of Latin
 * letters as one word — otherwise Chinese posts report a near-zero count.
 */
export function countWords(html: string): number {
  const text = stripHtml(html)
  if (!text) return 0

  const cjk = text.match(CJK_PATTERN)
  const latin = text.match(LATIN_PATTERN)

  return (cjk?.length ?? 0) + (latin?.length ?? 0)
}

/** Reading time in minutes, floored at 1. ~350 CJK chars or 200 words per minute. */
export function readingMinutes(html: string): number {
  const text = stripHtml(html)
  if (!text) return 1

  const cjk = text.match(CJK_PATTERN)?.length ?? 0
  const words = text.match(LATIN_PATTERN)?.length ?? 0

  const minutes = cjk / 350 + words / 200
  if (minutes < 1) return 1
  // Round up: "2.1 minutes" reading as 3 is friendlier than 2.
  return Math.floor(minutes + 0.999)
}

/** Truncate on a word/character boundary with an ellipsis. */
export function truncate(input: string, max = 120): string {
  const text = input.trim()
  if (text.length <= max) return text
  return `${text.slice(0, max).trimEnd()}…`
}

/** Derive a summary from post HTML when the author left it blank. */
export function autoExcerpt(html: string, max = 120): string {
  return truncate(stripHtml(html), max)
}
