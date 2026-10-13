import { readonly, ref } from 'vue'
import { getSite } from '@/api/site'
import type { SiteMeta } from '@/types/blog'

/**
 * Site profile is needed by the header, the footer, the home hero and the about
 * page, so it lives as a module-level singleton (like the auth store) rather
 * than being refetched per view. Concurrent callers share one in-flight request.
 */
const site = ref<SiteMeta | null>(null)
const loading = ref(false)
let inflight: Promise<SiteMeta | null> | null = null

async function load(force = false): Promise<SiteMeta | null> {
  if (site.value && !force) return site.value
  if (inflight) return inflight

  loading.value = true
  inflight = getSite()
    .then((meta) => {
      site.value = meta
      return meta
    })
    .catch(() => site.value)
    .finally(() => {
      loading.value = false
      inflight = null
    })

  return inflight
}

function setSite(meta: SiteMeta) {
  site.value = meta
}

export function useSite() {
  return {
    site: readonly(site),
    loading: readonly(loading),
    load,
    setSite,
  }
}
