// Package model holds the plain data types shared by the importers and the server.
package model

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Line is one ingredient or one step. H marks a section heading
// ("For the glaze") rather than an ingredient/step.
type Line struct {
	Text string `json:"text"`
	H    bool   `json:"h,omitempty"`
}

// Recipe is the canonical recipe document. Ingredients are deliberately kept
// as plain text: quantities, units and names are parsed on the fly by the
// client, so there is no food/unit database to maintain.
type Recipe struct {
	ID          int64             `json:"id"`
	Slug        string            `json:"slug"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	ImageColor  string            `json:"imageColor"`
	SourceURL   string            `json:"sourceUrl"`
	SourceName  string            `json:"sourceName"`
	Author      string            `json:"author"`
	Yield       string            `json:"yield"`
	Servings    float64           `json:"servings"`
	PrepMin     int               `json:"prepMin"`
	CookMin     int               `json:"cookMin"`
	TotalMin    int               `json:"totalMin"`
	Ingredients []Line            `json:"ingredients"`
	Steps       []Line            `json:"steps"`
	Notes       string            `json:"notes"`
	Tags        []string          `json:"tags"`
	Nutrition   map[string]string `json:"nutrition,omitempty"`
	Extra       json.RawMessage   `json:"extra,omitempty"`
	Rating      float64           `json:"rating"`
	Favorite    bool              `json:"favorite"`
	CookedCount int               `json:"cookedCount"`
	LastCooked  int64             `json:"lastCooked"`
	ShareToken  string            `json:"shareToken,omitempty"`
	Cookbooks   []int64           `json:"cookbooks"`
	CreatedAt   int64             `json:"createdAt"`
	UpdatedAt   int64             `json:"updatedAt"`
	// Version counts the saves that really changed the ingredients or the
	// steps; every one of them is kept in the recipe's history.
	Version int `json:"version"`
	// VersionNote is sent with a save to say what it did ("Weights added").
	VersionNote string `json:"versionNote,omitempty"`
}

var (
	slugStrip = regexp.MustCompile(`[^a-z0-9]+`)
	spaceRun  = regexp.MustCompile(`[ \t\f\v\x{00a0}\x{2009}\x{202f}]+`)
)

// Slugify turns a title into a URL-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(foldASCII(s))
	s = strings.Trim(slugStrip.ReplaceAllString(s, "-"), "-")
	if len(s) > 64 {
		s = strings.Trim(s[:64], "-")
	}
	if s == "" {
		s = "recipe"
	}
	return s
}

var folds = map[rune]string{
	'ä': "a", 'á': "a", 'à': "a", 'â': "a", 'ã': "a", 'å': "a", 'æ': "ae", 'ç': "c",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e", 'í': "i", 'ì': "i", 'î': "i", 'ï': "i",
	'ñ': "n", 'ö': "o", 'ó': "o", 'ò': "o", 'ô': "o", 'õ': "o", 'ø': "o", 'œ': "oe",
	'ß': "ss", 'ü': "u", 'ú': "u", 'ù': "u", 'û': "u", 'ý': "y", 'ÿ': "y",
}

func foldASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		lr := unicode.ToLower(r)
		if f, ok := folds[lr]; ok {
			b.WriteString(f)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CleanLine collapses whitespace on a single line of text.
func CleanLine(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(spaceRun.ReplaceAllString(s, " "))
}

// Normalize trims and bounds every field so that whatever an importer or a
// client sends is safe to store.
func (r *Recipe) Normalize() {
	r.Title = clip(CleanLine(r.Title), 300)
	r.Description = clip(strings.TrimSpace(r.Description), 4000)
	r.SourceURL = clip(strings.TrimSpace(r.SourceURL), 2000)
	r.SourceName = clip(CleanLine(r.SourceName), 200)
	r.Author = clip(CleanLine(r.Author), 200)
	r.Yield = clip(CleanLine(r.Yield), 120)
	r.Notes = clip(strings.TrimSpace(r.Notes), 20000)
	r.Ingredients = cleanLines(r.Ingredients, 1000, 600)
	r.Steps = cleanLines(r.Steps, 600, 8000)
	if r.Servings < 0 || r.Servings > 100000 {
		r.Servings = 0
	}
	r.PrepMin, r.CookMin, r.TotalMin = bound(r.PrepMin), bound(r.CookMin), bound(r.TotalMin)
	if r.TotalMin == 0 && (r.PrepMin > 0 || r.CookMin > 0) {
		r.TotalMin = r.PrepMin + r.CookMin
	}
	if r.Rating < 0 || r.Rating > 5 {
		r.Rating = 0
	}
	r.Tags = CleanTags(r.Tags)
	for k, v := range r.Nutrition {
		if strings.TrimSpace(v) == "" || len(k) > 40 {
			delete(r.Nutrition, k)
		} else {
			r.Nutrition[k] = clip(CleanLine(v), 60)
		}
	}
	if len(r.Extra) > 16000 {
		r.Extra = nil
	}
	r.VersionNote = clip(CleanLine(r.VersionNote), 120)
}

// SameLines reports whether two ingredient or step lists say the same thing,
// ignoring differences in spacing alone.
func SameLines(a, b []Line) bool {
	a, b = cleanLines(a, 1<<30, 1<<30), cleanLines(b, 1<<30, 1<<30)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].H != b[i].H || CleanLine(a[i].Text) != CleanLine(b[i].Text) {
			return false
		}
	}
	return true
}

// CleanTags trims, de-duplicates (case-insensitively) and bounds a tag list.
func CleanTags(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, t := range in {
		t = clip(CleanLine(t), 40)
		k := strings.ToLower(t)
		if t == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, t)
		if len(out) == 40 {
			break
		}
	}
	return out
}

func cleanLines(in []Line, max, maxLen int) []Line {
	out := make([]Line, 0, len(in))
	for _, l := range in {
		t := strings.TrimSpace(strings.ReplaceAll(l.Text, "\r", ""))
		if l.H {
			t = strings.TrimRight(CleanLine(t), ":")
		}
		if t == "" {
			continue
		}
		out = append(out, Line{Text: clip(t, maxLen), H: l.H})
		if len(out) == max {
			break
		}
	}
	return out
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s[max(0, len(s)-4):]) && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func bound(n int) int {
	if n < 0 || n > 60*24*30 {
		return 0
	}
	return n
}
