import { readonly, ref } from 'vue'
import { getLimits } from '@/api/limits'
import type { Limits } from '@/types/limits'

/**
 * Mirrors the code defaults in the backend (internal/conf + internal/biz).
 *
 * The store starts here so a form never renders a blank hint: while the request
 * is in flight — or if it fails — the page still says "10 MB" rather than
 * "NaN MB". The load replaces this with what the server reports, which is how a
 * value edited in config.yaml reaches the UI.
 */
const FALLBACK: Limits = {
  media: {
    maxUploadBytes: 10 * 1024 * 1024,
    allowedTypes: ['image/gif', 'image/jpeg', 'image/png', 'image/webp'],
  },
  music: {
    maxUploadBytes: 32 * 1024 * 1024,
    maxCoverBytes: 5 * 1024 * 1024,
    maxLyricsBytes: 1024 * 1024,
  },
  site: {
    maxSocials: 12,
    maxWallpapers: 24,
    maxWallpaperLength: 512,
    maxTitleLength: 60,
    maxTaglineLength: 100,
    maxDescriptionLength: 300,
    maxAuthorLength: 40,
    maxFooterLength: 120,
    maxSocialLabelLength: 40,
  },
}

const limits = ref<Limits>(FALLBACK)

let loaded = false
let inflight: Promise<void> | null = null

/** Loads the policy once; concurrent callers share the same request. */
async function load(force = false): Promise<void> {
  if (loaded && !force) return
  if (inflight) return inflight

  inflight = (async () => {
    try {
      limits.value = await getLimits()
      loaded = true
    } catch {
      // A failure here must not break the page it decorates: the fallback is
      // always a valid policy, just not necessarily this deployment's.
    } finally {
      inflight = null
    }
  })()

  return inflight
}

export function useLimits() {
  return { limits: readonly(limits), load }
}
