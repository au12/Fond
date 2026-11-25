package scrape

import (
	"bytes"
	"encoding/json"
	"errors"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/au12/fond/internal/model"
)

// ErrNoRecipe means the page was read but no recipe could be found on it.
var ErrNoRecipe = errors.New("no recipe found on that page")

// Result is what Extract found. Method says which strategy succeeded.
type Result struct {
	Recipe   model.Recipe
	ImageURL string
	Method   string // "json-ld", "microdata", "markup", "headings"
	Text     string // readable page text, for the optional AI fallback
}

// Extract finds a recipe in an HTML page. It tries, in order: schema.org
// JSON-LD, schema.org microdata/RDFa, well-known recipe-plugin markup, and
// finally "Ingredients"/"Instructions" headings. Gaps are filled from the
// page's Open Graph and meta tags.
func Extract(page []byte, pageURL string) (*Result, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	res := &Result{}
	meta := readMeta(doc)

	type cand struct {
		name string
		r    *model.Recipe
		img  string
	}
	var cands []cand
	for _, c := range []struct {
		name string
		fn   func(*goquery.Document) (*model.Recipe, string)
	}{{"json-ld", fromJSONLD}, {"microdata", fromMicrodata}, {"markup", fromPluginMarkup}, {"headings", fromHeadings}, {"classes", fromClasses}} {
		if r, img := c.fn(doc); r != nil && score(r) > 0 {
			cands = append(cands, cand{c.name, r, img})
		}
		// Structured data that is complete needs no second opinion.
		if n := len(cands); n == 1 && len(cands[0].r.Ingredients) >= 3 && len(cands[0].r.Steps) >= 1 && (len(cands[0].r.Ingredients) > 5 || len(cands[0].r.Steps) > 1) {
			break
		}
	}
	var best *model.Recipe
	if len(cands) > 0 {
		best, res.Method, res.ImageURL = cands[0].r, cands[0].name, cands[0].img
		// Some sites publish truncated structured data. Prefer a clearly
		// richer list found by a later strategy.
		for _, c := range cands[1:] {
			if n, m := len(c.r.Ingredients), len(best.Ingredients); n <= 80 && (m == 0 && n > 0 || m <= 5 && n >= 2*m+3 && c.name != "classes") {
				best.Ingredients = c.r.Ingredients
			}
			if n, m := len(c.r.Steps), len(best.Steps); n <= 80 && (m == 0 && n > 0 || m == 1 && n >= 3 && len(best.Steps[0].Text) < 400) {
				best.Steps = c.r.Steps
			}
			if best.Title == "" {
				best.Title = c.r.Title
			}
		}
	}
	if best == nil {
		best = &model.Recipe{}
	}
	r := best
	if r.Title == "" {
		r.Title = meta.title
	}
	if r.Description == "" {
		r.Description = meta.description
	}
	if res.ImageURL == "" {
		res.ImageURL = meta.image
	}
	if r.SourceName == "" {
		r.SourceName = meta.site
	}
	base, _ := url.Parse(pageURL)
	if meta.canonical != "" && base != nil {
		if cu, err := base.Parse(meta.canonical); err == nil && cu.Host == base.Host && cu.Path != "" && cu.Path != "/" {
			pageURL = cu.String()
		}
	}
	r.SourceURL = pageURL
	if base != nil {
		if r.SourceName == "" {
			r.SourceName = strings.TrimPrefix(base.Hostname(), "www.")
		}
		if res.ImageURL != "" {
			if iu, err := base.Parse(strings.TrimSpace(res.ImageURL)); err == nil && (iu.Scheme == "http" || iu.Scheme == "https") {
				res.ImageURL = iu.String()
			} else {
				res.ImageURL = ""
			}
		}
	}
	r.Title = cleanTitle(r.Title, r.SourceName)
	r.Normalize()
	res.Recipe = *r
	if len(r.Ingredients) == 0 && len(r.Steps) == 0 {
		res.Text = ReadableText(doc)
		return res, ErrNoRecipe
	}
	return res, nil
}

func score(r *model.Recipe) int {
	s := 0
	if r.Title != "" {
		s++
	}
	if len(r.Ingredients) > 0 {
		s += 10 + min(len(r.Ingredients), 5)
	}
	if len(r.Steps) > 0 {
		s += 10 + min(len(r.Steps), 5)
	}
	return s
}

// ---------------------------------------------------------------- text utils

var (
	reBreak    = regexp.MustCompile(`(?i)<\s*(br|/p|/li|/div|/h[1-6]|/tr)\s*/?>`)
	reTag      = regexp.MustCompile(`<[^>]*>`)
	reSpaces   = regexp.MustCompile(`[ \t\f\v\x{00a0}\x{2009}\x{202f}\x{200b}\x{feff}]+`)
	reStepNum  = regexp.MustCompile(`(?i)^\s*(?:step\s*\d+\s*[:.)\-–]*|\d{1,2}\s*[.)]\s+|\d{1,2}\s*[:\-–]\s+)`)
	reBullet   = regexp.MustCompile(`^\s*(?:[▢☐□■▪•●◦·*✓✔]|-\s|–\s)\s*`)
	reNumSplit = regexp.MustCompile(`\s+(?:\d{1,2}[.)])\s+`)
	reFirstNum = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
)

// cleanText strips markup and entities and keeps paragraph breaks as "\n".
func cleanText(s string) string {
	if strings.ContainsAny(s, "<&") {
		s = reBreak.ReplaceAllString(s, "\n")
		s = reTag.ReplaceAllString(s, "")
		s = html.UnescapeString(html.UnescapeString(s))
	}
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		l = strings.TrimSpace(reSpaces.ReplaceAllString(l, " "))
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func oneLine(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(cleanText(s), "\n", " "))
}

func cleanTitle(t, site string) string {
	t = oneLine(t)
	if i := strings.LastIndex(t, " | "); i > 8 {
		t = t[:i]
	}
	for _, sep := range []string{" | ", " - ", " – ", " — ", " :: "} {
		if i := strings.LastIndex(t, sep); i > 8 {
			tail := strings.ToLower(t[i+len(sep):])
			if site != "" && strings.Contains(strings.ToLower(site), tail) || strings.Contains(tail, strings.ToLower(site)) && site != "" ||
				strings.Contains(tail, "recipe") && len(tail) < 30 {
				t = t[:i]
			}
		}
	}
	for _, suffix := range []string{" Recipe", " recipe"} {
		if strings.HasSuffix(t, suffix) && len(t) > len(suffix)+6 {
			t = strings.TrimSuffix(t, suffix)
		}
	}
	return strings.TrimSpace(t)
}

func cleanIngredient(s string) string {
	s = oneLine(s)
	s = reBullet.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, "( ", "(")
	s = strings.ReplaceAll(s, " )", ")")
	return strings.TrimSpace(s)
}

func cleanStep(s string) string {
	s = strings.TrimSpace(s)
	s = reStepNum.ReplaceAllString(s, "")
	return strings.TrimSpace(reBullet.ReplaceAllString(s, ""))
}

// isHeadingish says whether a scraped line reads like a section label
// ("For the sauce:") rather than an ingredient or a step.
func isHeadingish(s string) bool {
	if len(s) == 0 || len(s) > 60 {
		return false
	}
	if strings.HasSuffix(s, ":") && !reFirstNum.MatchString(s) {
		return true
	}
	l := strings.ToLower(s)
	return (strings.HasPrefix(l, "for the ") || strings.HasPrefix(l, "for ")) && !reFirstNum.MatchString(s) && len(s) < 40 && !strings.Contains(s, ",")
}

func ingredientLines(raw []string) []model.Line {
	out := make([]model.Line, 0, len(raw))
	var prev string
	for _, s := range raw {
		for _, part := range strings.Split(cleanText(s), "\n") {
			t := cleanIngredient(part)
			if t == "" || t == prev {
				continue
			}
			prev = t
			out = append(out, model.Line{Text: t, H: isHeadingish(t)})
		}
	}
	return out
}

func stepLines(raw []string) []model.Line {
	out := make([]model.Line, 0, len(raw))
	for _, s := range raw {
		parts := strings.Split(cleanText(s), "\n")
		if len(parts) == 1 && len(parts[0]) > 300 {
			if sp := reNumSplit.Split(" "+parts[0], -1); len(sp) > 2 {
				parts = sp
			}
		}
		for _, part := range rejoin(parts) {
			t := cleanStep(part)
			if t == "" {
				continue
			}
			out = append(out, model.Line{Text: t, H: isHeadingish(t) && len(t) < 50})
		}
	}
	return out
}

// rejoin undoes hard line-wrapping: a line that does not end a sentence and is
// followed by one starting in lower case is the same paragraph.
func rejoin(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if n := len(out); n > 0 && p != "" && out[n-1] != "" {
			prev := out[n-1]
			last := prev[len(prev)-1]
			first := []rune(p)[0]
			if !strings.ContainsRune(".!?:;)\"”…", rune(last)) && last < 128 && first >= 'a' && first <= 'z' {
				out[n-1] = prev + " " + p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// ---------------------------------------------------------------- durations

var (
	reISO   = regexp.MustCompile(`(?i)^-?P(?:(\d+(?:\.\d+)?)W)?(?:(\d+(?:\.\d+)?)D)?(?:T(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)
	reHuman = regexp.MustCompile(`(?i)(\d+\s*[½¼¾⅓⅔]|\d+\s+\d/\d|\d/\d|\d+(?:[.,]\d+)?|[½¼¾⅓⅔])\s*(days?|d|hours?|hrs?|hr|h|std|stunden?|minutes?|mins?|min|m|seconds?|secs?|s)\b`)
)

var fracRunes = map[string]float64{"½": .5, "¼": .25, "¾": .75, "⅓": 1.0 / 3, "⅔": 2.0 / 3}

// ParseMinutes understands ISO-8601 durations ("PT1H30M") as well as free
// text ("1 hour 30 minutes", "45 min", "1h 15m", "90").
func ParseMinutes(v any) int {
	var s string
	switch t := v.(type) {
	case string:
		s = t
	case float64:
		return int(t)
	case map[string]any: // {"@type":"Duration","maxValue":"PT30M"}
		for _, k := range []string{"maxValue", "value", "minValue", "@value"} {
			if m := ParseMinutes(t[k]); m > 0 {
				return m
			}
		}
		return 0
	case []any:
		if len(t) > 0 {
			return ParseMinutes(t[0])
		}
		return 0
	default:
		return 0
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if m := reISO.FindStringSubmatch(strings.ReplaceAll(s, " ", "")); m != nil && len(s) > 1 {
		f := func(i int) float64 { x, _ := strconv.ParseFloat(m[i], 64); return x }
		total := f(1)*7*24*60 + f(2)*24*60 + f(3)*60 + f(4) + f(5)/60
		if total > 0 {
			return int(total + 0.5)
		}
		return 0
	}
	total := 0.0
	for _, m := range reHuman.FindAllStringSubmatch(s, -1) {
		n := parseNumber(m[1])
		switch u := strings.ToLower(m[2]); {
		case strings.HasPrefix(u, "d"):
			total += n * 1440
		case strings.HasPrefix(u, "h"), strings.HasPrefix(u, "st"):
			total += n * 60
		case strings.HasPrefix(u, "m"):
			total += n
		default:
			total += n / 60
		}
	}
	if total == 0 {
		if n, err := strconv.Atoi(s); err == nil { // bare number means minutes
			return n
		}
		if hh, mm, ok := strings.Cut(s, ":"); ok { // "1:30"
			h, e1 := strconv.Atoi(hh)
			m, e2 := strconv.Atoi(mm)
			if e1 == nil && e2 == nil {
				return h*60 + m
			}
		}
	}
	return int(total + 0.5)
}

func parseNumber(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if f, ok := fracRunes[s]; ok {
		return f
	}
	for sym, f := range fracRunes {
		if strings.HasSuffix(s, sym) {
			w, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, sym)), 64)
			return w + f
		}
	}
	if whole, frac, ok := strings.Cut(s, " "); ok && strings.Contains(frac, "/") {
		w, _ := strconv.ParseFloat(whole, 64)
		return w + parseNumber(strings.TrimSpace(frac))
	}
	if a, b, ok := strings.Cut(s, "/"); ok {
		x, _ := strconv.ParseFloat(strings.TrimSpace(a), 64)
		y, _ := strconv.ParseFloat(strings.TrimSpace(b), 64)
		if y != 0 {
			return x / y
		}
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// ParseYield turns "Makes 12 cookies" / 4 / ["4","4 servings"] into a display
// string and a number usable for scaling.
func ParseYield(v any) (string, float64) {
	var cands []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			if s := oneLine(t); s != "" {
				cands = append(cands, s)
			}
		case float64:
			cands = append(cands, strconv.FormatFloat(t, 'f', -1, 64))
		case []any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(v)
	if len(cands) == 0 {
		return "", 0
	}
	best := cands[0]
	for _, c := range cands {
		if strings.IndexFunc(c, func(r rune) bool { return r >= 'A' && r <= 'z' }) >= 0 && len(c) > len(best) && len(c) < 60 {
			best = c
		}
	}
	n := 0.0
	if m := reFirstNum.FindString(best); m != "" {
		n = parseNumber(m)
	}
	for _, p := range []string{"Makes ", "makes ", "Yield: ", "Yields ", "yields ", "Serves ", "serves ", "Servings: "} {
		if strings.HasPrefix(best, p) {
			rest := strings.TrimPrefix(best, p)
			if strings.HasPrefix(strings.ToLower(p), "serv") && reFirstNum.MatchString(rest) && !strings.ContainsAny(rest, "abcdefghijklmnopqrstuvwxyz") {
				rest += " servings"
			}
			best = rest
		}
	}
	if _, err := strconv.ParseFloat(best, 64); err == nil {
		if best == "1" {
			best += " serving"
		} else {
			best += " servings"
		}
	}
	return best, n
}

// ---------------------------------------------------------------- meta tags

type pageMeta struct{ title, description, image, site, canonical string }

func readMeta(doc *goquery.Document) pageMeta {
	var m pageMeta
	get := func(sel string) string {
		v, _ := doc.Find(sel).First().Attr("content")
		return oneLine(v)
	}
	m.title = get(`meta[property="og:title"]`)
	if m.title == "" {
		m.title = oneLine(doc.Find("h1").First().Text())
	}
	if m.title == "" {
		m.title = oneLine(doc.Find("title").First().Text())
	}
	m.description = get(`meta[property="og:description"]`)
	if m.description == "" {
		m.description = get(`meta[name="description"]`)
	}
	m.image = get(`meta[property="og:image:secure_url"]`)
	for _, sel := range []string{`meta[property="og:image"]`, `meta[name="og:image"]`, `meta[name="twitter:image"]`, `meta[property="twitter:image"]`, `meta[name="twitter:image:src"]`} {
		if m.image == "" {
			m.image = get(sel)
		}
	}
	m.site = get(`meta[property="og:site_name"]`)
	m.canonical, _ = doc.Find(`link[rel="canonical"]`).First().Attr("href")
	return m
}

// ReadableText returns the visible text of a page, for the AI fallback.
func ReadableText(doc *goquery.Document) string {
	body := doc.Find("body").Clone()
	body.Find("script,style,noscript,svg,nav,header,footer,form,iframe,aside,button,select").Remove()
	root := body
	for _, sel := range []string{"article", "main", `[role="main"]`} {
		if s := body.Find(sel).First(); s.Length() > 0 && len(s.Text()) > 600 {
			root = s
			break
		}
	}
	h, _ := root.Html()
	t := cleanText(h)
	if len(t) > 24000 {
		t = t[:24000]
	}
	return t
}

// ---------------------------------------------------------------- JSON-LD

func typeIs(node map[string]any, want string) bool {
	match := func(s string) bool {
		s = strings.ToLower(s)
		if i := strings.LastIndexAny(s, "/:#"); i >= 0 {
			s = s[i+1:]
		}
		return s == want
	}
	switch t := node["@type"].(type) {
	case string:
		return match(t)
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && match(s) {
				return true
			}
		}
	}
	return false
}

var reCtl = regexp.MustCompile(`[\x00-\x1f]`)

func parseLD(raw string) any {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "<!--")
	raw = strings.TrimSuffix(raw, "-->")
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "//<![CDATA[")
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "//]]>")
	raw = strings.TrimSuffix(strings.TrimSpace(raw), ";")
	var v any
	if json.Unmarshal([]byte(raw), &v) == nil {
		return v
	}
	// Raw control characters inside strings are the most common breakage.
	fixed := reCtl.ReplaceAllString(raw, " ")
	if json.Unmarshal([]byte(fixed), &v) == nil {
		return v
	}
	// Several objects glued together, or trailing garbage: decode as a stream.
	dec := json.NewDecoder(strings.NewReader(fixed))
	var all []any
	for {
		var x any
		if err := dec.Decode(&x); err != nil {
			break
		}
		all = append(all, x)
	}
	if len(all) > 0 {
		return all
	}
	return nil
}

func fromJSONLD(doc *goquery.Document) (*model.Recipe, string) {
	index := map[string]map[string]any{}
	var recipes []map[string]any
	var walk func(v any, depth int)
	walk = func(v any, depth int) {
		if depth > 12 {
			return
		}
		switch t := v.(type) {
		case []any:
			for _, e := range t {
				walk(e, depth+1)
			}
		case map[string]any:
			if id, ok := t["@id"].(string); ok && len(t) > 1 {
				if old, seen := index[id]; !seen || len(t) > len(old) {
					index[id] = t
				}
			}
			if typeIs(t, "recipe") {
				recipes = append(recipes, t)
			}
			for k, e := range t {
				if k == "@graph" || k == "mainEntity" || k == "mainEntityOfPage" || k == "hasPart" || k == "itemListElement" || k == "item" || k == "about" || k == "subjectOf" {
					walk(e, depth+1)
				}
			}
		}
	}
	doc.Find(`script[type="application/ld+json"], script[type="application/ld+json;charset=utf-8"]`).Each(func(_ int, s *goquery.Selection) {
		if v := parseLD(s.Text()); v != nil {
			walk(v, 0)
		}
	})
	if len(recipes) == 0 {
		return nil, ""
	}
	best := recipes[0]
	for _, n := range recipes[1:] {
		if listLen(n["recipeIngredient"])+listLen(n["recipeInstructions"]) > listLen(best["recipeIngredient"])+listLen(best["recipeInstructions"]) {
			best = n
		}
	}
	return ldRecipe(best, index)
}

func listLen(v any) int {
	switch t := v.(type) {
	case []any:
		return len(t)
	case nil:
		return 0
	}
	return 1
}

func deref(v any, index map[string]map[string]any) any {
	if m, ok := v.(map[string]any); ok && len(m) == 1 {
		if id, ok := m["@id"].(string); ok {
			if n, ok := index[id]; ok {
				return n
			}
		}
	}
	return v
}

func ldString(v any, index map[string]map[string]any) string {
	switch t := deref(v, index).(type) {
	case string:
		return oneLine(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case map[string]any:
		for _, k := range []string{"name", "@value", "text", "url"} {
			if s := ldString(t[k], index); s != "" {
				return s
			}
		}
	case []any:
		for _, e := range t {
			if s := ldString(e, index); s != "" {
				return s
			}
		}
	}
	return ""
}

func ldStrings(v any, index map[string]map[string]any) []string {
	var out []string
	switch t := deref(v, index).(type) {
	case string:
		out = append(out, t)
	case []any:
		for _, e := range t {
			out = append(out, ldStrings(e, index)...)
		}
	case map[string]any:
		if s := ldString(t, index); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func ldImage(v any, index map[string]map[string]any) string {
	best, bestW := "", -1.0
	var walk func(any)
	walk = func(x any) {
		switch t := deref(x, index).(type) {
		case string:
			if bestW < 0 && strings.TrimSpace(t) != "" {
				best, bestW = strings.TrimSpace(t), 0
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			u, _ := t["url"].(string)
			if u == "" {
				u, _ = t["contentUrl"].(string)
			}
			if u == "" {
				return
			}
			w := 0.0
			switch wv := t["width"].(type) {
			case float64:
				w = wv
			case string:
				w, _ = strconv.ParseFloat(strings.TrimSuffix(wv, "px"), 64)
			}
			if w > bestW {
				best, bestW = strings.TrimSpace(u), w
			}
		}
	}
	walk(v)
	return best
}

func ldSteps(v any, index map[string]map[string]any, out *[]model.Line, depth int) {
	if depth > 6 {
		return
	}
	switch t := deref(v, index).(type) {
	case string:
		*out = append(*out, stepLines([]string{t})...)
	case []any:
		for _, e := range t {
			ldSteps(e, index, out, depth+1)
		}
	case map[string]any:
		items := t["itemListElement"]
		if items == nil {
			items = t["steps"]
		}
		if items != nil {
			if name := ldString(t["name"], index); name != "" && typeIs(t, "howtosection") {
				*out = append(*out, model.Line{Text: strings.TrimRight(name, ":"), H: true})
			}
			ldSteps(items, index, out, depth+1)
			return
		}
		text, _ := t["text"].(string)
		if strings.TrimSpace(text) == "" {
			text, _ = t["description"].(string)
		}
		if strings.TrimSpace(text) == "" {
			text, _ = t["name"].(string)
		}
		*out = append(*out, stepLines([]string{text})...)
	}
}

var nutritionKeys = map[string]string{
	"calories": "calories", "fatContent": "fat", "saturatedFatContent": "saturatedFat", "carbohydrateContent": "carbs",
	"sugarContent": "sugar", "fiberContent": "fiber", "proteinContent": "protein", "sodiumContent": "sodium",
	"cholesterolContent": "cholesterol", "servingSize": "servingSize",
}

func ldRecipe(n map[string]any, index map[string]map[string]any) (*model.Recipe, string) {
	r := &model.Recipe{}
	r.Title = ldString(n["name"], index)
	if r.Title == "" {
		r.Title = ldString(n["headline"], index)
	}
	if d, ok := n["description"].(string); ok {
		r.Description = cleanText(d)
	}
	ing := n["recipeIngredient"]
	if ing == nil {
		ing = n["ingredients"]
	}
	r.Ingredients = ingredientLines(ldStrings(ing, index))
	ldSteps(n["recipeInstructions"], index, &r.Steps, 0)
	r.Yield, r.Servings = ParseYield(n["recipeYield"])
	if r.Yield == "" {
		r.Yield, r.Servings = ParseYield(n["yield"])
	}
	r.PrepMin = ParseMinutes(n["prepTime"])
	r.CookMin = ParseMinutes(n["cookTime"])
	r.TotalMin = ParseMinutes(n["totalTime"])
	r.Author = ldString(n["author"], index)
	if pub := ldString(n["publisher"], index); pub != "" && !strings.HasPrefix(pub, "http") {
		r.SourceName = pub
	}
	for _, key := range []string{"recipeCategory", "recipeCuisine"} {
		for _, s := range ldStrings(n[key], index) {
			for _, part := range strings.Split(s, ",") {
				if part = oneLine(part); part != "" && len(part) <= 30 {
					r.Tags = append(r.Tags, titleCase(part))
				}
			}
		}
	}
	if len(r.Tags) > 6 {
		r.Tags = r.Tags[:6]
	}
	if nut, ok := deref(n["nutrition"], index).(map[string]any); ok {
		r.Nutrition = map[string]string{}
		for k, mine := range nutritionKeys {
			if s := ldString(nut[k], index); s != "" {
				r.Nutrition[mine] = s
			}
		}
	}
	return r, ldImage(n["image"], index)
}

func titleCase(s string) string {
	if s == "" || strings.ToLower(s) != s {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ---------------------------------------------------------------- microdata

func propValue(s *goquery.Selection) string {
	for _, a := range []string{"content", "datetime"} {
		if v, ok := s.Attr(a); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	if goquery.NodeName(s) == "img" || goquery.NodeName(s) == "link" || goquery.NodeName(s) == "a" && s.Text() == "" {
		for _, a := range []string{"src", "data-src", "href"} {
			if v, ok := s.Attr(a); ok {
				return v
			}
		}
	}
	h, _ := s.Html()
	return h
}

func fromMicrodata(doc *goquery.Document) (*model.Recipe, string) {
	scope := doc.Find(`[itemtype*="schema.org/Recipe"], [typeof="Recipe"], [typeof="schema:Recipe"]`).First()
	attr := "itemprop"
	if scope.Length() == 0 {
		return nil, ""
	}
	if _, ok := scope.Attr("typeof"); ok {
		attr = "property"
	}
	// A property belongs to the recipe only if no other item scope sits between.
	own := func(s *goquery.Selection) bool {
		p := s.Parent()
		for p.Length() > 0 && p.Get(0) != scope.Get(0) {
			if _, ok := p.Attr("itemscope"); ok {
				return false
			}
			p = p.Parent()
		}
		return true
	}
	props := func(names ...string) []string {
		var out []string
		for _, n := range names {
			scope.Find(`[` + attr + `~="` + n + `"]`).Each(func(_ int, s *goquery.Selection) {
				if own(s) {
					out = append(out, propValue(s))
				}
			})
			if len(out) > 0 {
				break
			}
		}
		return out
	}
	first := func(names ...string) string {
		if p := props(names...); len(p) > 0 {
			return p[0]
		}
		return ""
	}
	r := &model.Recipe{}
	r.Title = oneLine(first("name"))
	r.Description = cleanText(first("description"))
	rawIng := props("recipeIngredient", "ingredients", "ingredient")
	if len(rawIng) > 1 {
		for i, x := range rawIng {
			rawIng[i] = strings.ReplaceAll(cleanText(x), "\n", " ")
		}
	}
	r.Ingredients = ingredientLines(rawIng)
	r.Steps = stepLines(props("recipeInstructions", "instructions", "step"))
	r.Yield, r.Servings = ParseYield(oneLine(first("recipeYield", "yield")))
	r.PrepMin = ParseMinutes(oneLine(first("prepTime")))
	r.CookMin = ParseMinutes(oneLine(first("cookTime")))
	r.TotalMin = ParseMinutes(oneLine(first("totalTime")))
	if a := scope.Find(`[` + attr + `~="author"]`).First(); a.Length() > 0 {
		if nm := a.Find(`[` + attr + `~="name"]`).First(); nm.Length() > 0 {
			r.Author = oneLine(propValue(nm))
		} else {
			r.Author = oneLine(propValue(a))
		}
	}
	img := ""
	scope.Find(`[` + attr + `~="image"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		img = strings.TrimSpace(propValue(s))
		if strings.Contains(img, "<") {
			img, _ = s.Find("img").First().Attr("src")
		}
		return img == ""
	})
	if len(r.Ingredients) == 0 && len(r.Steps) == 0 {
		return nil, ""
	}
	return r, img
}

// ---------------------------------------------------------------- plugin markup

var pluginSelectors = []struct{ ing, group, step, stepGroup, title string }{
	{".wprm-recipe-ingredient", ".wprm-recipe-ingredient-group-name", ".wprm-recipe-instruction-text", ".wprm-recipe-instruction-group-name", ".wprm-recipe-name"},
	{".tasty-recipes-ingredients li, .tasty-recipe-ingredients li", ".tasty-recipes-ingredients h4", ".tasty-recipes-instructions li, .tasty-recipe-instructions li", ".tasty-recipes-instructions h4", ".tasty-recipes-title"},
	{".mv-create-ingredients li", ".mv-create-ingredients h4", ".mv-create-instructions li", ".mv-create-instructions h4", ".mv-create-title"},
	{".easyrecipe .ingredient, .ERSIngredients li", "", ".easyrecipe .instruction, .ERSInstructions li", "", ".ERSName"},
	{".recipe-card-ingredients li, .recipe-ingredients li, .recipe__ingredients li, .recipe-ingred_txt, .ingredients-list li, .ingredient-list li, .ingredients__list li, ul.ingredients li, .ingredients ul li, [class*='ingredients-item'], [class*='IngredientList'] li, [data-ingredient]",
		"",
		".recipe-card-instructions li, .recipe-instructions li, .recipe__instructions li, .recipe-directions__list--item, .instructions-list li, .directions-list li, .method__list li, .recipe-method li, ol.instructions li, .instructions ol li, .directions ol li, .method ol li, .preparation ol li, [class*='InstructionList'] li, [class*='instructions-section'] p, [class*='preparation-step']",
		"", ""},
}

func fromPluginMarkup(doc *goquery.Document) (*model.Recipe, string) {
	for _, p := range pluginSelectors {
		ings := collect(doc, p.ing, p.group, false)
		steps := collect(doc, p.step, p.stepGroup, true)
		if len(ings) < 2 && len(steps) < 2 {
			continue
		}
		r := &model.Recipe{Ingredients: ings, Steps: steps}
		if p.title != "" {
			r.Title = oneLine(doc.Find(p.title).First().Text())
		}
		return r, ""
	}
	return nil, ""
}

func collect(doc *goquery.Document, sel, groupSel string, steps bool) []model.Line {
	if groupSel != "" {
		sel += ", " + groupSel
	}
	var out []model.Line
	seen := map[string]bool{}
	doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
		h, _ := s.Html()
		isGroup := groupSel != "" && s.Is(groupSel)
		var lines []model.Line
		if steps {
			lines = stepLines([]string{h})
		} else {
			lines = ingredientLines([]string{strings.Join(strings.Fields(strings.ReplaceAll(cleanText(h), "\n", " ")), " ")})
		}
		for _, l := range lines {
			if isGroup {
				l.H = true
			}
			if seen[l.Text] && !steps {
				continue
			}
			seen[l.Text] = true
			out = append(out, l)
		}
	})
	return out
}

// fromClasses is the loosest strategy: leaf elements whose class name mentions
// ingredients or instructions.
func fromClasses(doc *goquery.Document) (*model.Recipe, string) {
	leaves := func(sel string, minLen, maxLen int) []string {
		var out []string
		seen := map[string]bool{}
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			if s.Find(sel).Length() > 0 || s.Closest("nav,header,footer,form,aside").Length() > 0 {
				return
			}
			h, _ := s.Html()
			t := strings.ReplaceAll(cleanText(h), "\n", " ")
			if len(t) < minLen || len(t) > maxLen || seen[t] {
				return
			}
			seen[t] = true
			out = append(out, t)
		})
		return out
	}
	ings := leaves(`[class*="ingredient"], [class*="Ingredient"], [class*="zutat"]`, 2, 160)
	steps := leaves(`[class*="instruction"], [class*="Instruction"], [class*="direction"], [class*="Direction"], [class*="method-step"], [class*="preparation"], [class*="recipe-step"], [class*="RecipeStep"], [class*="step-text"], [class*="steps__"]`, 25, 4000)
	if len(ings) < 3 || len(ings) > 80 {
		ings = nil
	}
	if len(steps) > 60 {
		steps = nil
	}
	if ings == nil && len(steps) < 2 {
		return nil, ""
	}
	return &model.Recipe{Ingredients: ingredientLines(ings), Steps: stepLines(steps)}, ""
}

// ---------------------------------------------------------------- headings

var (
	reIngHead  = regexp.MustCompile(`(?i)^(ingredients?|what you(['’]ll| will)? need|you(['’]ll| will) need|zutaten|ingr[ée]dients?|ingredientes|ingredienti|ingredi[ëe]nten|ingredienser|składniki)\b.{0,24}$`)
	reStepHead = regexp.MustCompile(`(?i)^(instructions?|directions?|method|preparation|steps|how to make.{0,20}|procedure|zubereitung|anleitung|pr[ée]paration|preparaci[óo]n|elaboraci[óo]n|preparazione|procedimento|bereiding(swijze)?|g[öo]r s[åa] h[äa]r|instruktioner|przygotowanie)\b.{0,24}$`)
	reAnyHead  = regexp.MustCompile(`(?i)^(notes?|tips?|nutrition|nutritional|recipe notes|equipment|video|reviews?|comments?|related|you may also|more recipes|faq|storage|variations?|did you make)`)
)

const headTags = "h1,h2,h3,h4,h5,h6,strong,b,p,div,span,dt,legend"

func fromHeadings(doc *goquery.Document) (*model.Recipe, string) {
	var ingHead, stepHead *goquery.Selection
	doc.Find(headTags).Each(func(_ int, s *goquery.Selection) {
		if s.Children().Length() > 2 {
			return
		}
		t := strings.TrimRight(oneLine(s.Text()), ": ")
		if len(t) < 4 || len(t) > 44 {
			return
		}
		if ingHead == nil && reIngHead.MatchString(t) {
			ingHead = s
		} else if stepHead == nil && reStepHead.MatchString(t) {
			stepHead = s
		}
	})
	if ingHead == nil && stepHead == nil {
		return nil, ""
	}
	r := &model.Recipe{}
	if ingHead != nil {
		r.Ingredients = ingredientLines(following(ingHead, false))
	}
	if stepHead != nil {
		r.Steps = stepLines(following(stepHead, true))
	}
	if len(r.Ingredients) == 0 && len(r.Steps) == 0 {
		return nil, ""
	}
	return r, ""
}

// following gathers the list items (or, failing that, paragraphs) that come
// after a heading, climbing to the parent when the heading is wrapped.
func following(head *goquery.Selection, steps bool) []string {
	isStop := func(s *goquery.Selection) bool {
		t := strings.TrimRight(oneLine(s.Text()), ": ")
		if len(t) > 44 || len(t) < 3 {
			return false
		}
		return reIngHead.MatchString(t) || reStepHead.MatchString(t) || reAnyHead.MatchString(t)
	}
	node := head
	for level := 0; level < 4; level++ {
		var items, paras []string
		stopped := false
		node.NextAll().EachWithBreak(func(_ int, sib *goquery.Selection) bool {
			name := goquery.NodeName(sib)
			if (strings.HasPrefix(name, "h") && len(name) == 2 || sib.Children().Length() <= 2) && isStop(sib) {
				stopped = true
				return false
			}
			if sib.Find("h1,h2,h3,h4,h5,h6,strong,b").FilterFunction(func(_ int, x *goquery.Selection) bool { return isStop(x) }).Length() > 0 && len(items)+len(paras) > 0 {
				stopped = true
				return false
			}
			lis := sib.Find("li, tr")
			if name == "li" || name == "tr" {
				lis = sib
			}
			lis.Each(func(_ int, li *goquery.Selection) {
				if li.Find("li, tr").Length() == 0 {
					h, _ := li.Html()
					items = append(items, strings.ReplaceAll(cleanText(strings.ReplaceAll(h, "</td>", " </td>")), "\n", " "))
				}
			})
			if lis.Length() == 0 {
				ps := sib.Find("p")
				if name == "p" || name == "div" && ps.Length() == 0 {
					ps = sib
				}
				ps.Each(func(_ int, p *goquery.Selection) {
					h, _ := p.Html()
					if t := cleanText(h); len(t) > 2 {
						paras = append(paras, t)
					}
				})
			}
			return len(items) < 200
		})
		if len(items) >= 2 || (steps && len(items) >= 1) {
			return items
		}
		if len(paras) >= 1 && (steps || len(paras) >= 2) {
			var out []string
			for _, p := range paras {
				out = append(out, strings.Split(p, "\n")...)
			}
			if !steps || len(out) > 0 {
				return out
			}
		}
		if stopped && len(items)+len(paras) > 0 {
			return append(items, paras...)
		}
		node = node.Parent()
		if node.Length() == 0 || goquery.NodeName(node) == "body" {
			break
		}
	}
	return nil
}

// FromText parses a pasted plain-text recipe: first line is the title, then
// "Ingredients" and "Instructions/Directions/Method" headings split the rest.
func FromText(text string) (*model.Recipe, error) {
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		lines = append(lines, strings.TrimSpace(reSpaces.ReplaceAllString(l, " ")))
	}
	r := &model.Recipe{}
	section := "intro"
	var intro, notes []string
	for _, l := range lines {
		bare := strings.TrimRight(strings.Trim(l, "#*_ "), ": ")
		switch {
		case l == "":
			if section == "steps" || section == "intro" {
				continue
			}
			continue
		case len(bare) < 44 && reIngHead.MatchString(bare):
			section = "ing"
		case len(bare) < 44 && reStepHead.MatchString(bare):
			section = "steps"
		case len(bare) < 30 && reAnyHead.MatchString(bare) && section != "intro":
			section = "notes"
		case section == "intro":
			intro = append(intro, strings.Trim(l, "#* "))
		case section == "ing":
			t := cleanIngredient(strings.Trim(l, "*_"))
			r.Ingredients = append(r.Ingredients, model.Line{Text: t, H: isHeadingish(t) || strings.HasPrefix(l, "#")})
		case section == "steps":
			t := cleanStep(l)
			r.Steps = append(r.Steps, model.Line{Text: strings.Trim(t, "#"), H: strings.HasPrefix(l, "#") || isHeadingish(t)})
		default:
			notes = append(notes, l)
		}
	}
	if len(intro) > 0 {
		r.Title = intro[0]
		rest := intro[1:]
		var desc []string
		for _, l := range rest {
			ll := strings.ToLower(l)
			switch {
			case strings.HasPrefix(ll, "serves") || strings.HasPrefix(ll, "makes") || strings.HasPrefix(ll, "yield") || strings.HasPrefix(ll, "servings"):
				r.Yield, r.Servings = ParseYield(l)
			case strings.HasPrefix(ll, "prep"):
				r.PrepMin = ParseMinutes(l)
			case strings.HasPrefix(ll, "cook") || strings.HasPrefix(ll, "bake"):
				r.CookMin = ParseMinutes(l)
			case strings.HasPrefix(ll, "total"):
				r.TotalMin = ParseMinutes(l)
			case strings.HasPrefix(ll, "http"):
				r.SourceURL = l
			default:
				desc = append(desc, l)
			}
		}
		r.Description = strings.Join(desc, "\n")
	}
	r.Notes = strings.Join(notes, "\n")
	r.Normalize()
	if r.Title == "" || (len(r.Ingredients) == 0 && len(r.Steps) == 0) {
		return r, ErrNoRecipe
	}
	return r, nil
}
