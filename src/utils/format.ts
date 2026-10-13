/** Date and number formatting helpers. Locale is zh-CN throughout. */

const dateFull = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

const dateLong = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
})

const dateShort = new Intl.DateTimeFormat('zh-CN', { month: 'short', day: 'numeric' })

/** "2026-04-18" */
export function formatDate(value?: string | Date | null): string {
  const date = toDate(value)
  return date ? dateFull.format(date) : '—'
}

/** "2026年4月18日" */
export function formatDateLong(value?: string | Date | null): string {
  const date = toDate(value)
  return date ? dateLong.format(date) : '—'
}

/** "4月18日" */
export function formatDateShort(value?: string | Date | null): string {
  const date = toDate(value)
  return date ? dateShort.format(date) : '—'
}

/** "刚刚 / 3 分钟前 / 2 天前 / 2026-04-18" */
export function formatRelative(value?: string | Date | null): string {
  const date = toDate(value)
  if (!date) return '—'

  const diff = Date.now() - date.getTime()
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour

  if (diff < minute) return '刚刚'
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`
  if (diff < 30 * day) return `${Math.floor(diff / day)} 天前`
  return formatDate(date)
}

export function formatDateTime(value?: string | Date | null): string {
  const date = toDate(value)
  if (!date) return '—'
  return `${formatDate(date)} ${String(date.getHours()).padStart(2, '0')}:${String(
    date.getMinutes(),
  ).padStart(2, '0')}`
}

/** 1234 -> "1,234"; 12000 -> "1.2 万" */
export function formatCount(value: number | undefined | null): string {
  if (value === undefined || value === null) return '0'
  if (value < 10_000) return value.toLocaleString('zh-CN')
  return `${(value / 10_000).toFixed(1)} 万`
}

export function formatBytes(bytes: number | undefined | null): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${value.toFixed(value >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`
}

function toDate(value?: string | Date | null): Date | null {
  if (!value) return null
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}
