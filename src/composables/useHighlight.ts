import { computed, type ComputedRef } from 'vue'
import { common, createLowlight } from 'lowlight'

/**
 * TipTap's code-block extension highlights through ProseMirror decorations, so
 * the highlight spans never appear in `editor.getHTML()`. Whatever we render for
 * readers therefore has to be highlighted again, from the stored
 * `<pre><code class="language-x">` markup — which is what this module does.
 *
 * `common` bundles ~37 languages and keeps the bundle far smaller than
 * registering every highlight.js grammar.
 */
export const lowlight = createLowlight(common)

/** Languages lowlight can actually highlight, for the editor's language picker. */
export function registeredLanguages(): string[] {
  return lowlight.listLanguages().sort()
}

/** Friendly labels for the languages the picker offers. */
const LANGUAGE_LABELS: Record<string, string> = {
  bash: 'Bash',
  c: 'C',
  cpp: 'C++',
  csharp: 'C#',
  css: 'CSS',
  diff: 'Diff',
  go: 'Go',
  graphql: 'GraphQL',
  ini: 'INI',
  java: 'Java',
  javascript: 'JavaScript',
  json: 'JSON',
  kotlin: 'Kotlin',
  less: 'Less',
  lua: 'Lua',
  makefile: 'Makefile',
  markdown: 'Markdown',
  objectivec: 'Objective-C',
  perl: 'Perl',
  php: 'PHP',
  plaintext: '纯文本',
  python: 'Python',
  r: 'R',
  ruby: 'Ruby',
  rust: 'Rust',
  scss: 'SCSS',
  shell: 'Shell',
  sql: 'SQL',
  swift: 'Swift',
  typescript: 'TypeScript',
  vbnet: 'VB.NET',
  wasm: 'WebAssembly',
  xml: 'XML',
  yaml: 'YAML',
}

export interface LanguageOption {
  value: string
  label: string
}

/** Only languages that are both curated above and registered in lowlight. */
export const CODE_LANGUAGE_OPTIONS: LanguageOption[] = [
  { value: '', label: '纯文本' },
  ...registeredLanguages()
    .filter((name) => name in LANGUAGE_LABELS && name !== 'plaintext')
    .map((name) => ({ value: name, label: LANGUAGE_LABELS[name] as string })),
]

/**
 * Minimal hast shape. Declared locally rather than imported from `hast` so this
 * module keeps working under pnpm's strict layout, where a transitive type-only
 * dependency is not resolvable from our source.
 */
interface HastText {
  type: 'text'
  value: string
}

interface HastElement {
  type: 'element'
  tagName: string
  properties?: Record<string, unknown>
  children?: HastNode[]
}

interface HastRoot {
  type: 'root'
  children?: HastNode[]
}

type HastNode = HastText | HastElement | HastRoot

function escapeHtml(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function renderAttributes(properties: Record<string, unknown> | undefined): string {
  if (!properties) return ''
  const parts: string[] = []
  for (const [key, value] of Object.entries(properties)) {
    if (value === null || value === undefined || value === false) continue
    const text = Array.isArray(value) ? value.join(' ') : String(value)
    // hast stores the class list under `className`, not `class`.
    const name = key === 'className' ? 'class' : key
    parts.push(`${escapeHtml(name)}="${escapeHtml(text)}"`)
  }
  return parts.length > 0 ? ` ${parts.join(' ')}` : ''
}

function renderHast(node: HastNode): string {
  if (node.type === 'text') return escapeHtml(node.value)
  const children = (node.children ?? []).map(renderHast).join('')
  if (node.type === 'root') return children
  const open = `<${node.tagName}${renderAttributes(node.properties)}>`
  return `${open}${children}</${node.tagName}>`
}

/** Highlight one code block, or `null` when the language is unknown. */
export function highlightCode(language: string, code: string): string | null {
  const name = language.trim().toLowerCase()
  if (!name || !lowlight.registered(name)) return null
  try {
    return renderHast(lowlight.highlight(name, code) as unknown as HastNode)
  } catch {
    // A broken grammar should degrade to unhighlighted code, never break a page.
    return null
  }
}

const LANGUAGE_CLASS = /(?:^|\s)language-([\w+#.-]+)/

/**
 * Re-highlight every `<pre><code class="language-x">` in stored article HTML and
 * tag the `<pre>` with `data-language` so CSS can render the language badge.
 * Input is already sanitised server-side; this only ever adds `<span>`s.
 */
export function highlightCodeBlocks(html: string): string {
  if (!html || typeof window === 'undefined' || !html.includes('language-')) return html
  try {
    const doc = new DOMParser().parseFromString(html, 'text/html')
    for (const element of Array.from(doc.querySelectorAll('pre > code'))) {
      const match = LANGUAGE_CLASS.exec(element.className)
      const name = match?.[1]
      if (!name) continue
      const source = element.textContent ?? ''
      const highlighted = highlightCode(name, source)
      if (!highlighted) continue
      element.innerHTML = highlighted
      ;(element.parentElement as HTMLElement | null)?.setAttribute('data-language', name)
    }
    return doc.body.innerHTML
  } catch {
    return html
  }
}

/** Reactive wrapper: re-highlights whenever the underlying HTML changes. */
export function useHighlightedHtml(source: () => string): ComputedRef<string> {
  return computed(() => highlightCodeBlocks(source()))
}
