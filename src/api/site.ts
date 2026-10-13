import { http } from '@/api/http'
import type { SiteMeta } from '@/types/blog'

/** Public site metadata: title, tagline, socials, aggregate stats. */
export function getSite(signal?: AbortSignal): Promise<SiteMeta> {
  return http.get<SiteMeta>('/site', undefined, signal)
}

export function updateSite(meta: Partial<SiteMeta>): Promise<SiteMeta> {
  return http.put<SiteMeta>('/admin/site', meta)
}
