package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/au12/fond/internal/model"
)

func setupCook(t *testing.T) (*client, *Server) {
	t.Helper()
	c, srv := newTestServer(t)
	if code, _ := c.do("POST", "/api/auth/setup", map[string]string{"username": "cook", "name": "Cook", "password": "correct horse"}); code != 200 {
		t.Fatalf("setup = %d", code)
	}
	return c, srv
}

func lines(texts ...string) []map[string]any {
	out := []map[string]any{}
	for _, x := range texts {
		out = append(out, map[string]any{"text": x})
	}
	return out
}

func versionNumbers(t *testing.T, c *client, id string) (current float64, numbers []float64, list []any) {
	t.Helper()
	code, out := c.do("GET", "/api/recipes/"+id+"/versions", nil)
	if code != 200 {
		t.Fatalf("versions = %d %v", code, out)
	}
	list = out["versions"].([]any)
	for _, v := range list {
		numbers = append(numbers, v.(map[string]any)["version"].(float64))
	}
	return out["version"].(float64), numbers, list
}

func TestRecipeHistory(t *testing.T) {
	c, _ := setupCook(t)
	body := map[string]any{"title": "Loaf", "yield": "1 loaf", "ingredients": lines("3 cups flour", "1 tsp salt"), "steps": lines("Mix.", "Bake.")}
	code, rec := c.do("POST", "/api/recipes", body)
	if code != 201 || rec["version"] != float64(1) {
		t.Fatalf("create = %d, version %v", code, rec["version"])
	}
	if cur, nums, _ := versionNumbers(t, c, "1"); cur != 1 || len(nums) != 1 {
		t.Fatalf("a new recipe has version %v and history %v", cur, nums)
	}

	// Saves that leave the ingredients and steps alone are not versions.
	body["notes"], body["title"], body["tags"] = "Better with rye.", "Sunday loaf", []string{"Bread"}
	if _, rec = c.do("PUT", "/api/recipes/1", body); rec["version"] != float64(1) {
		t.Errorf("changing notes, title and tags made version %v", rec["version"])
	}
	body["ingredients"] = lines("  3 cups   flour ", "1 tsp salt")
	if _, rec = c.do("PUT", "/api/recipes/1", body); rec["version"] != float64(1) {
		t.Errorf("a spacing-only edit made version %v", rec["version"])
	}

	// A real change is.
	body["ingredients"], body["versionNote"] = lines("3 cups (360 g) flour", "1 tsp salt"), "Weights added"
	if _, rec = c.do("PUT", "/api/recipes/1", body); rec["version"] != float64(2) {
		t.Fatalf("an ingredient edit left version %v", rec["version"])
	}
	if _, ok := rec["versionNote"]; ok {
		t.Errorf("the note leaked into the recipe: %v", rec["versionNote"])
	}
	delete(body, "versionNote")
	body["steps"] = lines("Mix.", "Rest 30 minutes.", "Bake.")
	if _, rec = c.do("PUT", "/api/recipes/1", body); rec["version"] != float64(3) {
		t.Fatalf("a step edit left version %v", rec["version"])
	}
	cur, nums, list := versionNumbers(t, c, "1")
	if cur != 3 || len(nums) != 3 || nums[0] != 3 || nums[2] != 1 {
		t.Fatalf("history = %v (current %v), want newest first 3,2,1", nums, cur)
	}
	v2 := list[1].(map[string]any)
	if v2["note"] != "Weights added" || v2["user"] != "Cook" || v2["at"].(float64) == 0 {
		t.Errorf("version 2 = %v", v2)
	}
	if first := list[2].(map[string]any)["ingredients"].([]any)[0].(map[string]any)["text"]; first != "3 cups flour" {
		t.Errorf("version 1 was rewritten: %v", first)
	}

	// Restoring brings the old lines back as a new version and loses nothing.
	code, rec = c.do("POST", "/api/recipes/1/versions/1/restore", nil)
	if code != 200 || rec["version"] != float64(4) {
		t.Fatalf("restore = %d, version %v", code, rec["version"])
	}
	if got := rec["ingredients"].([]any)[0].(map[string]any)["text"]; got != "3 cups flour" {
		t.Errorf("restored ingredients = %v", got)
	}
	if len(rec["steps"].([]any)) != 2 || rec["title"] != "Sunday loaf" || rec["notes"] != "Better with rye." {
		t.Errorf("restore should bring back the steps and keep the title and notes: %v", rec)
	}
	_, _, list = versionNumbers(t, c, "1")
	if note := list[0].(map[string]any)["note"]; note != "Restored version 1" {
		t.Errorf("restore note = %v", note)
	}
	// Restoring what is already there changes nothing.
	if _, rec = c.do("POST", "/api/recipes/1/versions/1/restore", nil); rec["version"] != float64(4) {
		t.Errorf("a no-op restore made version %v", rec["version"])
	}
	if code, _ = c.do("POST", "/api/recipes/1/versions/99/restore", nil); code != 404 {
		t.Errorf("restoring a missing version = %d", code)
	}
	if code, _ = c.do("GET", "/api/recipes/7/versions", nil); code != 404 {
		t.Errorf("history of a missing recipe = %d", code)
	}
}

func TestHistoryIsBounded(t *testing.T) {
	_, srv := setupCook(t)
	rec := &model.Recipe{Title: "Stew", Ingredients: []model.Line{{Text: "1 onion"}}}
	if err := srv.store.SaveRecipe(rec, 1, ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxVersions+20; i++ {
		rec.Ingredients = []model.Line{{Text: itoa(i+2) + " onions"}}
		if err := srv.store.SaveRecipe(rec, 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	vs, _ := srv.store.Versions(rec.ID)
	if len(vs) != maxVersions+1 || vs[len(vs)-1].Version != 1 || vs[0].Version != maxVersions+21 {
		t.Errorf("kept %d versions, oldest %d, newest %d", len(vs), vs[len(vs)-1].Version, vs[0].Version)
	}
	// Deleting the recipe takes its history with it.
	srv.store.DeleteRecipe(rec.ID)
	if vs, _ = srv.store.Versions(rec.ID); len(vs) != 0 {
		t.Errorf("%d versions outlived their recipe", len(vs))
	}
}

func TestMigrationGivesOldRecipesAHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRecipe(&model.Recipe{Title: "Old favourite", Ingredients: []model.Line{{Text: "2 eggs"}}}, 0, ""); err != nil {
		t.Fatal(err)
	}
	// Make the database look as it did before recipes had versions.
	for _, q := range []string{`DROP TABLE recipe_versions`, `DROP TABLE food_rules`, `ALTER TABLE recipes DROP COLUMN version`} {
		if err := store.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	store.Close()

	store, err = OpenStore(dir)
	if err != nil {
		t.Fatalf("reopening an old database: %v", err)
	}
	defer store.Close()
	rec, err := store.Recipe(1, 0)
	if err != nil || rec.Version != 1 {
		t.Fatalf("recipe after migration: %v, version %d", err, rec.Version)
	}
	vs, _ := store.Versions(1)
	if len(vs) != 1 || vs[0].Version != 1 || vs[0].Ingredients[0].Text != "2 eggs" {
		t.Fatalf("history after migration = %+v", vs)
	}
	// And the next real edit carries on from there.
	rec.Ingredients = []model.Line{{Text: "3 eggs"}}
	if err := store.SaveRecipe(rec, 0, ""); err != nil || rec.Version != 2 {
		t.Errorf("edit after migration: %v, version %d", err, rec.Version)
	}
}

func TestFoodRules(t *testing.T) {
	c, _ := setupCook(t)
	code, rule := c.do("PUT", "/api/food-rules", map[string]any{"phrase": "  House  Granola ", "action": "custom", "gPerMl": 0.5})
	if code != 200 || rule["phrase"] != "house granola" || rule["source"] != "user" {
		t.Fatalf("save = %d %v", code, rule)
	}
	// One rule per phrase: saving again replaces it.
	code, again := c.do("PUT", "/api/food-rules", map[string]any{"phrase": "HOUSE granola", "action": "map", "foodId": "oats", "source": "ai"})
	if code != 200 || again["id"] != rule["id"] || again["foodId"] != "oats" || again["source"] != "ai" {
		t.Fatalf("replace = %d %v", code, again)
	}
	if _, ok := again["gPerMl"]; ok {
		t.Errorf("a mapping kept the old weight: %v", again)
	}
	for _, bad := range []map[string]any{
		{"phrase": "", "action": "skip"},
		{"phrase": "x", "action": "explode"},
		{"phrase": "x", "action": "map", "foodId": "Not An Id"},
		{"phrase": "x", "action": "custom"},
		{"phrase": "x", "action": "custom", "gPerMl": 40},
	} {
		if code, _ := c.do("PUT", "/api/food-rules", bad); code != 400 {
			t.Errorf("%v accepted: %d", bad, code)
		}
	}
	c.do("PUT", "/api/food-rules", map[string]any{"phrase": "gold leaf", "action": "skip"})
	res, _ := c.c.Get(c.base + "/api/food-rules")
	var rules []map[string]any
	json.NewDecoder(res.Body).Decode(&rules)
	if len(rules) != 2 || rules[0]["phrase"] != "gold leaf" {
		t.Fatalf("list = %v", rules)
	}

	// A shared recipe carries the household's rules so it weighs the same for a guest.
	c.do("POST", "/api/recipes", map[string]any{"title": "Granola bars", "ingredients": lines("2 cups house granola")})
	_, sh := c.do("PUT", "/api/recipes/1/share", map[string]bool{"on": true})
	anon := &client{t: t, c: &http.Client{}, base: c.base}
	code, pub := anon.do("GET", "/api/public/recipe/"+sh["shareToken"].(string), nil)
	if code != 200 || len(pub["foodRules"].([]any)) != 2 {
		t.Errorf("public recipe = %d, rules %v", code, pub["foodRules"])
	}
	if code, _ := anon.do("GET", "/api/food-rules", nil); code != 401 {
		t.Errorf("anonymous rules = %d", code)
	}

	if code, _ := c.do("DELETE", "/api/food-rules/"+itoa(int(rule["id"].(float64))), nil); code != 200 {
		t.Errorf("delete = %d", code)
	}
	res, _ = c.c.Get(c.base + "/api/food-rules")
	rules = nil
	json.NewDecoder(res.Body).Decode(&rules)
	if len(rules) != 1 {
		t.Errorf("after delete: %v", rules)
	}
}

func TestIngredientLines(t *testing.T) {
	c, _ := setupCook(t)
	c.do("POST", "/api/recipes", map[string]any{"title": "A", "ingredients": []map[string]any{{"text": "For the dough", "h": true}, {"text": "1 cup flour"}, {"text": "1 tsp salt"}}})
	c.do("POST", "/api/recipes", map[string]any{"title": "B", "ingredients": lines("1 cup flour", "2 eggs")})
	code, out := c.do("GET", "/api/ingredient-lines", nil)
	if code != 200 {
		t.Fatalf("lines = %d", code)
	}
	got := map[string]float64{}
	for _, l := range out["lines"].([]any) {
		m := l.(map[string]any)
		got[m["t"].(string)] = m["n"].(float64)
		if m["t"] == "1 cup flour" && len(m["r"].([]any)) != 2 {
			t.Errorf("flour is used in %v", m["r"])
		}
	}
	if len(got) != 3 || got["1 cup flour"] != 2 || got["2 eggs"] != 1 {
		t.Errorf("lines = %v", got)
	}
}

func TestSuggestFoods(t *testing.T) {
	c, _ := setupCook(t)
	ask := map[string]any{"phrases": []string{"icing sugar", "gold leaf"}, "foods": [][2]string{{"sugar-powdered", "powdered sugar"}, {"flour-ap", "all-purpose flour"}}}
	if code, _ := c.do("POST", "/api/food-rules/suggest", ask); code != 400 {
		t.Fatalf("suggestions without AI set up = %d, want 400", code)
	}
	var prompt string
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Messages []struct{ Content string }
		}
		json.NewDecoder(r.Body).Decode(&in)
		prompt = in.Messages[1].Content
		// A model that also invents an id and answers a question nobody asked.
		reply := "Sure! ```json\n{\"icing sugar\": \"sugar-powdered\", \"gold leaf\": \"metal-gold\", \"unasked\": \"flour-ap\"}\n```"
		json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": reply}}}})
	}))
	defer ai.Close()
	if code, out := c.do("PUT", "/api/settings/ai", map[string]string{"baseUrl": ai.URL, "model": "test"}); code != 200 {
		t.Fatalf("configure AI = %d %v", code, out)
	}
	code, out := c.do("POST", "/api/food-rules/suggest", ask)
	if code != 200 {
		t.Fatalf("suggest = %d %v", code, out)
	}
	got := out["suggestions"].(map[string]any)
	if len(got) != 1 || got["icing sugar"] != "sugar-powdered" {
		t.Errorf("suggestions = %v", got)
	}
	if !strings.Contains(prompt, "sugar-powdered: powdered sugar") || !strings.Contains(prompt, "gold leaf") {
		t.Errorf("prompt = %q", prompt)
	}
	if code, _ := c.do("POST", "/api/food-rules/suggest", map[string]any{"phrases": []string{}, "foods": ask["foods"]}); code != 400 {
		t.Errorf("empty question = %d", code)
	}
}
