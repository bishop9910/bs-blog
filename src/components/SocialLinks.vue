<script setup lang="ts">
import {
  PhEnvelopeSimple,
  PhGithubLogo,
  PhLink,
  PhRss,
  PhRssSimple,
  PhXLogo,
} from '@phosphor-icons/vue'
import type { SocialLink } from '@/types/blog'

withDefaults(
  defineProps<{
    // `readonly` because useSite() exposes a readonly ref, and its SiteMeta is
    // therefore deeply readonly at the call sites.
    socials: readonly SocialLink[]
    size?: number
  }>(),
  { size: 18 },
)

type IconComponent = typeof PhLink

/**
 * The backend normalises unknown icon keys to `link`, so a missing key here is
 * only ever a forward-compatibility case.
 */
const ICONS: Record<string, IconComponent> = {
  github: PhGithubLogo,
  x: PhXLogo,
  email: PhEnvelopeSimple,
  rss: PhRss,
  link: PhLink,
}

function iconFor(name: string): IconComponent {
  return ICONS[name] ?? PhLink
}

// Referenced so the tree-shaker keeps the alias used by the type above.
void PhRssSimple
</script>

<template>
  <ul class="socials">
    <li v-for="(link, index) in socials" :key="`${link.icon}-${link.url}-${index}`">
      <a
        class="socials__link"
        :href="link.url"
        :title="link.label || link.icon"
        :aria-label="link.label || link.icon"
        target="_blank"
        rel="noopener noreferrer"
      >
        <component :is="iconFor(link.icon)" :size="size" />
      </a>
    </li>
  </ul>
</template>

<style scoped>
.socials {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin: 0;
  padding: 0;
  list-style: none;
}

.socials__link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  /* Comfortably past the 44px minimum touch target even though the glyph is small. */
  width: var(--hit-min);
  height: var(--hit-min);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  transition:
    color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out),
    transform var(--dur-fast) var(--ease-out);
}

.socials__link:hover {
  color: var(--accent);
  background: var(--bg-hover);
  transform: translateY(-2px);
}

.socials__link:active {
  transform: translateY(0) scale(0.94);
}
</style>
