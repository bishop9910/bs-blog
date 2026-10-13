import { API_BASE, getToken, http } from '@/api/http'
import type { MediaAsset, Paginated } from '@/types/blog'

export function listMedia(
  page = 1,
  pageSize = 40,
  keyword?: string,
): Promise<Paginated<MediaAsset>> {
  return http.get<Paginated<MediaAsset>>('/admin/media', { page, pageSize, keyword })
}

/**
 * Uploads one image. The editor drops the returned `url` straight into the
 * document, so the backend is responsible for making it publicly reachable.
 */
export function uploadMedia(file: File, onProgress?: (percent: number) => void): Promise<MediaAsset> {
  const form = new FormData()
  form.append('file', file, file.name)

  // XHR rather than fetch: upload progress is not observable with fetch.
  return new Promise<MediaAsset>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}/admin/media`)
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
      if (xhr.status >= 200 && xhr.status < 300) resolve(body as MediaAsset)
      else reject(new Error(body.message || `上传失败（${xhr.status}）`))
    })

    xhr.addEventListener('error', () => reject(new Error('上传失败：网络错误')))
    xhr.addEventListener('abort', () => reject(new Error('上传已取消')))
    xhr.send(form)
  })
}

export function deleteMedia(id: number): Promise<void> {
  return http.delete<void>(`/admin/media/${id}`)
}
