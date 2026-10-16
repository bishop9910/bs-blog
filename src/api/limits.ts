import { http } from './http'
import type { Limits } from '@/types/limits'

/** The size and field caps this deployment enforces. Requires a session. */
export function getLimits(signal?: AbortSignal): Promise<Limits> {
  return http.get<Limits>('/admin/limits', undefined, signal)
}
