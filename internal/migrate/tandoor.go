package migrate

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/au12/fond/internal/mealie"
	"github.com/au12/fond/internal/model"
)

// Tandoor exports a zip of zips: one per recipe, each holding recipe.json and the photo.

type tandoorEntry struct {
	name  string
	json  []byte
	image []byte
}

func importTandoor(file string, sink Sink) error {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return errors.New("that file isn't a readable zip archive")
	}
	defer zr.Close()
	var entries []tandoorEntry
	collectTandoor(&zr.Reader, "", &entries, 0)
	if len(entries) == 0 {
		return errors.New("no Tandoor recipes were found in this zip")
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	for i, e := range entries {
		var m obj
		if json.Unmarshal(e.json, &m) != nil {
			continue
		}
		rec := fromTandoor(m)
		if rec.Title == "" {
			continue
		}
		sink.Progress(i, len(entries), rec.Title)
		if err := sink.Recipe(mealie.Item{Recipe: rec, Image: e.image, Origin: "tandoor:" + e.name}); err != nil {
			return err
		}
	}
	sink.Progress(len(entries), len(entries), "")
	return nil
}

// collectTandoor gathers recipe.json files, grouped with the image beside them, from a zip and the zips inside it.
func collectTandoor(zr *zip.Reader, prefix string, out *[]tandoorEntry, depth int) {
	dirs := map[string]*tandoorEntry{}
	var order []string
	for _, f := range zr.File {
		if skipName(f.Name) {
			continue
		}
		name := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		base := strings.ToLower(path.Base(name))
		dir := path.Dir(name)
		switch {
		case strings.HasSuffix(base, ".zip") && depth < 2:
			b := readAll(f)
			if inner, err := zip.NewReader(bytes.NewReader(b), int64(len(b))); err == nil {
				collectTandoor(inner, prefix+strings.TrimSuffix(path.Base(name), path.Ext(name))+"/", out, depth+1)
			}
		case base == "recipe.json" || strings.HasPrefix(base, "image."):
			e := dirs[dir]
			if e == nil {
				e = &tandoorEntry{name: strings.Trim(prefix+strings.TrimPrefix(dir, "."), "/")}
				dirs[dir] = e
				order = append(order, dir)
			}
			if base == "recipe.json" {
				e.json = readAll(f)
			} else if e.image == nil {
				e.image = readAll(f)
			}
		}
	}
	for _, d := range order {
		if e := dirs[d]; e.json != nil {
			if e.name == "" {
				e.name = "recipe"
			}
			*out = append(*out, *e)
		}
	}
}

func tandoorAmount(a float64) string {
	if a <= 0 {
		return ""
	}
	// Tandoor keeps amounts as decimals; show the usual fractions as fractions.
	return mealie.Quantity(math.Round(a*1000)/1000, true)
}

func tandoorIngredient(m obj) (model.Line, bool) {
	food, _ := m["food"].(obj)
	unit, _ := m["unit"].(obj)
	foodName, unitName, note := "", "", strings.TrimSpace(str(m, "note"))
	if food != nil {
		foodName = str(food, "name")
		if num(m, "amount") > 1 && str(food, "plural_name") != "" && truthy(m["always_use_plural_food"]) {
			foodName = str(food, "plural_name")
		}
	}
	if unit != nil {
		unitName = str(unit, "name")
		if num(m, "amount") > 1 && str(unit, "plural_name") != "" {
			unitName = str(unit, "plural_name")
		}
	}
	if truthy(m["is_header"]) {
		t := strings.TrimSpace(strings.Join(nonEmpty(note, foodName), " "))
		return model.Line{Text: strings.TrimSuffix(t, ":"), H: true}, t != ""
	}
	if orig := str(m, "original_text"); orig != "" && foodName == "" {
		return model.Line{Text: orig}, true
	}
	var parts []string
	if !truthy(m["no_amount"]) {
		parts = nonEmpty(tandoorAmount(num(m, "amount")), unitName)
	}
	parts = append(parts, nonEmpty(foodName)...)
	t := strings.Join(parts, " ")
	if note != "" {
		if t == "" {
			t = note
		} else {
			t += ", " + note
		}
	}
	return model.Line{Text: model.CleanLine(t)}, strings.TrimSpace(t) != ""
}

func fromTandoor(m obj) model.Recipe {
	r := model.Recipe{
		Title:       str(m, "name"),
		Description: str(m, "description"),
		SourceURL:   str(m, "source_url"),
		PrepMin:     int(num(m, "working_time")),
		CookMin:     int(num(m, "waiting_time")),
		Servings:    num(m, "servings"),
	}
	r.TotalMin = r.PrepMin + r.CookMin
	if r.Servings > 0 {
		if st := str(m, "servings_text"); st != "" {
			r.Yield = mealie.Quantity(r.Servings, false) + " " + st
		} else {
			r.Yield = mealie.Quantity(r.Servings, false) + " servings"
		}
	}
	var kw []string
	for _, k := range objs(m["keywords"]) {
		kw = append(kw, str(k, "name", "label"))
	}
	r.Tags = tags(kw)
	steps := objs(m["steps"])
	sort.SliceStable(steps, func(i, j int) bool { return num(steps[i], "order") < num(steps[j], "order") })
	named := 0
	for _, s := range steps {
		if str(s, "name") != "" {
			named++
		}
	}
	for _, s := range steps {
		ings := objs(s["ingredients"])
		sort.SliceStable(ings, func(i, j int) bool { return num(ings[i], "order") < num(ings[j], "order") })
		// A recipe in named parts keeps the parts as headings in both lists.
		if name := str(s, "name"); name != "" && len(steps) > 1 && named > 1 {
			if len(ings) > 0 {
				r.Ingredients = append(r.Ingredients, model.Line{Text: name, H: true})
			}
			r.Steps = append(r.Steps, model.Line{Text: name, H: true})
		}
		for _, ing := range ings {
			if l, ok := tandoorIngredient(ing); ok {
				r.Ingredients = append(r.Ingredients, l)
			}
		}
		text := reTemplate.ReplaceAllString(str(s, "instruction"), "")
		if truthy(s["show_as_header"]) && strings.TrimSpace(text) == "" {
			continue
		}
		r.Steps = append(r.Steps, lines(text, true)...)
	}
	if nut, ok := m["nutrition"].(obj); ok {
		r.Nutrition = map[string]string{}
		for k, mine := range map[string]string{"calories": "calories", "carbohydrates": "carbs", "fats": "fat", "proteins": "protein"} {
			if v := num(nut, k); v > 0 {
				r.Nutrition[mine] = mealie.Quantity(v, false)
			}
		}
	}
	return r
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t == "true" || t == "1"
	}
	return false
}

func nonEmpty(xs ...string) []string {
	out := []string{}
	for _, x := range xs {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}
