package migrate

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/au12/fond/internal/mealie"
	"github.com/au12/fond/internal/model"
	"github.com/au12/fond/internal/scrape"
)

// Recipe Sage exports schema.org JSON-LD: a list of Recipe objects, with photos as links.

func importRecipeSage(file string, sink Sink) error {
	var nodes []obj
	if zr, err := zip.OpenReader(file); err == nil {
		defer zr.Close()
		for _, f := range zr.File {
			if !skipName(f.Name) && strings.HasSuffix(strings.ToLower(f.Name), ".json") {
				nodes = append(nodes, ldNodes(readAll(f))...)
			}
		}
	} else {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		nodes = ldNodes(b)
	}
	if len(nodes) == 0 {
		return errors.New("no recipes were found in this Recipe Sage export")
	}
	for i, n := range nodes {
		rec, imgURL := fromRecipeSage(n)
		if rec.Title == "" {
			continue
		}
		sink.Progress(i, len(nodes), rec.Title)
		origin := "recipesage:" + str(n, "identifier")
		if origin == "recipesage:" {
			origin = "recipesage:" + model.Slugify(rec.Title) + ":" + str(n, "datePublished")
		}
		if err := sink.Recipe(mealie.Item{Recipe: rec, Origin: origin, ImageURL: imgURL}); err != nil {
			return err
		}
	}
	sink.Progress(len(nodes), len(nodes), "")
	return nil
}

func ldNodes(b []byte) []obj {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	var out []obj
	var walk func(any, int)
	walk = func(x any, depth int) {
		if depth > 3 {
			return
		}
		switch t := x.(type) {
		case []any:
			for _, y := range t {
				walk(y, depth+1)
			}
		case obj:
			if isLDRecipe(t) {
				out = append(out, t)
			} else if g, ok := t["@graph"]; ok {
				walk(g, depth+1)
			}
		}
	}
	walk(v, 0)
	return out
}

func fromRecipeSage(n obj) (model.Recipe, string) {
	rp, img := scrape.FromLD(n)
	r := *rp
	// The source is kept as isBasedOn (a link) and creditText (a name).
	if src := str(n, "isBasedOn"); strings.HasPrefix(src, "http") {
		r.SourceURL = src
	}
	if c := str(n, "creditText"); c != "" && !strings.HasPrefix(c, "http") {
		r.SourceName = c
	}
	var notes []string
	for _, c := range objs(n["comment"]) {
		if t := strings.TrimSpace(str(c, "text")); t != "" {
			notes = append(notes, t)
		}
	}
	r.Notes = strings.Join(notes, "\n\n")
	if rating, ok := n["aggregateRating"].(obj); ok {
		if v := num(rating, "ratingValue"); v > 0 && v <= 5 {
			r.Rating = v
		}
	}
	if t, err := time.Parse(time.RFC3339, str(n, "datePublished")); err == nil {
		r.CreatedAt = t.Unix()
	}
	// Recipe Sage labels arrive as recipeCategory; keep more of them than a web page would get.
	if cats, ok := n["recipeCategory"].([]any); ok {
		var names []string
		for _, c := range cats {
			if s, ok := c.(string); ok {
				names = append(names, s)
			}
		}
		r.Tags = tags(names)
	}
	if !strings.HasPrefix(img, "http") {
		img = ""
	}
	return r, img
}
