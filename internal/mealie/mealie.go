// Package mealie migrates recipes out of Mealie. It understands three
// sources: a full backup zip (database.json + data/), the "Data Management"
// recipe export zip (recipes/<slug>/<slug>.json), and a live server's API.
package mealie

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/au12/fond/internal/model"
	"github.com/au12/fond/internal/scrape"
)

// Item is one migrated recipe.
type Item struct {
	MealieID string
	Recipe   model.Recipe
	Image    []byte
	Favorite bool
}

// Book is a Mealie cookbook resolved to the recipes it contained.
type Book struct {
	Name, Description string
	RecipeIDs         []string
}

// Sink receives migrated data.
type Sink interface {
	Recipe(Item) error
	Cookbook(Book)
	Progress(done, total int, current string)
}

type row = map[string]any

// get reads the first present key; Mealie emits snake_case in exports and
// backups but camelCase from its API.
func get(m row, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func str(m row, keys ...string) string {
	switch t := get(m, keys...).(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return ""
}

func num(m row, keys ...string) float64 {
	switch t := get(m, keys...).(type) {
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	case bool:
		if t {
			return 1
		}
	}
	return 0
}

func truthy(m row, keys ...string) bool {
	switch t := get(m, keys...).(type) {
	case bool:
		return t
	case float64:
		return t != 0
	}
	return false
}

func obj(m row, keys ...string) row {
	r, _ := get(m, keys...).(map[string]any)
	return r
}

func list(m row, keys ...string) []row {
	arr, _ := get(m, keys...).([]any)
	out := make([]row, 0, len(arr))
	for _, e := range arr {
		if r, ok := e.(map[string]any); ok {
			out = append(out, r)
		}
	}
	return out
}

func normID(s string) string { return strings.ToLower(strings.ReplaceAll(s, "-", "")) }

func ts(s string) int64 {
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05.999999", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}

// ------------------------------------------------------------------ ingredient text

// Quantity renders a number the way a cook writes it: 1.5 -> "1 1/2".
func Quantity(q float64, fraction bool) string {
	if q <= 0 {
		return ""
	}
	whole := math.Floor(q)
	frac := q - whole
	if frac < 0.005 {
		return strconv.Itoa(int(whole))
	}
	if fraction {
		for _, d := range []int{2, 3, 4, 8, 5, 6, 10, 16} {
			n := math.Round(frac * float64(d))
			if n > 0 && n < float64(d) && math.Abs(frac-n/float64(d)) < 0.006 {
				f := fmt.Sprintf("%d/%d", int(n), d)
				if whole > 0 {
					return fmt.Sprintf("%d %s", int(whole), f)
				}
				return f
			}
		}
	}
	return strconv.FormatFloat(math.Round(q*1000)/1000, 'f', -1, 64)
}

// ingredientText mirrors Mealie's own display rule: quantity, unit, food, note.
func ingredientText(qty float64, unit, food row, note, original, refName string) string {
	var parts []string
	fraction := unit == nil || get(unit, "fraction") == nil || truthy(unit, "fraction")
	if q := Quantity(qty, fraction); q != "" {
		parts = append(parts, q)
		if unit != nil {
			u := ""
			plural := qty > 1
			if truthy(unit, "use_abbreviation", "useAbbreviation") {
				if plural {
					u = str(unit, "plural_abbreviation", "pluralAbbreviation")
				}
				if u == "" {
					u = str(unit, "abbreviation")
				}
			}
			if u == "" && plural {
				u = str(unit, "plural_name", "pluralName")
			}
			if u == "" {
				u = str(unit, "name")
			}
			if u != "" {
				parts = append(parts, u)
			}
		}
	}
	if food != nil {
		name := str(food, "name")
		if !(qty > 0 && qty <= 1) && !(qty > 0 && unit != nil) {
			if p := str(food, "plural_name", "pluralName"); p != "" {
				name = p
			}
		}
		if name != "" {
			parts = append(parts, name)
		}
	} else if refName != "" {
		parts = append(parts, refName)
	}
	text := strings.Join(parts, " ")
	if note != "" {
		if text != "" && (food != nil || refName != "") {
			text += ", " + note
		} else {
			text = strings.TrimSpace(text + " " + note)
		}
	}
	if text == "" {
		text = original
	}
	return text
}

var nutritionMap = [][3]string{
	{"calories", "calories", "calories"}, {"fat_content", "fatContent", "fat"}, {"saturated_fat_content", "saturatedFatContent", "saturatedFat"},
	{"carbohydrate_content", "carbohydrateContent", "carbs"}, {"sugar_content", "sugarContent", "sugar"}, {"fiber_content", "fiberContent", "fiber"},
	{"protein_content", "proteinContent", "protein"}, {"sodium_content", "sodiumContent", "sodium"}, {"cholesterol_content", "cholesterolContent", "cholesterol"},
}

func nutrition(m row) map[string]string {
	if m == nil {
		return nil
	}
	out := map[string]string{}
	for _, k := range nutritionMap {
		if v := str(m, k[0], k[1]); v != "" {
			out[k[2]] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// base fills the scalar fields shared by every Mealie representation.
func base(m row) model.Recipe {
	r := model.Recipe{
		Title:       str(m, "name"),
		Description: str(m, "description"),
		SourceURL:   str(m, "org_url", "orgURL", "orgUrl"),
		Rating:      num(m, "rating"),
	}
	mins := func(sec, text []string) int {
		if s := num(m, sec...); s > 0 {
			return int(s/60 + 0.5)
		}
		return scrape.ParseMinutes(str(m, text...))
	}
	r.PrepMin = mins([]string{"prep_time_seconds", "prepTimeSeconds"}, []string{"prep_time", "prepTime"})
	r.CookMin = mins([]string{"perform_time_seconds", "performTimeSeconds"}, []string{"perform_time", "performTime", "cook_time", "cookTime"})
	r.TotalMin = mins([]string{"total_time_seconds", "totalTimeSeconds"}, []string{"total_time", "totalTime"})
	r.Yield = str(m, "recipe_yield", "recipeYield")
	r.Servings = num(m, "recipe_servings", "recipeServings")
	if r.Servings == 0 {
		r.Servings = num(m, "recipe_yield_quantity", "recipeYieldQuantity")
	}
	if r.Servings == 0 && r.Yield != "" {
		_, r.Servings = scrape.ParseYield(r.Yield)
	}
	if r.Yield == "" && r.Servings > 0 {
		r.Yield = strconv.FormatFloat(r.Servings, 'f', -1, 64) + " servings"
	}
	if u, err := url.Parse(r.SourceURL); err == nil && u.Host != "" {
		r.SourceName = strings.TrimPrefix(u.Hostname(), "www.")
	}
	r.CreatedAt = ts(str(m, "created_at", "createdAt"))
	if r.CreatedAt == 0 {
		r.CreatedAt = ts(str(m, "date_added", "dateAdded"))
	}
	r.UpdatedAt = ts(str(m, "update_at", "updatedAt", "updated_at", "date_updated", "dateUpdated"))
	if lm := ts(str(m, "last_made", "lastMade")); lm > 0 {
		r.LastCooked, r.CookedCount = lm, 1
	}
	return r
}

// fromObject converts a nested recipe object (export zip or API).
func fromObject(m row) model.Recipe {
	r := base(m)
	for _, key := range [][]string{{"recipe_category", "recipeCategory"}, {"tags"}, {"tools"}} {
		for _, t := range list(m, key...) {
			r.Tags = append(r.Tags, str(t, "name"))
		}
	}
	for _, ing := range list(m, "recipe_ingredient", "recipeIngredient") {
		if title := str(ing, "title"); title != "" {
			r.Ingredients = append(r.Ingredients, model.Line{Text: title, H: true})
		}
		text := str(ing, "display")
		if text == "" {
			ref := ""
			if rr := obj(ing, "referenced_recipe", "referencedRecipe"); rr != nil {
				ref = str(rr, "name")
			}
			text = ingredientText(num(ing, "quantity"), obj(ing, "unit"), obj(ing, "food"), str(ing, "note"), str(ing, "original_text", "originalText"), ref)
		}
		if text != "" {
			r.Ingredients = append(r.Ingredients, model.Line{Text: text})
		}
	}
	for _, st := range list(m, "recipe_instructions", "recipeInstructions") {
		if title := str(st, "title"); title != "" {
			r.Steps = append(r.Steps, model.Line{Text: title, H: true})
		}
		if text := str(st, "text"); text != "" {
			r.Steps = append(r.Steps, model.Line{Text: text})
		}
	}
	r.Notes = joinNotes(list(m, "notes"))
	r.Nutrition = nutrition(obj(m, "nutrition"))
	return r
}

func joinNotes(notes []row) string {
	var parts []string
	for _, n := range notes {
		title, text := str(n, "title"), str(n, "text")
		switch {
		case title != "" && text != "":
			parts = append(parts, "**"+title+"**\n"+text)
		case title+text != "":
			parts = append(parts, title+text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// ------------------------------------------------------------------ zip import

type archive struct {
	files map[string]*zip.File
}

func (a *archive) read(name string, limit int64) []byte {
	f := a.files[strings.ToLower(name)]
	if f == nil || int64(f.UncompressedSize64) > limit {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, limit))
	if err != nil {
		return nil
	}
	return b
}

func (a *archive) image(dir string) []byte {
	for _, n := range []string{"original.webp", "original.jpg", "original.jpeg", "original.png", "min-original.webp"} {
		if b := a.read(dir+"/images/"+n, 60<<20); b != nil {
			return b
		}
	}
	return nil
}

// ImportZip migrates a Mealie backup or recipe-export zip.
func ImportZip(zipPath string, sink Sink) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return errors.New("that file isn't a readable zip archive")
	}
	defer zr.Close()
	a := &archive{files: map[string]*zip.File{}}
	dbName := ""
	var recipeJSON []string
	for _, f := range zr.File {
		name := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.Contains(name, "__MACOSX") || strings.HasPrefix(path.Base(name), "._") || f.FileInfo().IsDir() {
			continue
		}
		a.files[strings.ToLower(name)] = f
		if path.Base(name) == "database.json" && (dbName == "" || len(name) < len(dbName)) {
			dbName = name
		}
		// recipes/<slug>/<slug>.json
		if strings.HasSuffix(name, ".json") && path.Base(path.Dir(name))+".json" == path.Base(name) {
			recipeJSON = append(recipeJSON, name)
		}
	}
	switch {
	case dbName != "":
		return importBackup(a, dbName, sink)
	case len(recipeJSON) > 0:
		sort.Strings(recipeJSON)
		for i, name := range recipeJSON {
			var m row
			if json.Unmarshal(a.read(name, 20<<20), &m) != nil || str(m, "name") == "" {
				continue
			}
			item := Item{MealieID: normID(str(m, "id")), Recipe: fromObject(m), Image: a.image(path.Dir(name))}
			if item.MealieID == "" {
				item.MealieID = str(m, "slug")
			}
			sink.Progress(i, len(recipeJSON), item.Recipe.Title)
			if err := sink.Recipe(item); err != nil {
				return err
			}
		}
		sink.Progress(len(recipeJSON), len(recipeJSON), "")
		return nil
	}
	return errors.New("this zip doesn't look like a Mealie backup or recipe export (no database.json or recipes/ folder found)")
}

func importBackup(a *archive, dbName string, sink Sink) error {
	raw := a.read(dbName, 2<<30)
	if raw == nil {
		return errors.New("database.json could not be read")
	}
	var db map[string][]row
	if err := json.Unmarshal(raw, &db); err != nil {
		return fmt.Errorf("database.json is not valid: %w", err)
	}
	raw = nil
	prefix := path.Dir(dbName)
	if prefix == "." {
		prefix = ""
	} else {
		prefix += "/"
	}

	byID := func(table string) map[string]row {
		out := map[string]row{}
		for _, r := range db[table] {
			out[normID(str(r, "id"))] = r
		}
		return out
	}
	units, foods := byID("ingredient_units"), byID("ingredient_foods")
	cats, tags, tools := byID("categories"), byID("tags"), byID("tools")
	recipeNames := map[string]string{}
	for _, r := range db["recipes"] {
		recipeNames[normID(str(r, "id"))] = str(r, "name")
	}

	group := func(table, key string) map[string][]row {
		out := map[string][]row{}
		for _, r := range db[table] {
			k := normID(str(r, key))
			out[k] = append(out[k], r)
		}
		return out
	}
	ings, steps := group("recipes_ingredients", "recipe_id"), group("recipe_instructions", "recipe_id")
	notes, nutr := group("notes", "recipe_id"), group("recipe_nutrition", "recipe_id")
	settings := group("recipe_settings", "recipe_id")
	links := func(table, key string, names map[string]row) map[string][]string {
		out := map[string][]string{}
		for _, l := range db[table] {
			rid := normID(str(l, "recipe_id"))
			out[rid] = append(out[rid], normID(str(l, key)))
		}
		return out
	}
	rCats, rTags, rTools := links("recipes_to_categories", "category_id", cats), links("recipes_to_tags", "tag_id", tags), links("recipes_to_tools", "tool_id", tools)
	favs, made := map[string]bool{}, map[string]int64{}
	ratings := map[string]float64{}
	for _, l := range db["users_to_recipes"] {
		rid := normID(str(l, "recipe_id"))
		if truthy(l, "is_favorite") {
			favs[rid] = true
		}
		if v := num(l, "rating"); v > ratings[rid] {
			ratings[rid] = v
		}
	}
	for _, l := range db["households_to_recipes"] {
		if t := ts(str(l, "last_made")); t > made[normID(str(l, "recipe_id"))] {
			made[normID(str(l, "recipe_id"))] = t
		}
	}
	byPos := func(rows []row) {
		sort.SliceStable(rows, func(i, j int) bool { return num(rows[i], "position") < num(rows[j], "position") })
	}

	recipes := db["recipes"]
	total := len(recipes)
	for i, m := range recipes {
		id := normID(str(m, "id"))
		r := base(m)
		if r.Title == "" {
			continue
		}
		for _, c := range rCats[id] {
			r.Tags = append(r.Tags, str(cats[c], "name"))
		}
		for _, t := range rTags[id] {
			r.Tags = append(r.Tags, str(tags[t], "name"))
		}
		for _, t := range rTools[id] {
			r.Tags = append(r.Tags, str(tools[t], "name"))
		}
		// Before mid-2025 Mealie stored unparsed ingredients with quantity 1
		// and hid it behind this per-recipe flag.
		legacyNoAmount := len(settings[id]) > 0 && truthy(settings[id][0], "disable_amount")
		byPos(ings[id])
		for _, ing := range ings[id] {
			if title := str(ing, "title"); title != "" {
				r.Ingredients = append(r.Ingredients, model.Line{Text: title, H: true})
			}
			qty := num(ing, "quantity")
			unit, food := units[normID(str(ing, "unit_id"))], foods[normID(str(ing, "food_id"))]
			if legacyNoAmount && qty == 1 && unit == nil && food == nil {
				qty = 0
			}
			text := ingredientText(qty, unit, food, str(ing, "note"), str(ing, "original_text"), recipeNames[normID(str(ing, "referenced_recipe_id"))])
			if text != "" {
				r.Ingredients = append(r.Ingredients, model.Line{Text: text})
			}
		}
		byPos(steps[id])
		for _, st := range steps[id] {
			if title := str(st, "title"); title != "" {
				r.Steps = append(r.Steps, model.Line{Text: title, H: true})
			}
			if text := str(st, "text"); text != "" {
				r.Steps = append(r.Steps, model.Line{Text: text})
			}
		}
		r.Notes = joinNotes(notes[id])
		if len(nutr[id]) > 0 {
			r.Nutrition = nutrition(nutr[id][0])
		}
		if ratings[id] > r.Rating {
			r.Rating = ratings[id]
		}
		if t := made[id]; t > r.LastCooked {
			r.LastCooked, r.CookedCount = t, 1
		}
		item := Item{MealieID: id, Recipe: r, Favorite: favs[id]}
		for _, dir := range []string{dashed(id), id} {
			if item.Image = a.image(prefix + "data/recipes/" + dir); item.Image != nil {
				break
			}
		}
		sink.Progress(i, total, r.Title)
		if err := sink.Recipe(item); err != nil {
			return err
		}
	}
	sink.Progress(total, total, "")

	// Cookbooks in Mealie are saved filters; resolve them to recipe lists.
	cbLinks := func(table, key string) map[string][]string {
		out := map[string][]string{}
		for _, l := range db[table] {
			out[normID(str(l, "cookbook_id"))] = append(out[normID(str(l, "cookbook_id"))], normID(str(l, key)))
		}
		return out
	}
	cbCats, cbTags, cbTools := cbLinks("cookbooks_to_categories", "category_id"), cbLinks("cookbooks_to_tags", "tag_id"), cbLinks("cookbooks_to_tools", "tool_id")
	for _, cb := range db["cookbooks"] {
		cid := normID(str(cb, "id"))
		book := Book{Name: str(cb, "name"), Description: str(cb, "description")}
		filter := str(cb, "query_filter_string")
		for _, m := range recipes {
			id := normID(str(m, "id"))
			sets := map[string][]string{"recipe_category": rCats[id], "tags": rTags[id], "tools": rTools[id]}
			var in bool
			if filter != "" {
				in = evalFilter(filter, sets)
			} else {
				c, t, o := cbCats[cid], cbTags[cid], cbTools[cid]
				if len(c)+len(t)+len(o) == 0 {
					continue
				}
				in = matchSet(sets["recipe_category"], c, truthy(cb, "require_all_categories")) &&
					matchSet(sets["tags"], t, truthy(cb, "require_all_tags")) && matchSet(sets["tools"], o, truthy(cb, "require_all_tools"))
			}
			if in {
				book.RecipeIDs = append(book.RecipeIDs, id)
			}
		}
		if book.Name != "" {
			sink.Cookbook(book)
		}
	}
	return nil
}

func dashed(id string) string {
	if len(id) != 32 {
		return id
	}
	return id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}

func matchSet(have, want []string, all bool) bool {
	if len(want) == 0 {
		return true
	}
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	hits := 0
	for _, w := range want {
		if set[w] {
			hits++
		}
	}
	if all {
		return hits == len(want)
	}
	return hits > 0
}

var (
	reClause = regexp.MustCompile(`(?i)^\s*([\w.]+)\s+(NOT CONTAINS ALL|CONTAINS ALL|NOT IN|IN|=|<>|!=)\s+(.*)$`)
	reValue  = regexp.MustCompile(`[0-9a-fA-F-]{32,36}`)
	reOr     = regexp.MustCompile(`(?i)\s+OR\s+`)
	reAnd    = regexp.MustCompile(`(?i)\s+AND\s+`)
)

// evalFilter evaluates the subset of Mealie's query-filter language that its
// cookbook editor produces: clauses on recipe_category.id / tags.id /
// tools.id joined by AND / OR. Unknown clauses are treated as true.
func evalFilter(filter string, sets map[string][]string) bool {
	filter = strings.NewReplacer("(", " ", ")", " ").Replace(filter)
	for _, group := range reOr.Split(filter, -1) {
		ok, any := true, false
		for _, clause := range reAnd.Split(group, -1) {
			m := reClause.FindStringSubmatch(clause)
			if m == nil {
				continue
			}
			field := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(m[1], ".id"), ".ID"))
			have, known := sets[field]
			if !known {
				continue
			}
			any = true
			var want []string
			for _, v := range reValue.FindAllString(m[3], -1) {
				want = append(want, normID(v))
			}
			op := strings.ToUpper(strings.Join(strings.Fields(m[2]), " "))
			res := matchSet(have, want, op == "CONTAINS ALL" || op == "NOT CONTAINS ALL" || op == "=")
			if strings.HasPrefix(op, "NOT") || op == "<>" || op == "!=" {
				res = !res
			}
			ok = ok && res
		}
		if ok && any {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------------ live API import

type api struct {
	f     *scrape.Fetcher
	base  string
	token string
	ctx   context.Context
}

func (a *api) get(p string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, a.base+p, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/json, image/*")
	res, err := a.f.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return nil, errors.New("Mealie rejected that API token")
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("Mealie answered HTTP %d for %s", res.StatusCode, p)
	}
	return io.ReadAll(io.LimitReader(res.Body, limit))
}

func (a *api) json(p string) (row, error) {
	b, err := a.get(p, 64<<20)
	if err != nil {
		return nil, err
	}
	var m row
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, errors.New("that address didn't answer like a Mealie server")
	}
	return m, nil
}

// ImportAPI pulls every recipe from a running Mealie server.
func ImportAPI(ctx context.Context, f *scrape.Fetcher, baseURL, token string, sink Sink) error {
	u, err := scrape.CleanURL(baseURL)
	if err != nil {
		return err
	}
	a := &api{f: f, base: strings.TrimRight(u.Scheme+"://"+u.Host+strings.TrimRight(u.Path, "/"), "/"), token: strings.TrimSpace(token), ctx: ctx}
	a.base = strings.TrimSuffix(a.base, "/api")
	var slugs []row
	for page := 1; page < 2000; page++ {
		m, err := a.json(fmt.Sprintf("/api/recipes?page=%d&perPage=100&orderBy=created_at&orderDirection=asc", page))
		if err != nil {
			return err
		}
		items := list(m, "items")
		slugs = append(slugs, items...)
		if len(items) == 0 || float64(page) >= num(m, "total_pages", "totalPages") {
			break
		}
	}
	favs := map[string]bool{}
	if m, err := a.json("/api/users/self/favorites"); err == nil {
		for _, r := range list(m, "ratings") {
			if truthy(r, "isFavorite", "is_favorite") {
				favs[normID(str(r, "recipeId", "recipe_id"))] = true
			}
		}
	}
	for i, s := range slugs {
		slug := str(s, "slug")
		sink.Progress(i, len(slugs), str(s, "name"))
		m, err := a.json("/api/recipes/" + url.PathEscape(slug))
		if err != nil {
			continue
		}
		id := str(m, "id")
		item := Item{MealieID: normID(id), Recipe: fromObject(m), Favorite: favs[normID(id)]}
		if str(m, "image") != "" {
			if img, err := a.get("/api/media/recipes/"+id+"/images/original.webp", 60<<20); err == nil {
				item.Image = img
			}
		}
		if err := sink.Recipe(item); err != nil {
			return err
		}
	}
	sink.Progress(len(slugs), len(slugs), "")
	for _, p := range []string{"/api/households/cookbooks?perPage=-1", "/api/groups/cookbooks?perPage=-1"} {
		m, err := a.json(p)
		if err != nil {
			continue
		}
		for _, cb := range list(m, "items") {
			book := Book{Name: str(cb, "name"), Description: str(cb, "description")}
			if rm, err := a.json("/api/recipes?perPage=-1&cookbook=" + url.QueryEscape(str(cb, "id"))); err == nil {
				for _, r := range list(rm, "items") {
					book.RecipeIDs = append(book.RecipeIDs, normID(str(r, "id")))
				}
			}
			if book.Name != "" {
				sink.Cookbook(book)
			}
		}
		break
	}
	return nil
}
