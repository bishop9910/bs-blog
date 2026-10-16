<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { PhCaretDown, PhListDashes } from '@phosphor-icons/vue'
import { motionStagger } from '@/utils/anime'
import { scrollToElement } from '@/composables/useSmoothScroll'
import { slugify } from '@/utils/text'

/**
 * Table of contents for a rendered article.
 *
 * Two shapes, one instance: a panel pinned in the right gutter from 1200px up,
 * and a collapsed `<details>` inside the flow below that. They share the same
 * entries and the same active id, so the highlight survives a resize across the
 * breakpoint. The whole component renders nothing when the article has no h2/h3
 * — an empty "目录" card is worse than no table of contents at all.
 */

interface TocEntry {
  id: string
  text: string
  level: 2 | 3
}

const props = defineProps<{
  /**
   * Resolves the rendered article body (the element the headings live in).
   *
   * A getter rather than an element: the parent's template ref is filled in the
   * patch that mounts the article, which is after this component's props are
   * evaluated, and assigning a template ref never re-renders the parent — an
   * element prop would therefore stay `null`.
   */
  resolveContent: () => HTMLElement | null
}>()

const items = ref<TocEntry[]>([])
const activeId = ref('')
const listRef = ref<HTMLElement | null>(null)
const detailsRef = ref<HTMLDetailsElement | null>(null)

let observer: IntersectionObserver | null = null

/**
 * The line a heading has to clear to sit below the sticky header. Mirrors the
 * `scroll-margin-top` base.css already gives `.prose` headings, so a link in
 * this panel and a pasted `#hash` land the reader in the same place.
 */
function anchorOffset(): number {
  const styles = getComputedStyle(document.documentElement)
  const header = Number.parseFloat(styles.getPropertyValue('--header-h'))
  const gap = Number.parseFloat(styles.getPropertyValue('--space-lg'))
  return (Number.isFinite(header) ? header : 64) + (Number.isFinite(gap) ? gap : 24)
}

/**
 * The section the reader is currently in: the last heading whose top has passed
 * the line under the header.
 *
 * IntersectionObserver is only the *signal* here. A heading that scrolled out
 * of its band is absent from the entries altogether, so the winner is always
 * read back from live geometry instead of being reconstructed from the entry
 * list — which is what makes the gaps between headings keep the previous
 * section highlighted instead of clearing it.
 */
function resolveActive(): string {
  const line = anchorOffset()
  let current = items.value[0]?.id ?? ''
  for (const item of items.value) {
    const heading = document.getElementById(item.id)
    if (!heading) continue
    if (heading.getBoundingClientRect().top > line) break
    current = item.id
  }
  return current
}

function observe() {
  observer?.disconnect()
  observer = null
  if (items.value.length === 0 || typeof IntersectionObserver === 'undefined') return

  observer = new IntersectionObserver(
    () => {
      activeId.value = resolveActive()
    },
    {
      // A band under the header: anything crossing its edges re-ranks, and in
      // the gaps between headings no entry fires and the winner simply stays.
      rootMargin: `-${Math.round(anchorOffset())}px 0px -55% 0px`,
      threshold: 0,
    },
  )

  for (const item of items.value) {
    const heading = document.getElementById(item.id)
    if (heading) observer.observe(heading)
  }
  activeId.value = resolveActive()
}

/** Collect h2/h3 in document order, giving the ones without an id a stable one. */
async function collect() {
  await nextTick()
  const root = props.resolveContent()
  const found: TocEntry[] = []

  if (root) {
    // h2/h3 only: the post title is the page's h1, and h4 subdivisions would
    // make the list longer than it is useful to scan.
    const taken = new Set<string>()
    const headings = Array.from(root.querySelectorAll<HTMLElement>('h2, h3'))

    headings.forEach((heading, index) => {
      const text = (heading.textContent ?? '').trim()
      if (!text) return
      // Reuse the site's slug rules so a hand-written `#anchor` keeps working;
      // duplicates get a numeric suffix rather than silently colliding.
      const base = heading.id.trim() || slugify(text) || `section-${index + 1}`
      let id = base
      let suffix = 2
      while (taken.has(id)) {
        id = `${base}-${suffix}`
        suffix += 1
      }
      taken.add(id)
      if (heading.id !== id) heading.id = id
      found.push({ id, text, level: heading.tagName === 'H2' ? 2 : 3 })
    })
  }

  items.value = found
  activeId.value = found[0]?.id ?? ''

  await nextTick()
  reveal()
  observe()
}

/** One light wave so the panel does not just blink into place. */
function reveal() {
  const list = listRef.value
  // Nothing is laid out when the panel is hidden by the breakpoint — skip the
  // animation rather than writing inline styles onto an invisible list.
  if (!list || list.getClientRects().length === 0) return
  // A no-op under reduced motion, where the markup is already in its end state.
  motionStagger(
    list.querySelectorAll<HTMLElement>('[data-toc-item]'),
    { opacity: [0, 1], translateX: [-6, 0], duration: 420 },
    { each: 26 },
  )
}

function go(item: TocEntry) {
  const heading = document.getElementById(item.id)
  if (!heading) return
  // Highlight straight away: the jump takes a moment and the reader should not
  // have to wait for the observer to catch up.
  activeId.value = item.id
  // Reuse the page's own smooth-scroll engine so a TOC jump, the layout's
  // skip-link and the "回到顶部" button all move the same way. The offset is the
  // heading's `scroll-margin-top` equivalent.
  scrollToElement(heading, anchorOffset())
  // On narrow screens the list covers the text it just jumped to.
  if (detailsRef.value) detailsRef.value.open = false
}

/**
 * The public layout's smooth scroller claims wheel events for the page, which
 * would leave a long TOC unreachable: the panel can scroll but never gets the
 * gesture. Keep the gesture for the panel only while it can actually move, and
 * let the page have it again at either end (and whenever the list fits), so
 * hovering the panel never feels dead.
 */
function onWheel(event: WheelEvent) {
  const list = listRef.value
  if (!list) return
  const overflow = list.scrollHeight - list.clientHeight
  if (overflow <= 1) return
  const atStart = event.deltaY < 0 && list.scrollTop <= 0
  const atEnd = event.deltaY > 0 && list.scrollTop >= overflow - 1
  if (atStart || atEnd) return
  event.stopPropagation()
}

onMounted(() => {
  void collect()
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
})
</script>

<template>
  <template v-if="items.length > 0">
    <nav class="toc" aria-labelledby="toc-title">
      <p id="toc-title" class="toc__title">目录</p>
      <!-- The panel scrolls itself; see onWheel for how that is shared with the
           page's smooth scroller. -->
      <div ref="listRef" class="toc__list" @wheel="onWheel">
        <a
          v-for="item in items"
          :key="item.id"
          class="toc__link"
          :class="{ 'toc__link--sub': item.level === 3, 'is-active': item.id === activeId }"
          :href="`#${item.id}`"
          :aria-current="item.id === activeId ? 'location' : undefined"
          data-toc-item
          @click.prevent="go(item)"
        >
          {{ item.text }}
        </a>
      </div>
    </nav>

    <details ref="detailsRef" class="toc-inline">
      <summary class="toc-inline__summary">
        <PhListDashes class="toc-inline__icon" :size="16" aria-hidden="true" />
        目录
        <PhCaretDown class="toc-inline__caret" :size="14" aria-hidden="true" />
      </summary>
      <nav class="toc-inline__nav" aria-label="文章目录">
        <a
          v-for="item in items"
          :key="item.id"
          class="toc-inline__link"
          :class="{ 'toc-inline__link--sub': item.level === 3, 'is-active': item.id === activeId }"
          :href="`#${item.id}`"
          :aria-current="item.id === activeId ? 'location' : undefined"
          @click.prevent="go(item)"
        >
          {{ item.text }}
        </a>
      </nav>
    </details>
  </template>
</template>

<style scoped>
/* --------------------------------------------------------------------------
   Wide screens: a panel in the gutter to the right of the article column.
   -------------------------------------------------------------------------- */
.toc {
  display: none;
}

@media (min-width: 1200px) {
  .toc {
    --toc-width: 200px;

    position: fixed;
    /* Below the sticky header, at the same offset the site's other sticky
       asides use, and inside the frame the header is centred in. */
    top: calc(var(--header-h) + var(--space-lg));
    right: max(var(--space-lg), calc((100vw - var(--container-xl)) / 2 + var(--space-lg)));
    z-index: calc(var(--z-sticky) - 1);
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    /* Never wider than the gutter that is actually free, so a large zoom level
       (which grows the article column) can never make the panel overlap it. */
    width: min(var(--toc-width), calc(50vw - var(--measure) / 2 - var(--space-lg)));
    max-height: calc(100vh - var(--header-h) - var(--space-4xl));
    overflow-y: auto;
    overscroll-behavior: contain;
    /* `ch` resolves against this element's font, so matching the article's size
       is what makes the gutter arithmetic above agree with .prose. */
    font-size: var(--text-lg);
  }

  .toc__list {
    display: flex;
    flex-direction: column;
  }
}

.toc__title {
  margin: 0;
  padding-left: var(--space-md);
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-caps);
  text-transform: uppercase;
}

.toc__link {
  display: block;
  padding: var(--space-sm) var(--space-sm) var(--space-sm) var(--space-md);
  border-left: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-snug);
  text-decoration: none;
  transition:
    color var(--dur-fast) var(--ease-out),
    border-color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.toc__link:hover {
  border-left-color: var(--border-strong);
  color: var(--text-primary);
}

/* Second level, indented so the outline is readable at a glance. */
.toc__link--sub {
  padding-left: var(--space-2xl);
  font-size: var(--text-xs);
}

.toc__link.is-active {
  border-left-color: var(--accent);
  background: var(--accent-subtle);
  box-shadow: inset 2px 0 0 var(--accent);
  color: var(--accent);
}

/* --------------------------------------------------------------------------
   Narrow screens: the same list, collapsed inside the article.
   -------------------------------------------------------------------------- */
.toc-inline {
  max-width: var(--measure);
  margin: 0 auto var(--space-xl);
  padding: 0 var(--space-lg);
  /* Matches .prose's `ch`, so the card lines up with the body it describes. */
  font-size: var(--text-lg);
}

.toc-inline__summary {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  min-height: var(--hit-min);
  padding: 0 var(--space-md);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-card);
  color: var(--text-secondary);
  /* The block's font is the article's (it sizes `ch`); the label is not. */
  font-size: var(--text-sm);
  cursor: pointer;
  list-style: none;
  transition:
    border-color var(--dur-fast) var(--ease-out),
    color var(--dur-fast) var(--ease-out);
}

.toc-inline__summary::-webkit-details-marker {
  display: none;
}

.toc-inline[open] .toc-inline__summary {
  border-bottom-right-radius: 0;
  border-bottom-left-radius: 0;
  border-color: var(--border-strong);
  color: var(--text-primary);
}

.toc-inline__summary:hover {
  border-color: var(--border-accent);
  color: var(--text-primary);
}

.toc-inline__summary > :first-child {
  color: var(--accent);
}

.toc-inline__caret {
  margin-left: auto;
  transition: transform var(--dur-normal) var(--ease-out);
}

.toc-inline[open] .toc-inline__caret {
  transform: rotate(180deg);
}

.toc-inline__nav {
  display: flex;
  flex-direction: column;
  padding: var(--space-sm) 0;
  border: 1px solid var(--border-strong);
  border-top: 0;
  border-radius: 0 0 var(--radius-md) var(--radius-md);
  background: var(--bg-card);
}

.toc-inline__link {
  display: flex;
  align-items: center;
  min-height: var(--hit-min);
  padding: 0 var(--space-md);
  border-left: 2px solid transparent;
  color: var(--text-secondary);
  font-size: var(--text-sm);
  line-height: var(--leading-snug);
  text-decoration: none;
}

.toc-inline__link--sub {
  padding-left: var(--space-2xl);
  font-size: var(--text-xs);
}

.toc-inline__link:hover {
  color: var(--text-primary);
}

.toc-inline__link.is-active {
  border-left-color: var(--accent);
  background: var(--accent-subtle);
  color: var(--accent);
}

@media (min-width: 1200px) {
  .toc-inline {
    display: none;
  }
}
</style>
