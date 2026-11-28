// Package migrate reads the exports of other recipe apps: Tandoor, Paprika and Recipe Sage.
// Mealie has a package of its own, and Fond's own backups are restored by the server.
package migrate

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/au12/fond/internal/mealie"
	"github.com/au12/fond/internal/model"
	"github.com/au12/fond/internal/scrape"
)

type Sink = mealie.Sink

// Kind names an export format.
type Kind string

const (
	Unknown    Kind = ""
	Fond       Kind = "fond"
	Mealie     Kind = "mealie"
	Tandoor    Kind = "tandoor"
	Paprika    Kind = "paprika"
	RecipeSage Kind = "recipesage"
)

// Names are how each app is named to the person importing.
var Names = map[Kind]string{Fond: "Fond backup", Mealie: "Mealie", Tandoor: "Tandoor", Paprika: "Paprika", RecipeSage: "Recipe Sage"}

const maxEntry = 64 << 20

// Detect looks inside a file and says which app made it.
func Detect(file string) Kind {
	f, err := os.Open(file)
	if err != nil {
		return Unknown
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	f.Close()
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte{0x1f, 0x8b}):
		return Paprika // a single .paprikarecipe is gzipped JSON
	case bytes.HasPrefix(head, []byte("PK")):
		return detectZip(file)
	}
	t := bytes.TrimLeft(head, " \t\r\n\xef\xbb\xbf")
	if len(t) > 0 && (t[0] == '[' || t[0] == '{') {
		b, err := os.ReadFile(file)
		if err != nil {
			return Unknown
		}
		return detectJSON(b)
	}
	return Unknown
}

func detectJSON(b []byte) Kind {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return Unknown
	}
	if m, ok := v.(map[string]any); ok {
		if _, ok := m["recipes"]; ok {
			if _, ok := m["exportedAt"]; ok {
				return Fond
			}
		}
		if isLDRecipe(m) || m["@graph"] != nil {
			return RecipeSage
		}
	}
	if list, ok := v.([]any); ok {
		for _, x := range list {
			if m, ok := x.(map[string]any); ok && isLDRecipe(m) {
				return RecipeSage
			}
		}
	}
	return Unknown
}

func isLDRecipe(m map[string]any) bool {
	switch t := m["@type"].(type) {
	case string:
		return strings.EqualFold(t, "Recipe")
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok && strings.EqualFold(s, "Recipe") {
				return true
			}
		}
	}
	return false
}

func detectZip(file string) Kind {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return Unknown
	}
	defer zr.Close()
	var jsonFiles []*zip.File
	for _, f := range zr.File {
		name := strings.ToLower(path.Clean(strings.ReplaceAll(f.Name, "\\", "/")))
		base := path.Base(name)
		switch {
		case strings.Contains(name, "__macosx"):
		case base == "fond.json":
			return Fond
		case strings.HasSuffix(base, ".paprikarecipe"):
			return Paprika
		case base == "database.json", base == "mealie.db":
			return Mealie
		case base == "recipe.json":
			return Tandoor
		case strings.HasSuffix(base, ".zip"):
			// Tandoor's export is a zip of zips, one per recipe.
			if rc, err := f.Open(); err == nil {
				b, _ := io.ReadAll(io.LimitReader(rc, maxEntry))
				rc.Close()
				if inner, err := zip.NewReader(bytes.NewReader(b), int64(len(b))); err == nil {
					for _, g := range inner.File {
						if path.Base(strings.ToLower(g.Name)) == "recipe.json" {
							return Tandoor
						}
					}
				}
			}
		case strings.HasSuffix(base, ".json"):
			// Mealie recipe exports keep recipes/<slug>/<slug>.json.
			if path.Base(path.Dir(name))+".json" == base {
				return Mealie
			}
			jsonFiles = append(jsonFiles, f)
		}
	}
	for _, f := range jsonFiles {
		if rc, err := f.Open(); err == nil {
			b, _ := io.ReadAll(io.LimitReader(rc, maxEntry))
			rc.Close()
			if k := detectJSON(b); k == RecipeSage {
				return k
			}
		}
	}
	return Unknown
}

// Import reads a file of the given kind into the sink.
func Import(file string, kind Kind, sink Sink) error {
	switch kind {
	case Tandoor:
		return importTandoor(file, sink)
	case Paprika:
		return importPaprika(file, sink)
	case RecipeSage:
		return importRecipeSage(file, sink)
	}
	return errors.New("Fond doesn't know how to read this file")
}

// ------------------------------------------------------------------ helpers

type obj = map[string]any

func str(m obj, keys ...string) string {
	for _, k := range keys {
		switch t := m[k].(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				return s
			}
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
	}
	return ""
}

func num(m obj, keys ...string) float64 {
	for _, k := range keys {
		switch t := m[k].(type) {
		case float64:
			return t
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
				return f
			}
		}
	}
	return 0
}

func objs(v any) []obj {
	list, _ := v.([]any)
	out := make([]obj, 0, len(list))
	for _, x := range list {
		if m, ok := x.(obj); ok {
			out = append(out, m)
		}
	}
	return out
}

// lines splits a block of text into ingredient or step lines.
func lines(text string, steps bool) []model.Line {
	if steps {
		return scrape.StepLines([]string{text})
	}
	return scrape.IngredientLines([]string{text})
}

var reTemplate = regexp.MustCompile(`\{\{[^}]*\}\}`)

func tags(names []string) []string {
	out := []string{}
	for _, n := range names {
		for _, part := range strings.Split(n, ",") {
			if t := strings.TrimSpace(part); t != "" && len(t) <= 40 {
				out = append(out, t)
			}
		}
	}
	return model.CleanTags(out)
}

func readAll(f *zip.File) []byte {
	if f.UncompressedSize64 > maxEntry {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	b, _ := io.ReadAll(io.LimitReader(rc, maxEntry))
	return b
}

func gunzip(b []byte) []byte {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return b // some tools save .paprikarecipe files uncompressed
	}
	out, err := io.ReadAll(io.LimitReader(zr, maxEntry))
	if err != nil {
		return nil
	}
	return out
}

func skipName(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "__macosx") || strings.HasPrefix(path.Base(n), "._") || strings.HasSuffix(n, "/")
}
