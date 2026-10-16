/**
 * Shared domain types for the bs-blog frontend.
 * These mirror the backend proto contract (backend/api/blog/v1/blog.proto).
 * JSON is camelCase in both directions.
 */

export type PostStatus = 'draft' | 'published'

export type PostVisibility = 'public' | 'private'

export interface Tag {
  id: number
  name: string
  slug: string
  /** Optional palette hint; falls back to the theme accent. */
  color?: string
  postCount: number
}

export interface PostSummary {
  id: number
  slug: string
  title: string
  summary: string
  coverUrl: string
  status: PostStatus
  visibility: PostVisibility
  tags: Tag[]
  /** RFC3339. Absent while the post is a draft. */
  publishedAt?: string
  createdAt: string
  updatedAt: string
  readingMinutes: number
  wordCount: number
  viewCount: number
}

export interface Post extends PostSummary {
  /** Sanitised HTML — the canonical stored representation. */
  contentHtml: string
  /** ProseMirror document tree, kept for lossless re-editing. */
  contentJson: string
  /** Optional per-post overrides. */
  metaTitle?: string
  metaDescription?: string
}

export interface PostLink {
  slug: string
  title: string
}

export interface PostNav {
  prev?: PostLink
  next?: PostLink
}

export interface PostDetail {
  post: Post
  nav: PostNav
}

export interface Paginated<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export interface ArchiveMonth {
  month: number
  posts: PostSummary[]
}

export interface ArchiveYear {
  year: number
  count: number
  months: ArchiveMonth[]
}

export interface SocialLink {
  /** Stable key used to pick the icon: github | x | email | rss | link */
  icon: string
  label: string
  url: string
}

export interface SiteMeta {
  title: string
  tagline: string
  description: string
  author: string
  avatarUrl: string
  /** Wallpaper pool. A visit paints one of these at random, so order is a hint
   * about preference, not a sequence. */
  wallpapers: string[]
  socials: SocialLink[]
  footerNote: string
  stats: SiteStats
}

export interface SiteStats {
  postCount: number
  tagCount: number
  wordCount: number
  firstPublishedAt?: string
  lastPublishedAt?: string
}

export interface User {
  id: number
  username: string
  nickname: string
  avatarUrl: string
  lastLoginAt?: string
}

export interface LoginResult {
  token: string
  /** RFC3339 expiry of the access token. */
  expiresAt: string
  user: User
}

export interface MediaAsset {
  id: number
  url: string
  filename: string
  mimeType: string
  size: number
  width: number
  height: number
  createdAt: string
}

export interface PostQuery {
  page?: number
  pageSize?: number
  status?: PostStatus | 'all'
  tag?: string
  keyword?: string
  sort?: 'created' | 'published' | 'updated' | 'title'
  order?: 'asc' | 'desc'
}

/**
 * Draft payload sent to create/update. `status` is intentionally absent —
 * publishing is a separate, explicit transition.
 */
export interface PostInput {
  title: string
  slug?: string
  summary?: string
  coverUrl?: string
  contentHtml: string
  contentJson: string
  tagNames: string[]
  visibility?: PostVisibility
  metaTitle?: string
  metaDescription?: string
}
