package migrate

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/au12/fond/internal/mealie"
)

type sink struct {
	items []mealie.Item
}

func (s *sink) Recipe(it mealie.Item) error { s.items = append(s.items, it); return nil }
func (s *sink) Cookbook(mealie.Book)        {}
func (s *sink) Progress(int, int, string)   {}

func writeZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	zw.Close()
	return buf.Bytes()
}

func save(t *testing.T, name string, data []byte) string {
	p := filepath.Join(t.TempDir(), name)
	os.WriteFile(p, data, 0o644)
	return p
}

func TestTandoor(t *testing.T) {
	recipe := map[string]any{
		"name": "Pancakes", "description": "Fluffy.", "servings": 4, "servings_text": "pancakes", "working_time": 10, "waiting_time": 15,
		"source_url": "https://example.com/pancakes", "keywords": []any{map[string]any{"name": "Breakfast"}},
		"steps": []any{
			map[string]any{"name": "Batter", "order": 0, "instruction": "Whisk {{ ingredients[0] }} together.", "ingredients": []any{
				map[string]any{"food": map[string]any{"name": "flour"}, "unit": map[string]any{"name": "cup", "plural_name": "cups"}, "amount": 1.5, "note": "sifted", "order": 0},
				map[string]any{"food": map[string]any{"name": "salt"}, "unit": nil, "amount": 0, "no_amount": true, "order": 1},
				map[string]any{"food": nil, "note": "For the top", "is_header": true, "order": 2},
				map[string]any{"food": map[string]any{"name": "eggs"}, "unit": nil, "amount": 2, "order": 3},
			}},
			map[string]any{"name": "Cook", "order": 1, "instruction": "Fry in butter.\nServe hot.", "ingredients": []any{}},
		},
		"nutrition": map[string]any{"calories": 320},
	}
	rj, _ := json.Marshal(recipe)
	inner := writeZip(t, map[string][]byte{"recipe.json": rj, "image.jpg": []byte("JPEG")})
	outer := writeZip(t, map[string][]byte{"12.zip": inner})
	file := save(t, "export.zip", outer)
	if k := Detect(file); k != Tandoor {
		t.Fatalf("detected %q", k)
	}
	var s sink
	if err := Import(file, Tandoor, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.items) != 1 {
		t.Fatalf("got %d recipes", len(s.items))
	}
	it := s.items[0]
	r := it.Recipe
	var ing, steps []string
	for _, l := range r.Ingredients {
		ing = append(ing, map[bool]string{true: "# ", false: ""}[l.H]+l.Text)
	}
	for _, l := range r.Steps {
		steps = append(steps, map[bool]string{true: "# ", false: ""}[l.H]+l.Text)
	}
	want := "# Batter|1 1/2 cups flour, sifted|salt|# For the top|2 eggs"
	if strings.Join(ing, "|") != want {
		t.Errorf("ingredients = %q", strings.Join(ing, "|"))
	}
	if got := strings.Join(steps, "|"); got != "# Batter|Whisk together.|# Cook|Fry in butter.|Serve hot." {
		t.Errorf("steps = %q", got)
	}
	if r.Title != "Pancakes" || r.Yield != "4 pancakes" || r.Servings != 4 || r.PrepMin != 10 || r.CookMin != 15 || r.TotalMin != 25 || r.SourceURL == "" {
		t.Errorf("recipe = %+v", r)
	}
	if len(r.Tags) != 1 || r.Tags[0] != "Breakfast" || r.Nutrition["calories"] != "320" {
		t.Errorf("tags/nutrition = %v %v", r.Tags, r.Nutrition)
	}
	if string(it.Image) != "JPEG" || it.Origin != "tandoor:12" {
		t.Errorf("image/origin = %q %q", it.Image, it.Origin)
	}
}

func gz(b []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(b)
	zw.Close()
	return buf.Bytes()
}

func TestPaprika(t *testing.T) {
	one, _ := json.Marshal(map[string]any{
		"uid": "ABC-1", "name": "Chili", "ingredients": "For the base:\n2 tbsp oil\n1 onion, diced\n\n1 lb beef",
		"directions": "Brown the beef.\nSimmer for an hour.", "notes": "Better the next day.", "servings": "6 servings",
		"prep_time": "15 mins", "cook_time": "1 hr 10 min", "source": "Grandma", "categories": []string{"Dinner", "Winter"},
		"rating": 5, "photo_data": base64.StdEncoding.EncodeToString([]byte("PHOTO")), "created": "2021-03-04 10:00:00",
	})
	two, _ := json.Marshal(map[string]any{"uid": "ABC-2", "name": "Toast", "ingredients": "1 slice bread", "directions": "Toast it."})
	file := save(t, "My Recipes.paprikarecipes", writeZip(t, map[string][]byte{"Chili.paprikarecipe": gz(one), "Toast.paprikarecipe": gz(two)}))
	if k := Detect(file); k != Paprika {
		t.Fatalf("detected %q", k)
	}
	var s sink
	if err := Import(file, Paprika, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.items) != 2 {
		t.Fatalf("got %d", len(s.items))
	}
	c := s.items[0]
	r := c.Recipe
	if r.Title != "Chili" || len(r.Ingredients) != 4 || !r.Ingredients[0].H || r.Ingredients[3].Text != "1 lb beef" {
		t.Errorf("ingredients = %+v", r.Ingredients)
	}
	if len(r.Steps) != 2 || r.PrepMin != 15 || r.CookMin != 70 || r.TotalMin != 85 || r.Servings != 6 || r.Rating != 5 {
		t.Errorf("recipe = %+v", r)
	}
	if r.SourceName != "Grandma" || len(r.Tags) != 2 || r.Notes != "Better the next day." || r.CreatedAt == 0 {
		t.Errorf("details = %+v", r)
	}
	if string(c.Image) != "PHOTO" || c.Origin != "paprika:ABC-1" {
		t.Errorf("image/origin = %q %q", c.Image, c.Origin)
	}
	// A single recipe file, gzipped JSON, is read too.
	single := save(t, "Toast.paprikarecipe", gz(two))
	if k := Detect(single); k != Paprika {
		t.Fatalf("single detected %q", k)
	}
	s = sink{}
	if err := Import(single, Paprika, &s); err != nil || len(s.items) != 1 {
		t.Fatalf("single = %v %d", err, len(s.items))
	}
}

func TestRecipeSage(t *testing.T) {
	data, _ := json.Marshal([]any{map[string]any{
		"@context": "http://schema.org", "@type": "Recipe", "identifier": "f00", "name": "Granola", "description": "Crunchy.",
		"recipeIngredient": []string{"3 cups oats", "1/2 cup honey"}, "recipeInstructions": []any{map[string]any{"@type": "HowToStep", "text": "Mix."}, map[string]any{"@type": "HowToStep", "text": "Bake at 325°F for 25 minutes."}},
		"recipeYield": "8 servings", "totalTime": "PT40M", "recipeCategory": []string{"breakfast", "snacks"}, "isBasedOn": "https://example.com/granola",
		"creditText": "Example Kitchen", "comment": []any{map[string]any{"@type": "Comment", "name": "Author Notes", "text": "Keeps a month."}},
		"image": []string{"https://cdn.example.com/granola.jpg"}, "datePublished": "2022-01-02T03:04:05.000Z",
	}})
	file := save(t, "recipesage-data.json", data)
	if k := Detect(file); k != RecipeSage {
		t.Fatalf("detected %q", k)
	}
	var s sink
	if err := Import(file, RecipeSage, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.items) != 1 {
		t.Fatalf("got %d", len(s.items))
	}
	it := s.items[0]
	r := it.Recipe
	if r.Title != "Granola" || len(r.Ingredients) != 2 || len(r.Steps) != 2 || r.TotalMin != 40 || r.Servings != 8 || r.Notes != "Keeps a month." {
		t.Errorf("recipe = %+v", r)
	}
	if r.SourceURL != "https://example.com/granola" || r.SourceName != "Example Kitchen" || len(r.Tags) != 2 {
		t.Errorf("source/tags = %q %q %v", r.SourceURL, r.SourceName, r.Tags)
	}
	if it.ImageURL != "https://cdn.example.com/granola.jpg" || it.Origin != "recipesage:f00" {
		t.Errorf("image/origin = %q %q", it.ImageURL, it.Origin)
	}
}

func TestDetect(t *testing.T) {
	cases := map[string]struct {
		data []byte
		want Kind
	}{
		"fond.zip":    {writeZip(t, map[string][]byte{"fond.json": []byte(`{"version":3}`)}), Fond},
		"mealie.zip":  {writeZip(t, map[string][]byte{"backup/database.json": []byte(`{}`)}), Mealie},
		"mexport.zip": {writeZip(t, map[string][]byte{"recipes/toast/toast.json": []byte(`{}`)}), Mealie},
		"notes.txt":   {[]byte("hello"), Unknown},
		"random.zip":  {writeZip(t, map[string][]byte{"a.txt": []byte("x")}), Unknown},
		"backup.json": {[]byte(`{"version":2,"exportedAt":1,"recipes":[]}`), Fond},
		"other.json":  {[]byte(`{"hello":"world"}`), Unknown},
		"sage.zip":    {writeZip(t, map[string][]byte{"data.json": []byte(`[{"@type":"Recipe","name":"x"}]`)}), RecipeSage},
	}
	for name, c := range cases {
		if got := Detect(save(t, name, c.data)); got != c.want {
			t.Errorf("%s: detected %q, want %q", name, got, c.want)
		}
	}
}
