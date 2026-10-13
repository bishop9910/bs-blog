import { http } from '@/api/http'
import type { LoginResult, User } from '@/types/blog'

/** Exchange the admin password for a bearer token. */
export function login(username: string, password: string): Promise<LoginResult> {
  return http.post<LoginResult>('/auth/login', { username, password })
}

export function logout(): Promise<void> {
  return http.post<void>('/auth/logout')
}

/** Validates the stored token and returns the current admin. */
export function me(signal?: AbortSignal): Promise<User> {
  return http.get<{ user: User }>('/auth/me', undefined, signal).then((r) => r.user)
}

export function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  return http.post<void>('/auth/password', { oldPassword, newPassword })
}

/** Unauthenticated probe so the login screen can say "not initialised yet". */
export function setupState(): Promise<{ initialised: boolean }> {
  return http.get<{ initialised: boolean }>('/auth/setup')
}

/** First-run only: claims the blog by setting the admin password. */
export function setup(username: string, password: string): Promise<LoginResult> {
  return http.post<LoginResult>('/auth/setup', { username, password })
}
