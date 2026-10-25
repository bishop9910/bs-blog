import { API_BASE, getToken, http } from '@/api/http'
import type { Paginated } from '@/types/blog'
import type { ResourceFile, ResourceInput } from '@/types/resource'

export interface ResourceQuery {
  page?: number
  pageSize?: number
  keyword?: string
}

/** Visitor-facing list: only the files the owner marked visible. */
export function listPublicResources(query: ResourceQuery = {}): Promise<Paginated<ResourceFile>> {
  return http.get<Paginated<ResourceFile>>('/resources', { ...query })
}

/** Admin list: every file, hidden ones included. */
export function listResources(query: ResourceQuery = {}): Promise<Paginated<ResourceFile>> {
  return http.get<Paginated<ResourceFile>>('/admin/resources', { ...query })
}

/**
 * The download address.
 *
 * Built here rather than returned by the API because it is a route, not stored
 * data: `${API_BASE}` already carries the deployment's prefix, and the endpoint
 * answers with Content-Disposition: attachment, so simply navigating to it saves
 * the file without leaving the page.
 */
export function resourceDownloadUrl(id: number): string {
  return `${API_BASE}/resources/${id}/download`
}

/**
 * Uploads one file together with the metadata typed into the admin form.
 *
 * XHR rather than fetch: upload progress is not observable with fetch, and a
 * 30 MB project archive is exactly the kind of transfer where a bare spinner
 * looks broken.
 */
export function uploadResource(
  file: File,
  meta: ResourceInput,
  onProgress?: (percent: number) => void,
): Promise<ResourceFile> {
  const form = new FormData()
  form.append('file', file, file.name)
  form.append('title', meta.title)
  form.append('description', meta.description)
  form.append('coverUrl', meta.coverUrl)
  if (meta.visible !== undefined) form.append('visible', meta.visible ? 'true' : 'false')

  return new Promise<ResourceFile>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}/admin/resources`)
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
      if (xhr.status >= 200 && xhr.status < 300) resolve(body as ResourceFile)
      else reject(new Error(body.message || `上传失败（${xhr.status}）`))
    })

    xhr.addEventListener('error', () => reject(new Error('上传失败：网络错误')))
    xhr.addEventListener('abort', () => reject(new Error('上传已取消')))
    xhr.send(form)
  })
}

export function updateResource(id: number, input: ResourceInput): Promise<ResourceFile> {
  return http.put<ResourceFile>(`/admin/resources/${id}`, input)
}

export function deleteResource(id: number): Promise<void> {
  return http.delete<void>(`/admin/resources/${id}`)
}
