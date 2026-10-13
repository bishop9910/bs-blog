/**
 * Minimal typed fetch wrapper.
 *
 * Deliberately dependency-free: the whole HTTP surface this app needs is
 * JSON requests, bearer-token auth, form uploads and consistent errors.
 */

export const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? '/api/v1').replace(/\/$/, '')

const TOKEN_KEY = 'bs-blog.token'

/* -------------------------------------------------------------------------- */
/* Token storage                                                              */
/* -------------------------------------------------------------------------- */

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

export function setToken(token: string | null): void {
  try {
    if (token === null) localStorage.removeItem(TOKEN_KEY)
    else localStorage.setItem(TOKEN_KEY, token)
  } catch {
    /* storage unavailable (private mode) — session stays in memory only */
  }
}

/* -------------------------------------------------------------------------- */
/* Errors                                                                     */
/* -------------------------------------------------------------------------- */

export interface ApiErrorBody {
  code?: number | string
  reason?: string
  message?: string
  metadata?: Record<string, string>
}

export class ApiError extends Error {
  readonly status: number
  readonly code: number | string
  readonly reason: string
  readonly details?: Record<string, string>

  constructor(status: number, body: ApiErrorBody) {
    super(body.message || body.reason || `Request failed with status ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.code = body.code ?? status
    this.reason = body.reason ?? ''
    this.details = body.metadata
  }

  get isUnauthorized(): boolean {
    return this.status === 401
  }

  get isNetwork(): boolean {
    return this.status === 0
  }
}

/* -------------------------------------------------------------------------- */
/* Core                                                                       */
/* -------------------------------------------------------------------------- */

type QueryValue = string | number | boolean | undefined | null

export function buildQuery(params: Record<string, QueryValue>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    search.set(key, String(value))
  }
  const qs = search.toString()
  return qs ? `?${qs}` : ''
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  /** Serialised as JSON unless it is already a BodyInit. */
  body?: unknown
  query?: Record<string, QueryValue>
  /** Send without the Authorization header. */
  anonymous?: boolean
  signal?: AbortSignal
}

/** Called on any 401 so the app can bounce to the login screen. */
let onUnauthorized: (() => void) | null = null

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  onUnauthorized = handler
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, query, anonymous = false, signal } = options

  const headers = new Headers({ Accept: 'application/json' })
  const isForm = body instanceof FormData

  if (body !== undefined && !isForm) headers.set('Content-Type', 'application/json')

  if (!anonymous) {
    const token = getToken()
    if (token) headers.set('Authorization', `Bearer ${token}`)
  }

  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}${query ? buildQuery(query) : ''}`, {
      method,
      headers,
      body: body === undefined ? undefined : isForm ? (body as FormData) : JSON.stringify(body),
      signal,
    })
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === 'AbortError') throw cause
    throw new ApiError(0, { message: '无法连接到服务器，请检查后端是否运行' })
  }

  if (response.status === 204) return undefined as T

  const text = await response.text()
  let payload: unknown = undefined
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = { message: text.slice(0, 300) }
    }
  }

  if (!response.ok) {
    if (response.status === 401 && !anonymous) {
      setToken(null)
      onUnauthorized?.()
    }
    throw new ApiError(response.status, (payload ?? {}) as ApiErrorBody)
  }

  return payload as T
}

/* -------------------------------------------------------------------------- */
/* Convenience helpers                                                        */
/* -------------------------------------------------------------------------- */

export const http = {
  get: <T>(path: string, query?: Record<string, QueryValue>, signal?: AbortSignal) =>
    request<T>(path, { method: 'GET', query, signal }),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body }),
  put: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PUT', body }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PATCH', body }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  upload: <T>(path: string, form: FormData) => request<T>(path, { method: 'POST', body: form }),
}
