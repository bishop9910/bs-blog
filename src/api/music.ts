import { API_BASE, getToken, http } from '@/api/http'
import type { MusicAdminLibrary, MusicLibrary, MusicLyrics, MusicTrack } from '@/types/music'

/**
 * The whole library in one request.
 *
 * A personal library is small enough that paging it would cost more than it
 * saves: shuffle, "next" and the playlist all need the full track list anyway,
 * so splitting it across pages would only make the player stateful about
 * fetching.
 */
export function getMusicLibrary(signal?: AbortSignal): Promise<MusicLibrary> {
  return http.get<MusicLibrary>('/music/library', undefined, signal)
}

export function getTrackLyrics(id: string, signal?: AbortSignal): Promise<MusicLyrics> {
  return http.get<MusicLyrics>(`/music/tracks/${encodeURIComponent(id)}/lyrics`, undefined, signal)
}

export function getAdminMusic(): Promise<MusicAdminLibrary> {
  return http.get<MusicAdminLibrary>('/admin/music')
}

export function rescanMusic(): Promise<MusicAdminLibrary> {
  return http.post<MusicAdminLibrary>('/admin/music/rescan')
}

export function deleteMusic(id: string): Promise<void> {
  return http.delete<void>(`/admin/music/${encodeURIComponent(id)}`)
}

/**
 * Uploads one audio file into the library directory.
 *
 * An optional lyrics file rides along in the same request: the server writes it
 * as the track's .lrc sidecar, which is the same name a hand-tagged library
 * uses. Sending both at once keeps the upload atomic from the author's point of
 * view — there is no window where a track exists without its lyrics.
 *
 * XHR rather than the shared `http.upload` helper, for the same reason the
 * media upload does it: fetch cannot report upload progress, and a music file
 * is large enough that a progress bar is not optional.
 */
export function uploadMusic(
  file: File,
  lyrics?: File | null,
  onProgress?: (percent: number) => void,
): Promise<MusicTrack> {
  const form = new FormData()
  form.append('file', file, file.name)
  if (lyrics) form.append('lyrics', lyrics, lyrics.name)

  return new Promise<MusicTrack>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}/admin/music/upload`)
    xhr.responseType = 'json'

    const token = getToken()
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)

    xhr.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable && onProgress) {
        onProgress(Math.round((event.loaded / event.total) * 100))
      }
    })

    xhr.addEventListener('load', () => {
      const body = xhr.response ?? {}
      if (xhr.status >= 200 && xhr.status < 300) resolve(body as MusicTrack)
      else reject(new Error(body.message || `上传失败（${xhr.status}）`))
    })

    xhr.addEventListener('error', () => reject(new Error('上传失败：网络错误')))
    xhr.addEventListener('abort', () => reject(new Error('上传已取消')))
    xhr.send(form)
  })
}

/**
 * Replaces a track's lyrics after the fact, or attaches them to a song that was
 * uploaded without any.
 *
 * The batch upload takes lyrics for one song only, so this is the route that
 * makes a whole album dropped in at once come out with its `.lrc` files: pick
 * each track in the table and attach its file.
 */
export function setTrackLyrics(id: string, file: File): Promise<MusicTrack> {
  const form = new FormData()
  form.append('lyrics', file, file.name)
  return http.upload<MusicTrack>(`/admin/music/${encodeURIComponent(id)}/lyrics`, form, 'PUT')
}

/** Removes a track's `.lrc`; lyrics inside a JSON sidecar or the tags stay. */
export function clearTrackLyrics(id: string): Promise<MusicTrack> {
  return http.delete<MusicTrack>(`/admin/music/${encodeURIComponent(id)}/lyrics`)
}

/**
 * Rewrites a server-supplied media path onto the configured API base.
 *
 * The backend hands out paths that include the /api/v1 prefix, while the
 * frontend may be pointed at a different origin entirely through
 * VITE_API_BASE_URL. Swapping the documented prefix keeps both in step without
 * the player having to know the route shapes.
 */
export function mediaUrl(path: string): string {
  const PREFIX = '/api/v1'
  if (!path) return ''
  if (path.startsWith(PREFIX)) return `${API_BASE}${path.slice(PREFIX.length)}`
  return path
}
