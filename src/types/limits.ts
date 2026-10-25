/**
 * Effective upload and field policy, as reported by GET /admin/limits.
 *
 * These numbers are what the server actually enforces, so the forms pre-check
 * against them instead of hard-coding a second copy: change a value in
 * config.yaml and the UI follows on the next load.
 */

export interface MediaLimits {
  /** Per-image ceiling in bytes (data.maxUploadBytes). */
  maxUploadBytes: number
  /** MIME types the backend accepts. Not configurable: it is the sanitisation boundary. */
  allowedTypes: string[]
}

export interface MusicLimits {
  /** Per-audio-file ceiling in bytes (music.maxUploadBytes). */
  maxUploadBytes: number
  /** Largest sidecar cover read from disk (music.maxCoverBytes). */
  maxCoverBytes: number
  /** Largest .lrc file read (music.maxLyricsBytes). */
  maxLyricsBytes: number
}

export interface SiteLimits {
  maxSocials: number
  maxWallpapers: number
  maxWallpaperLength: number
  maxTitleLength: number
  maxTaglineLength: number
  maxDescriptionLength: number
  maxAuthorLength: number
  maxFooterLength: number
  maxSocialLabelLength: number
}

export interface ResourceLimits {
  /** Per-file ceiling in bytes (resources.maxUploadBytes). */
  maxUploadBytes: number
  /** Lowercase extensions without the dot, sorted: also used as the file picker's accept list. */
  allowedExtensions: string[]
}

export interface Limits {
  media: MediaLimits
  music: MusicLimits
  resources: ResourceLimits
  site: SiteLimits
}
