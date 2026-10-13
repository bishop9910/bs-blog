/**
 * Auth state as a module-scope singleton.
 *
 * No Pinia: this app has exactly one shared store, and a plain reactive
 * object avoids a dependency. Import `useAuth()` anywhere.
 */

import { computed, readonly, ref } from 'vue'
import * as authApi from '@/api/auth'
import { getToken, setToken, setUnauthorizedHandler } from '@/api/http'
import type { User } from '@/types/blog'

const user = ref<User | null>(null)
const loading = ref(false)
/** True once the token has been checked (or proven absent) this session. */
const resolved = ref(false)

export const isAuthenticated = computed(() => user.value !== null)

/** Clears local session state. Does not call the API. */
function clear(): void {
  user.value = null
  setToken(null)
}

/**
 * Validates the persisted token against the backend.
 * Safe to call repeatedly; concurrent calls share one request.
 */
let inflight: Promise<boolean> | null = null

function restore(): Promise<boolean> {
  if (resolved.value) return Promise.resolve(user.value !== null)
  if (inflight) return inflight

  const token = getToken()
  if (!token) {
    resolved.value = true
    return Promise.resolve(false)
  }

  inflight = authApi
    .me()
    .then((me) => {
      user.value = me
      return true
    })
    .catch(() => {
      clear()
      return false
    })
    .finally(() => {
      resolved.value = true
      inflight = null
    })

  return inflight
}

async function signIn(username: string, password: string): Promise<void> {
  loading.value = true
  try {
    const result = await authApi.login(username, password)
    setToken(result.token)
    user.value = result.user
    resolved.value = true
  } finally {
    loading.value = false
  }
}

async function signOut(): Promise<void> {
  try {
    await authApi.logout()
  } catch {
    /* the token is being discarded regardless */
  } finally {
    clear()
  }
}

async function claim(username: string, password: string): Promise<void> {
  const result = await authApi.setup(username, password)
  setToken(result.token)
  user.value = result.user
  resolved.value = true
}

/** Hook the 401 path in the HTTP layer to the local session. */
setUnauthorizedHandler(() => clear())

export function useAuth() {
  return {
    user: readonly(user),
    loading: readonly(loading),
    resolved: readonly(resolved),
    isAuthenticated,
    restore,
    signIn,
    signOut,
    claim,
    clear,
  }
}
