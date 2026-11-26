package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func (c *client) fetch(path string) (int, string) {
	c.t.Helper()
	res, err := c.c.Get(c.base + path)
	if err != nil {
		c.t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode, res.Header.Get("Content-Type")
}

func TestCookbookKeepsItsLeather(t *testing.T) {
	c, _ := newTestServer(t)
	c.do("POST", "/api/auth/setup", map[string]string{"username": "cook", "password": "correct horse"})

	_, made := c.do("POST", "/api/cookbooks", map[string]any{"title": "Breads", "color": "bay"})
	id, seed := int(made["id"].(float64)), made["leather"].(float64)
	if seed == 0 {
		t.Fatal("a new cookbook was given no hide")
	}
	_, other := c.do("POST", "/api/cookbooks", map[string]any{"title": "Soups", "color": "bay"})
	if other["leather"].(float64) == seed {
		t.Fatal("two new cookbooks were given the same hide")
	}

	// A save that names no hide, or zero, leaves the cover as it was.
	for _, body := range []map[string]any{{"title": "Breads and rolls", "color": "bay"}, {"title": "Breads and rolls", "color": "tomato", "leather": 0}} {
		_, saved := c.do("PUT", fmt.Sprintf("/api/cookbooks/%d", id), body)
		if saved["leather"].(float64) != seed {
			t.Fatalf("saving %v changed the hide from %v to %v", body, seed, saved["leather"])
		}
	}
	// Asking for another hide changes it, and the choice sticks.
	c.do("PUT", fmt.Sprintf("/api/cookbooks/%d", id), map[string]any{"title": "Breads", "color": "bay", "leather": 4242})
	if _, got := c.do("GET", fmt.Sprintf("/api/cookbooks/%d", id), nil); got["leather"].(float64) != 4242 {
		t.Fatalf("hide after choosing 4242 = %v", got["leather"])
	}
	// A new cookbook may arrive with its hide already chosen.
	if _, picked := c.do("POST", "/api/cookbooks", map[string]any{"title": "Cakes", "color": "plum", "leather": 99}); picked["leather"].(float64) != 99 {
		t.Fatalf("hide chosen at creation = %v, want 99", picked["leather"])
	}
}

func TestLeatherIsDrawnOnlyForThoseAllowed(t *testing.T) {
	c, srv := newTestServer(t)
	c.do("POST", "/api/auth/setup", map[string]string{"username": "cook", "password": "correct horse"})
	_, book := c.do("POST", "/api/cookbooks", map[string]any{"title": "Breads", "color": "bay", "leather": 31337})
	id := int(book["id"].(float64))
	visitor := &client{t: t, c: &http.Client{}, base: c.base}

	for _, bad := range []string{"/leather/31337-bay-x.jpg", "/leather/0-bay-s.jpg", "/leather/31337-mauve-s.jpg", "/leather/99999999999-bay-s.jpg", "/leather/..%2Ffond.db", "/leather/31337-bay-s.png"} {
		if code, _ := c.fetch(bad); code != 404 {
			t.Errorf("%s = %d, want 404", bad, code)
		}
	}
	// A visitor cannot make the server draw: not a hide nobody uses, nor the cover of a book that is not shared.
	for _, path := range []string{"/leather/777-bay-s.jpg", "/leather/31337-bay-s.jpg"} {
		if code, _ := visitor.fetch(path); code != 404 {
			t.Errorf("visitor %s = %d, want 404", path, code)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(srv.cfg.DataDir, "leather")); len(entries) != 0 {
		t.Fatalf("%d hides were drawn for a visitor", len(entries))
	}
	// Once the book is shared its cover is drawn for anyone, in both sizes, and kept.
	c.do("PUT", fmt.Sprintf("/api/cookbooks/%d/share", id), map[string]bool{"on": true})
	for _, size := range []string{"s", "l"} {
		code, ctype := visitor.fetch("/leather/31337-bay-" + size + ".jpg")
		if code != 200 || ctype != "image/jpeg" {
			t.Fatalf("shared cover, size %s = %d %s", size, code, ctype)
		}
	}
	// Someone signed in may try any hide while choosing one.
	if code, _ := c.fetch("/leather/777-chestnut-s.jpg"); code != 200 {
		t.Fatalf("signed-in preview = %d", code)
	}

	// Hides nobody is bound in are thrown away after a day; a book's own are kept.
	dir := filepath.Join(srv.cfg.DataDir, "leather")
	old := time.Now().Add(-48 * time.Hour)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		os.Chtimes(filepath.Join(dir, e.Name()), old, old)
	}
	srv.pruneLeather()
	left := map[string]bool{}
	entries, _ = os.ReadDir(dir)
	for _, e := range entries {
		left[e.Name()] = true
	}
	if !left["31337-bay-s.jpg"] || !left["31337-bay-l.jpg"] || len(left) != 2 {
		t.Fatalf("after pruning: %v", left)
	}
}

func TestOldCookbooksAreGivenLeather(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	// As a database from before covers had hides: no such column.
	for _, q := range []string{
		`ALTER TABLE cookbooks DROP COLUMN leather`,
		`INSERT INTO cookbooks (title, created_at, updated_at) VALUES ('One', 1, 1), ('Two', 1, 1)`,
	} {
		if _, err := store.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	store.Close()
	if store, err = OpenStore(dir); err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	books, _ := store.Cookbooks()
	if len(books) != 2 || books[0].Leather == 0 || books[1].Leather == 0 || books[0].Leather == books[1].Leather {
		t.Fatalf("hides after migration: %+v", books)
	}
	// Opening it again must not deal the books new hides.
	first := books[0].Leather
	store.Close()
	if store, err = OpenStore(dir); err != nil {
		t.Fatal(err)
	}
	if again, _ := store.Cookbooks(); again[0].Leather != first {
		t.Fatalf("hide changed on reopening: %d then %d", first, again[0].Leather)
	}
}
