package scrape

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var reNorm = regexp.MustCompile(`[^a-z0-9]+`)

func norm(s string) string { return reNorm.ReplaceAllString(strings.ToLower(s), "") }

// TestCorpus measures extraction against the saved real-world pages from the
// recipe-scrapers project (https://github.com/hhursev/recipe-scrapers).
// Run with: FOND_CORPUS=/path/to/recipe-scrapers/tests/test_data go test ./internal/scrape -run Corpus -v
func TestCorpus(t *testing.T) {
	dir := os.Getenv("FOND_CORPUS")
	if dir == "" {
		t.Skip("FOND_CORPUS not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.testhtml"))
	sort.Strings(files)
	var total, okIng, okSteps, okBoth, okTitle, noRecipe int
	methods := map[string]int{}
	var fails []string
	for _, f := range files {
		raw, err := os.ReadFile(strings.TrimSuffix(f, ".testhtml") + ".json")
		if err != nil {
			continue
		}
		var want struct {
			Title        string   `json:"title"`
			Ingredients  []string `json:"ingredients"`
			Instructions []string `json:"instructions_list"`
			Host         string   `json:"host"`
		}
		if json.Unmarshal(raw, &want) != nil || len(want.Ingredients) == 0 {
			continue
		}
		page, _ := os.ReadFile(f)
		total++
		res, err := Extract(page, "https://"+want.Host+"/recipe")
		if err != nil {
			noRecipe++
			fails = append(fails, fmt.Sprintf("NONE   %s", strings.TrimPrefix(f, dir)))
			continue
		}
		methods[res.Method]++
		got := map[string]bool{}
		for _, l := range res.Recipe.Ingredients {
			got[norm(l.Text)] = true
		}
		hit := 0
		for _, w := range want.Ingredients {
			if got[norm(w)] {
				hit++
			}
		}
		ingOK := float64(hit) >= 0.8*float64(len(want.Ingredients))
		if !ingOK && float64(len(res.Recipe.Ingredients)) >= 0.75*float64(len(want.Ingredients)) && float64(len(res.Recipe.Ingredients)) <= 1.4*float64(len(want.Ingredients))+1 {
			// Same list, formatted differently (the reference data is often
			// built from on-page markup, not the site's own JSON-LD).
			var all strings.Builder
			for _, l := range res.Recipe.Ingredients {
				all.WriteString(" " + strings.ToLower(l.Text))
			}
			words, found := 0, 0
			for _, w := range want.Ingredients {
				for _, tok := range strings.FieldsFunc(strings.ToLower(w), func(r rune) bool { return r < 'a' || r > 'z' && r < 128 }) {
					if len(tok) < 4 {
						continue
					}
					words++
					if strings.Contains(all.String(), tok) {
						found++
					}
				}
			}
			ingOK = words > 0 && float64(found) >= 0.8*float64(words)
		}
		stepsOK := len(want.Instructions) == 0 || len(res.Recipe.Steps) > 0
		titleOK := want.Title == "" || strings.Contains(norm(res.Recipe.Title), norm(want.Title)) || strings.Contains(norm(want.Title), norm(res.Recipe.Title)) && res.Recipe.Title != ""
		if ingOK {
			okIng++
		}
		if stepsOK {
			okSteps++
		}
		if titleOK {
			okTitle++
		}
		if ingOK && stepsOK {
			okBoth++
		} else {
			fails = append(fails, fmt.Sprintf("%-6s %s ing %d/%d (got %d) steps %d/%d [%s]", "PART", strings.TrimPrefix(f, dir), hit, len(want.Ingredients), len(res.Recipe.Ingredients), len(res.Recipe.Steps), len(want.Instructions), res.Method))
		}
	}
	pct := func(n int) string { return fmt.Sprintf("%d (%.1f%%)", n, 100*float64(n)/float64(total)) }
	t.Logf("pages %d | ingredients+steps %s | ingredients %s | steps %s | title %s | nothing found %s", total, pct(okBoth), pct(okIng), pct(okSteps), pct(okTitle), pct(noRecipe))
	t.Logf("methods: %v", methods)
	if out := os.Getenv("FOND_CORPUS_FAILS"); out != "" {
		os.WriteFile(out, []byte(strings.Join(fails, "\n")), 0o644)
	}
}
