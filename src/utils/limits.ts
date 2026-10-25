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

/**
 * "zip / 7z / rar 等 46 种格式" — the head of the server's extension list.
 *
 * The full list is far too long for a hint line, and quoting only a few
 * formats helps most: the file picker's accept filter already carries all of
 * them, and the server is the one that actually enforces the list.
 */
export function describeExtensions(allowed: readonly string[], preview = 3): string {
  if (allowed.length === 0) return ''
  const head = allowed.slice(0, preview).join(' / ')
  return allowed.length > preview ? `${head} 等 ${allowed.length} 种格式` : head
}
