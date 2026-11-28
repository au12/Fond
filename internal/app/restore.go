package app

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/au12/fond/internal/mealie"
	"github.com/au12/fond/internal/migrate"
	"github.com/au12/fond/internal/model"
)

// ------------------------------------------------------------------ Fond backups

// backup is the shape of fond.json in a Fond export. Version 2 (Fond 2.x) has recipes, cookbooks, history
// and food rules; version 3 adds where each recipe came from, favourites, the meal plan and the shopping list.
type backup struct {
	Version    int                   `json:"version"`
	ExportedAt int64                 `json:"exportedAt"`
	Recipes    []*model.Recipe       `json:"recipes"`
	Cookbooks  []*Cookbook           `json:"cookbooks"`
	History    map[string][]*Version `json:"history"`
	FoodRules  []*FoodRule           `json:"foodRules"`
	Origins    map[string]string     `json:"origins,omitempty"`
	Favorites  []backupFavorite      `json:"favorites,omitempty"`
	MealPlan   []PlanEntry           `json:"mealPlan,omitempty"`
	Shopping   []ShopItem            `json:"shopping,omitempty"`
}

type backupFavorite struct {
	User     string `json:"user"`
	RecipeID int64  `json:"recipeId"`
}

// exportAll streams everything the household keeps as one zip: a portable backup that Fond can restore.
func (s *Server) exportAll(w http.ResponseWriter, r *http.Request, u *User) error {
	b := backup{Version: 3, ExportedAt: now(), History: map[string][]*Version{}, Origins: map[string]string{}}
	rows, err := s.store.db.Query(`SELECT ` + recipeCols + `, origin FROM recipes ORDER BY id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var origin string
		rec, err := scanRecipe(scanTail{rows, &origin})
		if err != nil {
			continue
		}
		rec.ShareToken = ""
		b.Recipes = append(b.Recipes, rec)
		if origin != "" {
			b.Origins[itoa(int(rec.ID))] = origin
		}
	}
	rows.Close()
	b.Cookbooks, _ = s.store.Cookbooks()
	for _, c := range b.Cookbooks {
		s.store.cookbookEntries(c)
		c.ShareToken = ""
	}
	for _, rec := range b.Recipes {
		if vs, err := s.store.Versions(rec.ID); err == nil && len(vs) > 1 {
			b.History[itoa(int(rec.ID))] = vs
		}
	}
	b.FoodRules, _ = s.store.FoodRules()
	if fr, err := s.store.db.Query(`SELECT u.username, f.recipe_id FROM favorites f JOIN users u ON u.id=f.user_id ORDER BY f.user_id, f.recipe_id`); err == nil {
		for fr.Next() {
			var f backupFavorite
			fr.Scan(&f.User, &f.RecipeID)
			b.Favorites = append(b.Favorites, f)
		}
		fr.Close()
	}
	b.MealPlan, _ = s.store.Plan("0000-01-01", "9999-12-31")
	b.Shopping = s.store.Shopping()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="fond-export-`+time.Now().Format("2006-01-02")+`.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()
	f, _ := zw.Create("fond.json")
	json.NewEncoder(f).Encode(b)
	for _, rec := range b.Recipes {
		if rec.Image == "" {
			continue
		}
		if src, err := os.Open(filepath.Join(s.images.Dir, rec.Image+".jpg")); err == nil {
			if dst, err := zw.CreateHeader(&zip.FileHeader{Name: "images/" + rec.Image + ".jpg", Method: zip.Store}); err == nil {
				io.Copy(dst, src)
			}
			src.Close()
		}
	}
	return nil
}

// scanTail scans a recipe row that carries one more column at the end.
type scanTail struct {
	rows interface{ Scan(...any) error }
	tail any
}

func (t scanTail) Scan(dest ...any) error { return t.rows.Scan(append(dest, t.tail)...) }

// restoreFond reads a Fond export back in. "merge" adds what the library does not have yet; "replace"
// first clears the library, cookbooks, corrections, meal plan and shopping list, then restores the backup as it was.
func (s *Server) restoreFond(file, mode string, j *Job) error {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return errors.New("that file isn't a readable zip archive")
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	var manifest *zip.File
	for _, f := range zr.File {
		name := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.Contains(name, "__MACOSX") {
			continue
		}
		files[name] = f
		if path.Base(name) == "fond.json" && (manifest == nil || len(name) < len(manifest.Name)) {
			manifest = f
		}
	}
	if manifest == nil {
		return errors.New("this zip has no fond.json, so it isn't a Fond export")
	}
	root := path.Dir(path.Clean(strings.ReplaceAll(manifest.Name, "\\", "/")))
	rc, err := manifest.Open()
	if err != nil {
		return err
	}
	var b backup
	err = json.NewDecoder(io.LimitReader(rc, 1<<30)).Decode(&b)
	rc.Close()
	if err != nil {
		return errors.New("fond.json in this zip can't be read")
	}
	if b.Version > 3 {
		return fmt.Errorf("this backup was made by a newer Fond (format %d); update Fond and try again", b.Version)
	}
	st := s.store

	var oldImages []string
	if mode == "replace" {
		if r, err := st.db.Query(`SELECT image FROM recipes WHERE image<>''`); err == nil {
			for r.Next() {
				var k string
				r.Scan(&k)
				oldImages = append(oldImages, k)
			}
			r.Close()
		}
		tx, err := st.db.Begin()
		if err != nil {
			return err
		}
		for _, q := range []string{`DELETE FROM meal_plan`, `DELETE FROM shopping`, `DELETE FROM cookbooks`, `DELETE FROM food_rules`, `DELETE FROM recipes`} {
			if _, err := tx.Exec(q); err != nil {
				tx.Rollback()
				return err
			}
		}
		if st.fts {
			tx.Exec(`DELETE FROM recipes_fts`)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	ids := map[int64]int64{} // id in the backup → id here
	total := len(b.Recipes)
	for i, rec := range b.Recipes {
		j.Progress(i, total, rec.Title)
		old := rec.ID
		origin := b.Origins[itoa(int(old))]
		if origin == "" {
			origin = "fond:" + strconv.FormatInt(b.ExportedAt, 36) + ":" + itoa(int(old))
		}
		if mode != "replace" {
			if id := st.findSame(rec, origin); id > 0 {
				ids[old] = id
				j.mu.Lock()
				j.Skipped++
				j.mu.Unlock()
				continue
			}
		}
		key, color := "", ""
		if rec.Image != "" {
			if f := files[path.Join(root, "images", rec.Image+".jpg")]; f != nil {
				if data := readZip(f, 60<<20); data != nil {
					key, color, _ = s.images.Save(data)
				}
			}
		}
		history := b.History[itoa(int(old))]
		r := *rec
		r.ID, r.ShareToken, r.Image, r.ImageColor, r.Favorite = 0, "", key, color, false
		if err := st.SaveRecipe(&r, j.userID, origin); err != nil {
			if key != "" {
				s.images.Delete(key)
			}
			j.mu.Lock()
			j.problem(fmt.Sprintf("%s: %v", rec.Title, err))
			j.mu.Unlock()
			continue
		}
		ids[old] = r.ID
		if len(history) > 0 {
			st.restoreHistory(r.ID, rec.Version, history)
		}
		j.mu.Lock()
		j.Imported++
		j.mu.Unlock()
	}
	j.Progress(total, total, "Cookbooks and corrections")

	for _, f := range b.FoodRules {
		if f.validate() != nil {
			continue
		}
		if mode != "replace" {
			var n int
			st.db.QueryRow(`SELECT COUNT(*) FROM food_rules WHERE phrase=?`, f.Phrase).Scan(&n)
			if n > 0 {
				continue // the household's own correction wins over the backup's
			}
		}
		st.SaveFoodRule(f, j.userID)
	}

	for _, c := range b.Cookbooks {
		if mode != "replace" {
			var n int
			st.db.QueryRow(`SELECT COUNT(*) FROM cookbooks WHERE title=? AND leather=?`, c.Title, c.Leather).Scan(&n)
			if n > 0 {
				continue
			}
		}
		entries := []CookbookItem{}
		for _, e := range c.Entries {
			if id, ok := ids[e.RecipeID]; ok {
				entries = append(entries, CookbookItem{RecipeID: id, Chapter: e.Chapter})
			}
		}
		book := *c
		book.ID, book.ShareToken = 0, ""
		if err := st.SaveCookbook(&book, entries); err == nil {
			j.mu.Lock()
			j.Cookbooks++
			j.mu.Unlock()
		}
	}

	users := map[string]int64{}
	if us, err := st.Users(); err == nil {
		for _, u := range us {
			users[strings.ToLower(u.Username)] = u.ID
		}
	}
	for _, f := range b.Favorites {
		if uid, ok := users[strings.ToLower(f.User)]; ok {
			if id, ok := ids[f.RecipeID]; ok {
				st.Exec(`INSERT OR IGNORE INTO favorites (user_id, recipe_id) VALUES (?,?)`, uid, id)
			}
		}
	}

	for _, e := range b.MealPlan {
		if !validDay(e.Day) {
			continue
		}
		if _, ok := meals[e.Meal]; !ok {
			e.Meal = "dinner"
		}
		var rid *int64
		if e.RecipeID != nil {
			id, ok := ids[*e.RecipeID]
			if !ok {
				continue
			}
			rid = &id
		}
		if rid == nil && e.Text == "" {
			continue
		}
		if mode != "replace" {
			var n int
			st.db.QueryRow(`SELECT COUNT(*) FROM meal_plan WHERE day=? AND meal=? AND recipe_id IS ? AND text=?`, e.Day, e.Meal, rid, e.Text).Scan(&n)
			if n > 0 {
				continue
			}
		}
		st.Exec(`INSERT INTO meal_plan (day, meal, recipe_id, text, servings, position, created_by, created_at) VALUES (?,?,?,?,?,?,?,?)`,
			e.Day, e.Meal, rid, clipStr(e.Text, 200), e.Servings, e.Position, j.userID, now())
	}

	have := map[string]bool{}
	if mode != "replace" {
		for _, it := range st.Shopping() {
			have[strings.ToLower(it.Text)] = true
		}
	}
	for _, it := range b.Shopping {
		t := clipStr(model.CleanLine(it.Text), 300)
		if t == "" || have[strings.ToLower(t)] || (mode != "replace" && it.Checked) {
			continue
		}
		var rid *int64
		if it.RecipeID != nil {
			if id, ok := ids[*it.RecipeID]; ok {
				rid = &id
			}
		}
		st.Exec(`INSERT INTO shopping (text, checked, recipe_id, recipe_title, grp, created_at) VALUES (?,?,?,?,?,?)`, t, it.Checked, rid, clipStr(it.RecipeTitle, 200), clipStr(it.Group, 40), now())
	}

	for _, k := range oldImages {
		s.images.Delete(k)
	}
	j.Progress(total, total, "")
	return nil
}

// findSame finds a recipe already in the library that a backup's recipe is a copy of.
func (st *Store) findSame(rec *model.Recipe, origin string) int64 {
	var id int64
	if st.db.QueryRow(`SELECT id FROM recipes WHERE origin=? LIMIT 1`, origin).Scan(&id) == nil {
		return id
	}
	r := *rec
	r.Normalize()
	if st.db.QueryRow(`SELECT id FROM recipes WHERE title=? AND ingredients=? AND steps=? LIMIT 1`, r.Title, js(r.Ingredients), js(r.Steps)).Scan(&id) == nil {
		return id
	}
	return 0
}

// restoreHistory puts back a recipe's versions as the backup kept them.
func (st *Store) restoreHistory(id int64, current int, versions []*Version) {
	tx, err := st.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM recipe_versions WHERE recipe_id=?`, id)
	top := current
	for _, v := range versions {
		if v.Version < 1 {
			continue
		}
		tx.Exec(`INSERT OR REPLACE INTO recipe_versions (recipe_id, version, title, yield, servings, ingredients, steps, note, user_id, user_name, created_at)
			VALUES (?,?,?,?,?,?,?,?,NULL,?,?)`, id, v.Version, v.Title, v.Yield, v.Servings, js(v.Ingredients), js(v.Steps), clipStr(v.Note, 300), clipStr(v.User, 60), v.At)
		if v.Version > top {
			top = v.Version
		}
	}
	if top < 1 {
		top = 1
	}
	tx.Exec(`UPDATE recipes SET version=? WHERE id=?`, top, id)
	// The recipe as it stands is always the latest version.
	tx.Exec(`INSERT OR IGNORE INTO recipe_versions (recipe_id, version, title, yield, servings, ingredients, steps, note, user_id, user_name, created_at)
		SELECT id, version, title, yield, servings, ingredients, steps, '', created_by, '', updated_at FROM recipes WHERE id=?`, id)
	tx.Commit()
}

func readZip(f *zip.File, limit int64) []byte {
	if int64(f.UncompressedSize64) > limit {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, limit))
	if err != nil {
		return nil
	}
	return b
}

// ------------------------------------------------------------------ one door for every app's export

// runImport reads a file of any format Fond knows into a job.
func (s *Server) runImport(file string, kind migrate.Kind, mode string, j *Job) error {
	switch kind {
	case migrate.Fond:
		return s.restoreFond(file, mode, j)
	case migrate.Mealie:
		return mealie.ImportZip(file, j)
	default:
		return migrate.Import(file, kind, j)
	}
}

// importFile takes an export from Fond, Mealie, Tandoor, Paprika or Recipe Sage, works out which it is,
// and reads it in the background.
func (s *Server) importFile(w http.ResponseWriter, r *http.Request, u *User) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<30)
	mr, err := r.MultipartReader()
	if err != nil {
		return fail(400, "Send the export as a file upload")
	}
	os.MkdirAll(filepath.Join(s.cfg.DataDir, "tmp"), 0o755)
	tmp, err := os.CreateTemp(filepath.Join(s.cfg.DataDir, "tmp"), "import-*")
	if err != nil {
		return err
	}
	found, mode := false, "merge"
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		switch part.FormName() {
		case "file":
			if _, err := io.Copy(tmp, part); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return fail(400, "The upload was interrupted")
			}
			found = true
		case "mode":
			b, _ := io.ReadAll(io.LimitReader(part, 20))
			if string(b) == "replace" {
				mode = "replace"
			}
		}
		part.Close()
	}
	tmp.Close()
	if !found {
		os.Remove(tmp.Name())
		return fail(400, "No file was sent")
	}
	kind := migrate.Detect(tmp.Name())
	if kind == migrate.Unknown {
		os.Remove(tmp.Name())
		return fail(400, "Fond can't tell what made this file. It reads exports from Fond, Mealie, Tandoor, Paprika (.paprikarecipes) and Recipe Sage (JSON-LD).")
	}
	if kind != migrate.Fond {
		mode = "merge"
	}
	j := s.startJob(u, func(j *Job) error {
		defer os.Remove(tmp.Name())
		return s.runImport(tmp.Name(), kind, mode, j)
	})
	s.writeJSON(w, r, 202, map[string]string{"job": j.ID, "kind": string(kind), "name": migrate.Names[kind]})
	return nil
}

// fetchImage fetches a photo that an export links to rather than includes.
func (s *Server) fetchImage(link string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	b, err := s.public.Bytes(ctx, link, 40<<20)
	if err != nil {
		return nil
	}
	return b
}
