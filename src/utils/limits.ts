import { formatBytes } from './format'

/** "image/jpeg" -> "JPEG". Anything unrecognised keeps its subtype, uppercased. */
export function imageTypeLabel(mimeType: string): string {
  const known: Record<string, string> = {
    'image/jpeg': 'JPEG',
    'image/png': 'PNG',
    'image/gif': 'GIF',
    'image/webp': 'WebP',
  }
  const hit = known[mimeType]
  if (hit) return hit
  const subtype = mimeType.split('/')[1]
  return subtype ? subtype.toUpperCase() : mimeType
}

/** "支持 PNG / JPEG / WebP / GIF" — built from the list the server accepts. */
export function describeImageTypes(allowedTypes: readonly string[]): string {
  if (allowedTypes.length === 0) return ''
  return allowedTypes.map(imageTypeLabel).join(' / ')
}

/** "单文件不超过 10 MB" */
export function describeSizeLimit(bytes: number): string {
  return `单文件不超过 ${formatBytes(bytes)}`
}
