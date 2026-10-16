/**
 * Shared duration formatting for the music player.
 *
 * Track durations come from the Go scanner as milliseconds and may legitimately
 * be zero when a format carried no parsable header — callers pass a fallback so
 * the UI shows a dash instead of a misleading 0:00.
 */
export function formatDuration(seconds: number, fallback = '--:--'): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return fallback

  const total = Math.floor(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = total % 60
  const pad = (value: number): string => String(value).padStart(2, '0')

  return hours > 0 ? `${hours}:${pad(minutes)}:${pad(rest)}` : `${minutes}:${pad(rest)}`
}

/** Same formatting, but from a millisecond value as the API sends it. */
export function formatDurationMs(milliseconds: number, fallback = '--:--'): string {
  return formatDuration(milliseconds / 1000, fallback)
}
