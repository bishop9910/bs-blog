package textutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildPart mirrors the "inputBuild"/"slugBuild" entries in the shared
// contract file: a repeated unit, so long fixtures stay readable.
type buildPart struct {
	Unit  string `json:"unit"`
	Count int    `json:"count"`
}

type contractCase struct {
	Name       string      `json:"name"`
	Input      string      `json:"input"`
	InputBuild []buildPart `json:"inputBuild"`
	Slug       *string     `json:"slug"`
	SlugBuild  []buildPart `json:"slugBuild"`
	Words      *int        `json:"words"`
	Minutes    *int        `json:"minutes"`
	Excerpt    *string     `json:"excerpt"`
}

type contractFile struct {
	ExcerptLimit int            `json:"excerptLimit"`
	Cases        []contractCase `json:"cases"`
}

func buildOf(parts []buildPart) string {
	var b strings.Builder
	for _, p := range parts {
		for i := 0; i < p.Count; i++ {
			b.WriteString(p.Unit)
		}
	}
	return b.String()
}

// TestFrontendContract asserts the shared expectations in
// scripts/text-cases.json. scripts/check-text-contract.mjs asserts the same
// file against src/utils/text.ts, so when both are green the two
// implementations cannot have drifted apart.
//
// This matters because the server recomputes the slug, the word count and the
// reading time when a post is saved: a drift here means the editor previews one
// value and the database stores another.
func TestFrontendContract(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "scripts", "text-cases.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var spec contractFile
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(spec.Cases) == 0 {
		t.Fatalf("%s contains no cases", path)
	}
	if spec.ExcerptLimit <= 0 {
		spec.ExcerptLimit = 160
	}

	for _, c := range spec.Cases {
		t.Run(c.Name, func(t *testing.T) {
			input := c.Input
			if len(c.InputBuild) > 0 {
				input = buildOf(c.InputBuild)
			}

			wantSlug := c.Slug
			if wantSlug == nil && len(c.SlugBuild) > 0 {
				built := buildOf(c.SlugBuild)
				wantSlug = &built
			}

			if wantSlug != nil {
				if got := Slugify(input); got != *wantSlug {
					t.Errorf("Slugify(%q) = %q, want %q", input, got, *wantSlug)
				}
			}
			if c.Words != nil {
				if got := CountWords(input); got != *c.Words {
					t.Errorf("CountWords(%q) = %d, want %d", input, got, *c.Words)
				}
			}
			if c.Minutes != nil {
				if got := ReadingMinutes(input); got != *c.Minutes {
					t.Errorf("ReadingMinutes(%q) = %d, want %d", input, got, *c.Minutes)
				}
			}
			if c.Excerpt != nil {
				if got := AutoExcerpt(input, spec.ExcerptLimit); got != *c.Excerpt {
					t.Errorf("AutoExcerpt(%q) = %q, want %q", input, got, *c.Excerpt)
				}
			}
		})
	}
}

func TestStripHTMLDecodesEntitiesAndDropsScripts(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"entities", "<p>a &amp; b</p>", "a & b"},
		{"script-dropped", "<p>Hi</p><script>var a = 1</script>", "Hi"},
		{"style-dropped", "<style>p { color: red }</style><p>Hi</p>", "Hi"},
		{"tags-become-spaces", "<p>one</p><p>two</p>", "one two"},
		{"empty", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StripHTML(c.in); got != c.want {
				t.Errorf("StripHTML(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSlugifyBoundsTheSourceBeforeUnicodeWork(t *testing.T) {
	if got, want := Slugify(strings.Repeat("a", 300)), strings.Repeat("a", 80); got != want {
		t.Errorf("Slugify(300 x a) = %q, want %q", got, want)
	}
}

func TestTruncateAppendsEllipsisOnlyWhenItCut(t *testing.T) {
	cases := []struct {
		in    string
		limit int
		want  string
	}{
		{"hello world", 5, "hello…"},
		{"hi", 5, "hi"},
		{"abcde", 5, "abcde"},
		{"anything", 0, ""},
		{"中文标题很长很长", 3, "中文标…"},
	}

	for _, c := range cases {
		if got := Truncate(c.in, c.limit); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.in, c.limit, got, c.want)
		}
	}
}

func TestIsCJKCoversEveryDeclaredRange(t *testing.T) {
	inside := []rune{0x3400, 0x4DBF, 0x4E00, 0x9FFF, 0xF900, 0xFAFF, 0x3040, 0x30FF, 0xAC00, 0xD7AF}
	outside := []rune{0x4DFF, 0xA000, 0x3100, 0x1100, 'a', '1', '-', ' '}

	for _, r := range inside {
		if !IsCJK(r) {
			t.Errorf("IsCJK(%#U) = false, want true", r)
		}
	}
	for _, r := range outside {
		if IsCJK(r) {
			t.Errorf("IsCJK(%#U) = true, want false", r)
		}
	}
}
