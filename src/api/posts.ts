import { http } from '@/api/http'
import type {
  ArchiveYear,
  Paginated,
  Post,
  PostDetail,
  PostInput,
  PostQuery,
  PostSummary,
  Tag,
} from '@/types/blog'

function queryOf(q: PostQuery = {}): Record<string, string | number | undefined> {
  return {
    page: q.page,
    pageSize: q.pageSize,
    status: q.status === 'all' ? undefined : q.status,
    tag: q.tag,
    keyword: q.keyword,
    sort: q.sort,
    order: q.order,
  }
}

/* -------------------------------------------------------------------------- */
/* Public                                                                     */
/* -------------------------------------------------------------------------- */

/** Published posts only. */
export function listPublished(q: PostQuery = {}): Promise<Paginated<PostSummary>> {
  return http.get<Paginated<PostSummary>>('/posts', queryOf(q))
}

export function getPublished(slug: string): Promise<PostDetail> {
  return http.get<PostDetail>(`/posts/${encodeURIComponent(slug)}`)
}

/** Published posts grouped by year → month. */
export function getArchive(): Promise<{ years: ArchiveYear[] }> {
  return http.get<{ years: ArchiveYear[] }>('/archive')
}

export function listTags(): Promise<{ tags: Tag[] }> {
  return http.get<{ tags: Tag[] }>('/tags')
}

/* -------------------------------------------------------------------------- */
/* Admin                                                                      */
/* -------------------------------------------------------------------------- */

/** Drafts and published posts alike. */
export function listAdminPosts(q: PostQuery = {}): Promise<Paginated<PostSummary>> {
  return http.get<Paginated<PostSummary>>('/admin/posts', queryOf(q))
}

export function getAdminPost(id: number): Promise<Post> {
  return http.get<Post>(`/admin/posts/${id}`)
}

export function createPost(input: PostInput): Promise<Post> {
  return http.post<Post>('/admin/posts', input)
}

export function updatePost(id: number, input: PostInput): Promise<Post> {
  return http.put<Post>(`/admin/posts/${id}`, input)
}

export function deletePost(id: number): Promise<void> {
  return http.delete<void>(`/admin/posts/${id}`)
}

/** draft -> published, stamping publishedAt on first publish. */
export function publishPost(id: number): Promise<Post> {
  return http.post<Post>(`/admin/posts/${id}/publish`)
}

/** published -> draft, keeping publishedAt for a later re-publish. */
export function unpublishPost(id: number): Promise<Post> {
  return http.post<Post>(`/admin/posts/${id}/unpublish`)
}

/** Slug preview + uniqueness check for the editor's slug field. */
export function checkSlug(
  slug: string,
  excludeId?: number,
): Promise<{ slug: string; available: boolean }> {
  return http.get<{ slug: string; available: boolean }>('/admin/slug', {
    slug,
    excludeId,
  })
}
