// Package sanitize cleans the rich-text HTML produced by the editor before it
// is stored or served.
//
// Sanitising on the way in is not redundant with the frontend's DOMPurify pass:
// the stored HTML is the canonical representation and is rendered with v-html,
// and a client-side-only defence can be bypassed by anything that writes to the
// API directly. Cleaning here means every reader, and every future export, sees
// only content that has already passed the allow-list.
package sanitize

import (
	"regexp"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var (
	classPattern = regexp.MustCompile(`^[A-Za-z0-9_\-\s]{1,200}$`)
	alignPattern = regexp.MustCompile(`^(left|right|center|justify)$`)
	colorPattern = regexp.MustCompile(`^(#[0-9A-Fa-f]{3,8}|rgba?\([0-9.,\s%]{3,40}\)|[A-Za-z]{3,20})$`)
	spanPattern  = regexp.MustCompile(`^(true|false)$`)
	intPattern   = regexp.MustCompile(`^[0-9]{1,3}$`)
	langPattern  = regexp.MustCompile(`^[A-Za-z0-9+#\-\s]{1,40}$`)
	dataType     = regexp.MustCompile(`^[A-Za-z\-]{1,40}$`)
)

var (
	once   sync.Once
	policy *bluemonday.Policy
)

// Policy returns the shared allow-list, built once.
//
// The list is driven by what the editor can emit: StarterKit's marks and
// blocks, task lists, tables, highlights, text alignment, text colour, and
// lowlight code blocks. Anything else — script, style, iframe, form, embed,
// event handlers, data: URLs — is dropped.
func Policy() *bluemonday.Policy {
	once.Do(func() {
		p := bluemonday.NewPolicy()

		p.AllowStandardURLs() // http, https, mailto
		p.AllowRelativeURLs(true)
		p.AllowStandardAttributes()

		p.AllowElements(
			"p", "br", "hr", "span", "div", "label",
			"h1", "h2", "h3", "h4", "h5", "h6",
			"strong", "b", "em", "i", "u", "s", "del", "ins", "mark", "sub", "sup",
			"code", "pre", "blockquote",
			"ul", "ol", "li",
			"figure", "figcaption",
			"table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col",
		)

		// Links.
		p.AllowAttrs("href", "target", "rel").OnElements("a")
		p.AddTargetBlankToFullyQualifiedLinks(true)
		p.RequireNoReferrerOnLinks(true)

		// Images. Uploaded files are served from a relative /uploads URL.
		p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")

		// Task list checkboxes, rendered inert by the frontend CSS.
		p.AllowAttrs("type", "checked", "disabled").OnElements("input")

		// Table cell spans.
		p.AllowAttrs("colspan", "rowspan").Matching(intPattern).OnElements("td", "th")

		// Editor bookkeeping attributes.
		p.AllowAttrs("class").Matching(classPattern).Globally()
		p.AllowAttrs("data-type").Matching(dataType).Globally()
		p.AllowAttrs("data-checked").Matching(spanPattern).OnElements("li")
		p.AllowAttrs("data-language", "data-theme").Matching(langPattern).OnElements("pre", "code")
		p.AllowAttrs("data-color").Matching(colorPattern).OnElements("mark", "span")

		// The small slice of inline CSS the editor can produce. Everything else
		// — position, z-index, url(), expression() — is refused.
		p.AllowStyles("text-align").Matching(alignPattern).Globally()
		p.AllowStyles("color", "background-color").Matching(colorPattern).Globally()

		policy = p
	})
	return policy
}

// HTML runs the allow-list over an editor document.
func HTML(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	return strings.TrimSpace(Policy().Sanitize(input))
}
