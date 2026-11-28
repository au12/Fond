package migrate

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/au12/fond/internal/mealie"
	"github.com/au12/fond/internal/model"
	"github.com/au12/fond/internal/scrape"
)

// Paprika exports a .paprikarecipes file: a zip of .paprikarecipe entries, each gzipped JSON with the photo inside.

func importPaprika(file string, sink Sink) error {
	var blobs [][]byte
	if zr, err := zip.OpenReader(file); err == nil {
		defer zr.Close()
		files := append([]*zip.File{}, zr.File...)
		sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
		for _, f := range files {
			if !skipName(f.Name) && strings.HasSuffix(strings.ToLower(f.Name), ".paprikarecipe") {
				blobs = append(blobs, readAll(f))
			}
		}
	} else {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		blobs = append(blobs, b)
	}
	if len(blobs) == 0 {
		return errors.New("no Paprika recipes were found in this file")
	}
	for i, b := range blobs {
		var m obj
		if json.Unmarshal(gunzip(b), &m) != nil {
			continue
		}
		rec, img := fromPaprika(m)
		if rec.Title == "" {
			continue
		}
		sink.Progress(i, len(blobs), rec.Title)
		origin := "paprika:" + str(m, "uid")
		if origin == "paprika:" {
			origin = "paprika:" + model.Slugify(rec.Title) + ":" + path.Base(rec.SourceURL)
		}
		item := mealie.Item{Recipe: rec, Image: img, Origin: origin}
		if img == nil {
			item.ImageURL = str(m, "image_url")
		}
		if err := sink.Recipe(item); err != nil {
			return err
		}
	}
	sink.Progress(len(blobs), len(blobs), "")
	return nil
}

func fromPaprika(m obj) (model.Recipe, []byte) {
	r := model.Recipe{
		Title:       str(m, "name"),
		Description: scrape.CleanText(str(m, "description")),
		SourceURL:   str(m, "source_url"),
		SourceName:  str(m, "source"),
		Notes:       strings.TrimSpace(str(m, "notes")),
		Ingredients: lines(str(m, "ingredients"), false),
		Steps:       lines(str(m, "directions"), true),
		PrepMin:     scrape.ParseMinutes(str(m, "prep_time")),
		CookMin:     scrape.ParseMinutes(str(m, "cook_time")),
		TotalMin:    scrape.ParseMinutes(str(m, "total_time")),
		Rating:      num(m, "rating"),
	}
	if r.Rating > 5 || r.Rating < 0 {
		r.Rating = 0
	}
	if r.TotalMin == 0 {
		r.TotalMin = r.PrepMin + r.CookMin
	}
	if s := str(m, "servings"); s != "" {
		r.Yield, r.Servings = scrape.ParseYield(s)
	}
	if r.SourceName != "" && strings.HasPrefix(r.SourceName, "http") {
		if r.SourceURL == "" {
			r.SourceURL = r.SourceName
		}
		r.SourceName = ""
	}
	var cats []string
	if list, ok := m["categories"].([]any); ok {
		for _, c := range list {
			if s, ok := c.(string); ok {
				cats = append(cats, s)
			}
		}
	}
	r.Tags = tags(cats)
	if n := str(m, "nutritional_info"); n != "" {
		if r.Notes != "" {
			r.Notes += "\n\n"
		}
		r.Notes += "Nutrition: " + n
	}
	if t, err := time.Parse("2006-01-02 15:04:05", str(m, "created")); err == nil {
		r.CreatedAt = t.Unix()
	}
	var img []byte
	if d := str(m, "photo_data"); d != "" {
		img, _ = base64.StdEncoding.DecodeString(d)
	}
	if img == nil {
		for _, p := range objs(m["photos"]) {
			if b, err := base64.StdEncoding.DecodeString(str(p, "data")); err == nil && len(b) > 0 {
				img = b
				break
			}
		}
	}
	return r, img
}
