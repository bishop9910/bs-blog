/**
 * Music library types.
 *
 * These mirror the JSON handed out by the Go backend (see
 * backend/internal/service/music.go). Track ids are opaque hashes of the file
 * path rather than database ids, because the library is a disk scan: the file
 * system is the source of truth and nothing is stored in SQLite.
 */

export interface MusicTrack {
  id: string
  title: string
  artist: string
  album: string
  albumArtist: string
  year: number
  trackNo: number
  /** Milliseconds. Zero when the format carried no parsable duration. */
  durationMs: number
  hasCover: boolean
  hasLyrics: boolean
  /** Server path, e.g. /api/v1/music/stream/<id>. Pass through mediaUrl(). */
  streamUrl: string
  /** Server path, e.g. /api/v1/music/cover/<id>. Pass through mediaUrl(). */
  coverUrl: string
}

export interface MusicAlbum {
  name: string
  artist: string
  year: number
  trackCount: number
  coverUrl: string
}

export interface MusicArtist {
  name: string
  trackCount: number
}

export interface MusicLibrary {
  tracks: MusicTrack[]
  albums: MusicAlbum[]
  artists: MusicArtist[]
  trackCount: number
  /** RFC3339 timestamp of the last completed scan. */
  scannedAt: string
}

/** The admin view adds where the library was read from. */
export interface MusicAdminLibrary extends MusicLibrary {
  dir: string
  exists: boolean
  extensions: string[]
  watch: boolean
  albumCount: number
  artistCount: number
}

export interface MusicLyrics {
  /** Raw LRC text. Empty when the track has no sidecar. */
  lyrics: string
  hasLyrics: boolean
}
