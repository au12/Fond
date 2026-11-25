package scrape

import (
	"strings"
	"testing"
)

const page = `<html><head><title>Best Lemon Bars Recipe | Example Kitchen</title>
<meta property="og:image" content="/img/bars.jpg"><meta property="og:site_name" content="Example Kitchen">
<script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"WebSite","name":"Example Kitchen"},
{"@type":["Recipe","NewsArticle"],"name":"Lemon Bars","description":"Tart &amp; buttery.","recipeYield":["16","16 bars"],
"prepTime":"PT20M","cookTime":"PT45M","totalTime":"P0DT1H5M","author":{"@id":"#me"},
"recipeIngredient":["1 cup (2 sticks) butter","  2 cups flour ","4 large eggs"],
"recipeInstructions":[{"@type":"HowToSection","name":"Crust","itemListElement":[{"@type":"HowToStep","text":"1. Press the dough into the pan."}]},
{"@type":"HowToStep","text":"<p>Bake for 20 minutes.</p>"}],"recipeCategory":"Dessert","keywords":"lemon, bars, easy, best lemon bars ever"},
{"@type":"Person","@id":"#me","name":"Sam Cook"}]}</script></head><body></body></html>`

func TestExtractJSONLD(t *testing.T) {
	res, err := Extract([]byte(page), "https://example.com/lemon-bars?utm_source=x")
	if err != nil {
		t.Fatal(err)
	}
	r := res.Recipe
	if r.Title != "Lemon Bars" || r.Description != "Tart & buttery." || r.Author != "Sam Cook" || r.SourceName != "Example Kitchen" {
		t.Errorf("basics: %+v", r)
	}
	if r.Yield != "16 bars" || r.Servings != 16 || r.PrepMin != 20 || r.CookMin != 45 || r.TotalMin != 65 {
		t.Errorf("yield/times: %q %v %d %d %d", r.Yield, r.Servings, r.PrepMin, r.CookMin, r.TotalMin)
	}
	if len(r.Ingredients) != 3 || r.Ingredients[1].Text != "2 cups flour" {
		t.Errorf("ingredients: %+v", r.Ingredients)
	}
	if len(r.Steps) != 3 || !r.Steps[0].H || r.Steps[1].Text != "Press the dough into the pan." || r.Steps[2].Text != "Bake for 20 minutes." {
		t.Errorf("steps: %+v", r.Steps)
	}
	if len(r.Tags) != 1 || r.Tags[0] != "Dessert" {
		t.Errorf("SEO keywords must not become tags: %v", r.Tags)
	}
	if res.ImageURL != "https://example.com/img/bars.jpg" || res.Method != "json-ld" {
		t.Errorf("image %q method %q", res.ImageURL, res.Method)
	}
}

func TestExtractHeadings(t *testing.T) {
	html := `<html><body><h1>Grandma's Stew</h1><h3>Ingredients</h3><ul><li>2 lb beef</li><li>3 carrots</li><li>1 onion</li></ul>
<h3>Directions</h3><ol><li>Brown the beef.</li><li>Simmer for 2 hours.</li></ol><h3>Notes</h3><p>Freezes well.</p></body></html>`
	res, err := Extract([]byte(html), "https://blog.example/stew")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Recipe.Ingredients) != 3 || len(res.Recipe.Steps) != 2 || res.Recipe.Title != "Grandma's Stew" {
		t.Errorf("%+v", res.Recipe)
	}
}

func TestNoRecipe(t *testing.T) {
	if _, err := Extract([]byte(`<html><body><h1>About us</h1><p>We sell shoes.</p></body></html>`), "https://example.com"); err != ErrNoRecipe {
		t.Errorf("want ErrNoRecipe, got %v", err)
	}
}

func TestParseMinutes(t *testing.T) {
	for in, want := range map[string]int{"PT1H30M": 90, "PT45M": 45, "P1DT2H": 1560, "1 hour 15 minutes": 75, "45 min": 45, "1h 30m": 90, "90": 90, "1½ hours": 90, "": 0} {
		if got := ParseMinutes(in); got != want {
			t.Errorf("ParseMinutes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestFromText(t *testing.T) {
	r, err := FromText("Weeknight Dal\nServes 4\n\nIngredients\n1 cup red lentils\n- 1 onion, diced\n\nFor the tarka:\n2 tbsp ghee\n\nMethod\n1. Rinse the lentils.\n2. Simmer for 20 minutes.\n")
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "Weeknight Dal" || r.Servings != 4 || len(r.Ingredients) != 4 || !r.Ingredients[2].H || len(r.Steps) != 2 || strings.HasPrefix(r.Steps[0].Text, "1") {
		t.Errorf("%+v", r)
	}
}

func TestBlocksPrivateAddresses(t *testing.T) {
	f := NewFetcher(false)
	for _, u := range []string{"http://127.0.0.1/", "http://10.0.0.5/x", "http://169.254.169.254/latest/meta-data", "http://[::1]/", "http://localhost/"} {
		if _, _, err := f.Page(t.Context(), u); err != ErrBlockedAddress {
			t.Errorf("%s: want ErrBlockedAddress, got %v", u, err)
		}
	}
}
