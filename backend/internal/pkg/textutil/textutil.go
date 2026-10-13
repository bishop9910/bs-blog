// Package textutil mirrors the frontend's src/utils/text.ts.
//
// The blog is written in Chinese as often as in English, so slug and word-count
// rules have to treat CJK characters as first-class rather than as separators:
// a naive [a-z0-9] slug filter turns a Chinese title into an empty string, and
// a whitespace word count reports every Chinese post as under a minute long.
package textutil

import (
	"html"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	tagPattern      = regexp.MustCompile(`(?s)<[^>]*>`)
	scriptPattern   = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</\s*(script|style)\s*>`)
	whitespaceRun   = regexp.MustCompile(`\s+`)
	dashRun         = regexp.MustCompile(`-{2,}`)
	latinWord       = regexp.MustCompile(`[A-Za-z0-9]+`)
	maxSlugLen      = 80
	maxSlugSrcRunes = 200
)

// Slugify converts a title into a URL-safe slug, preserving CJK characters.
//
// "Hello, 世界 World!" becomes "hello-世界-world". Latin text is decomposed
// (NFKD) and stripped of accents so "Café" becomes "cafe".
func Slugify(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	// Bound the input before doing any Unicode work.
	runes := []rune(trimmed)
	if len(runes) > maxSlugSrcRunes {
		trimmed = string(runes[:maxSlugSrcRunes])
	}

	decomposed := norm.NFKD.String(strings.ToLower(trimmed))

	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		switch {
		// Drop combining accents left behind by the NFKD decomposition.
		case unicode.Is(unicode.Mn, r):
			continue
		// Letters and digits survive — this is what keeps 中文 slugs alive.
		case unicode.IsLetter(r), unicode.IsNumber(r):
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	// Recompose. NFKD split Hangul syllables into jamo and accented letters into
	// base + combining mark, so without this a Korean title slugs to a run of
	// jamo instead of readable syllables. (The combining marks themselves were
	// dropped above, so NFC can only put the jamo back together.)
	slug := norm.NFC.String(b.String())
	slug = dashRun.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len([]rune(slug)) > maxSlugLen {
		slug = strings.Trim(string([]rune(slug)[:maxSlugLen]), "-")
	}
	return slug
}

// StripHTML returns the visible text of an HTML fragment: scripts and styles
// removed entirely, other tags replaced by a space so words do not run together,
// and entities decoded so "&amp;" counts as one character rather than five.
func StripHTML(input string) string {
	if input == "" {
		return ""
	}
	text := scriptPattern.ReplaceAllString(input, " ")
	text = tagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return strings.TrimSpace(whitespaceRun.ReplaceAllString(text, " "))
}

// CountWords counts CJK characters individually and everything else as
// whitespace-delimited words, matching the frontend's countWords.
func CountWords(html string) int {
	text := StripHTML(html)
	if text == "" {
		return 0
	}

	cjk := 0
	for _, r := range text {
		if IsCJK(r) {
			cjk++
		}
	}

	// Latin runs adjacent to CJK would otherwise be counted twice, so measure
	// the non-CJK remainder by stripping the CJK characters out first.
	withoutCJK := strings.Map(func(r rune) rune {
		if IsCJK(r) {
			return ' '
		}
		return r
	}, text)

	return cjk + len(latinWord.FindAllString(withoutCJK, -1))
}

// ReadingMinutes estimates reading time. Chinese is read roughly 350 characters
// per minute, Latin roughly 200 words per minute; the two contributions are
// summed so mixed posts land in a sensible place. Never returns less than 1.
func ReadingMinutes(html string) int {
	text := StripHTML(html)
	if text == "" {
		return 1
	}

	cjk := 0
	for _, r := range text {
		if IsCJK(r) {
			cjk++
		}
	}

	withoutCJK := strings.Map(func(r rune) rune {
		if IsCJK(r) {
			return ' '
		}
		return r
	}, text)
	words := len(latinWord.FindAllString(withoutCJK, -1))

	minutes := float64(cjk)/350 + float64(words)/200
	if minutes < 1 {
		return 1
	}
	// Round up: "2.1 minutes" reading as 3 is friendlier than 2.
	return int(minutes + 0.999)
}

// IsCJK reports whether r is a Chinese/Japanese/Korean character that should be
// counted per-glyph rather than per whitespace-delimited word.
func IsCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
		return true
	case r >= 0x3400 && r <= 0x4DBF: // Extension A
		return true
	case r >= 0xF900 && r <= 0xFAFF: // Compatibility Ideographs
		return true
	case r >= 0x3040 && r <= 0x30FF: // Hiragana + Katakana
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // Hangul syllables
		return true
	default:
		return false
	}
}

// Truncate shortens s to limit runes, appending an ellipsis when it cut
// anything. Used for auto-generated summaries.
func Truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

// AutoExcerpt builds a plain-text summary from post HTML when the author did
// not write one.
func AutoExcerpt(html string, limit int) string {
	return Truncate(StripHTML(html), limit)
}
