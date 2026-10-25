/**
 * A downloadable file published on the resources page.
 *
 * Wire shape of GET /resources — camelCase, mirroring biz.ResourceDTO in
 * backend/internal/biz/dto.go. There is deliberately no download URL field: the
 * address is derived in api/resources.ts from the id, so a file can be moved on
 * disk (or the API mounted under another prefix) without touching stored rows.
 */

export interface ResourceFile {
  id: number
  /** Display name; defaults to the filename without its extension. */
  title: string
  description: string
  /** Optional thumbnail URL, usually a media-library image. */
  coverUrl: string
  /** Original name of the uploaded file, extension included. */
  filename: string
  /** Lowercase extension without the dot, e.g. "zip". */
  extension: string
  size: number
  downloadCount: number
  /** False hides the file from the public listing; the admin table still shows it. */
  visible: boolean
  createdAt: string
  updatedAt: string
}

/** Editable metadata. Omitted `visible` lets the server keep (or pick) its value. */
export interface ResourceInput {
  title: string
  description: string
  coverUrl: string
  visible?: boolean
}
