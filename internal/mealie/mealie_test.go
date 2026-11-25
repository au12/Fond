package mealie

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/au12/fond/internal/scrape"
)

type countSink struct {
	recipes, withIng, withSteps, books, bookRecipes, images, favs int
	sample                                                        []Item
}

func (c *countSink) Recipe(it Item) error {
	it.Recipe.Normalize()
	c.recipes++
	if len(it.Recipe.Ingredients) > 0 {
		c.withIng++
	}
	if len(it.Recipe.Steps) > 0 {
		c.withSteps++
	}
	if it.Image != nil {
		c.images++
	}
	if it.Favorite {
		c.favs++
	}
	if len(c.sample) < 2 {
		c.sample = append(c.sample, it)
	}
	return nil
}
func (c *countSink) Cookbook(b Book)           { c.books++; c.bookRecipes += len(b.RecipeIDs) }
func (c *countSink) Progress(int, int, string) {}

// TestBackups runs the importer over Mealie's own backup fixtures.
// FOND_MEALIE_FIXTURES=/path/to/mealie/tests/data/backups go test ./internal/mealie -v
func TestBackups(t *testing.T) {
	dir := os.Getenv("FOND_MEALIE_FIXTURES")
	if dir == "" {
		t.Skip("FOND_MEALIE_FIXTURES not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.zip"))
	for _, f := range files {
		s := &countSink{}
		if err := ImportZip(f, s); err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		t.Logf("%-40s recipes %3d  ingredients %3d  steps %3d  images %d  favourites %d  cookbooks %d (%d recipes)",
			filepath.Base(f), s.recipes, s.withIng, s.withSteps, s.images, s.favs, s.books, s.bookRecipes)
	}
}

func TestQuantity(t *testing.T) {
	for in, want := range map[float64]string{0.5: "1/2", 1.5: "1 1/2", 2: "2", 0.333: "1/3", 0.125: "1/8", 2.75: "2 3/4", 0.41: "0.41", 0: ""} {
		if got := Quantity(in, true); got != want {
			t.Errorf("Quantity(%v) = %q, want %q", in, got, want)
		}
	}
	if got := Quantity(1.5, false); got != "1.5" {
		t.Errorf("decimal: %q", got)
	}
}

func TestIngredientText(t *testing.T) {
	cup := row{"name": "cup", "plural_name": "cups", "fraction": true}
	flour := row{"name": "flour"}
	egg := row{"name": "egg", "plural_name": "eggs"}
	cases := []struct {
		qty        float64
		unit, food row
		note, want string
	}{
		{2.5, cup, flour, "sifted", "2 1/2 cups flour, sifted"},
		{1, cup, flour, "", "1 cup flour"},
		{3, nil, egg, "", "3 eggs"},
		{1, nil, egg, "beaten", "1 egg, beaten"},
		{0, nil, nil, "2 cups sugar", "2 cups sugar"},
		{2, nil, nil, "large onions", "2 large onions"},
	}
	for _, c := range cases {
		if got := ingredientText(c.qty, c.unit, c.food, c.note, "", ""); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestFilter(t *testing.T) {
	sets := map[string][]string{"recipe_category": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, "tags": {"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccccccccccccccc"}}
	cases := map[string]bool{
		`recipe_category.id IN ["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"]`:                                                         true,
		`recipe_category.id IN ["dddddddd-dddd-dddd-dddd-dddddddddddd"]`:                                                         false,
		`tags.id CONTAINS ALL ["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","cccccccc-cccc-cccc-cccc-cccccccccccc"]`:                   true,
		`tags.id CONTAINS ALL ["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","dddddddd-dddd-dddd-dddd-dddddddddddd"]`:                   false,
		`recipe_category.id IN ["dddddddd-dddd-dddd-dddd-dddddddddddd"] OR tags.id IN ["cccccccc-cccc-cccc-cccc-cccccccccccc"]`:  true,
		`recipe_category.id IN ["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"] AND tags.id IN ["dddddddd-dddd-dddd-dddd-dddddddddddd"]`: false,
	}
	for f, want := range cases {
		if got := evalFilter(f, sets); got != want {
			t.Errorf("%s = %v, want %v", f, got, want)
		}
	}
}

// TestExportZip covers the "Data Management" recipe export: one JSON document
// per recipe (snake_case, nested objects) next to its images folder.
func TestExportZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("recipes/apple-pie/apple-pie.json")
	w.Write([]byte(`{"id":"6f2a5a0e-1111-2222-3333-444455556666","name":"Apple Pie","slug":"apple-pie","recipe_servings":8,"total_time":"1 hour 30 minutes",
	 "org_url":"https://www.example.com/pie","rating":4,"recipe_category":[{"name":"Dessert"}],"tags":[{"name":"Autumn"}],"tools":[{"name":"Rolling pin"}],
	 "recipe_ingredient":[{"title":"Filling","quantity":6,"unit":null,"food":{"name":"apple","plural_name":"apples"},"note":"peeled","display":""},
	   {"quantity":1.5,"unit":{"name":"cup","plural_name":"cups","fraction":true},"food":{"name":"sugar"},"note":"","display":"1 ½ cups sugar"},
	   {"quantity":0,"unit":null,"food":null,"note":"pinch of salt"}],
	 "recipe_instructions":[{"title":"","text":"Mix."},{"title":"Bake","text":"Bake for 50 minutes."}],
	 "notes":[{"title":"Tip","text":"Use tart apples."}],"nutrition":{"calories":"320"}}`))
	img, _ := zw.Create("recipes/apple-pie/images/original.webp")
	img.Write([]byte("not-really-an-image"))
	zw.Close()
	f.Close()

	s := &countSink{}
	if err := ImportZip(path, s); err != nil {
		t.Fatal(err)
	}
	if s.recipes != 1 || s.images != 1 {
		t.Fatalf("recipes %d images %d", s.recipes, s.images)
	}
	r := s.sample[0].Recipe
	want := []string{"Filling", "6 apples, peeled", "1 ½ cups sugar", "pinch of salt"}
	for i, w := range want {
		if r.Ingredients[i].Text != w {
			t.Errorf("ingredient %d = %q, want %q", i, r.Ingredients[i].Text, w)
		}
	}
	if !r.Ingredients[0].H || len(r.Steps) != 3 || !r.Steps[1].H || r.TotalMin != 90 || r.Servings != 8 || r.SourceName != "example.com" || r.Rating != 4 {
		t.Errorf("mapped wrong: %+v", r)
	}
	if len(r.Tags) != 3 || r.Notes != "**Tip**\nUse tart apples." || r.Nutrition["calories"] != "320" {
		t.Errorf("tags/notes/nutrition: %v %q %v", r.Tags, r.Notes, r.Nutrition)
	}
}

// TestAPI runs the live-server importer against a stand-in Mealie API.
func TestAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/recipes", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(401)
			return
		}
		if r.URL.Query().Get("cookbook") != "" {
			w.Write([]byte(`{"items":[{"id":"aaaaaaaa-0000-0000-0000-000000000001"}]}`))
			return
		}
		w.Write([]byte(`{"page":1,"totalPages":1,"items":[{"slug":"toast","name":"Toast"},{"slug":"soup","name":"Soup"}]}`))
	})
	mux.HandleFunc("/api/recipes/toast", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"aaaaaaaa-0000-0000-0000-000000000001","name":"Toast","image":"x","recipeServings":2,"orgURL":"https://ex.com/t",
		 "recipeIngredient":[{"quantity":2,"unit":{"name":"slice","pluralName":"slices"},"food":{"name":"bread"},"note":""}],"recipeInstructions":[{"text":"Toast it."}]}`))
	})
	mux.HandleFunc("/api/recipes/soup", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"aaaaaaaa-0000-0000-0000-000000000002","name":"Soup","recipeIngredient":[{"note":"1 onion","display":"1 onion"}],"recipeInstructions":[{"text":"Simmer."}]}`))
	})
	mux.HandleFunc("/api/media/recipes/aaaaaaaa-0000-0000-0000-000000000001/images/original.webp", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("img")) })
	mux.HandleFunc("/api/users/self/favorites", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ratings":[{"recipeId":"aaaaaaaa-0000-0000-0000-000000000002","isFavorite":true}]}`))
	})
	mux.HandleFunc("/api/households/cookbooks", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"id":"cb1","name":"Breakfast","description":""}]}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	s := &countSink{}
	if err := ImportAPI(t.Context(), scrape.NewFetcher(true), ts.URL+"/", "secret", s); err != nil {
		t.Fatal(err)
	}
	if s.recipes != 2 || s.images != 1 || s.favs != 1 || s.books != 1 || s.bookRecipes != 1 {
		t.Errorf("recipes %d images %d favourites %d cookbooks %d (%d)", s.recipes, s.images, s.favs, s.books, s.bookRecipes)
	}
	if got := s.sample[0].Recipe.Ingredients[0].Text; got != "2 slices bread" {
		t.Errorf("ingredient = %q", got)
	}
	if err := ImportAPI(t.Context(), scrape.NewFetcher(true), ts.URL, "wrong", &countSink{}); err == nil {
		t.Error("a bad token should fail")
	}
}
