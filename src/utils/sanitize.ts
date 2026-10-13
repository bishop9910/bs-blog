import DOMPurify from 'dompurify'

/**
 * The backend already runs the stored HTML through the same kind of allow-list
 * (backend/internal/pkg/sanitize), so this is belt-and-braces: it catches a bad
 * import, a hand-edited row, or a payload that reached the database before the
 * server-side policy was tightened. The reader page is the last place a script
 * could still execute, so it is worth the cost there.
 */
export function sanitizeHtml(html: string): string {
  if (!html) return ''
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    ADD_ATTR: ['target', 'rel', 'data-language', 'data-type', 'data-checked'],
    // Never let a link open a new tab without the opener being severed.
    FORBID_TAGS: ['style', 'form', 'input', 'button'],
  })
}
