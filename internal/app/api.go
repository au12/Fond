package app

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/au12/fond/internal/model"
)

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	m.HandleFunc("GET /img/{name}", s.handleImage)
	m.HandleFunc("GET /s/{token}", s.sharedRecipePage)
	m.HandleFunc("GET /c/{token}", s.sharedCookbookPage)
	m.HandleFunc("POST /share", s.receiveClip)
	m.HandleFunc("/", s.handleStatic)

	m.HandleFunc("GET /api/state", s.open(s.state))
	m.HandleFunc("POST /api/auth/setup", s.open(s.setup))
	m.HandleFunc("POST /api/auth/login", s.open(s.login))
	m.HandleFunc("POST /api/auth/logout", s.open(s.logout))
	m.HandleFunc("GET /api/me", s.user(s.me))
	m.HandleFunc("PUT /api/me", s.user(s.updateMe))
	m.HandleFunc("PUT /api/me/prefs", s.user(s.updatePrefs))
	m.HandleFunc("PUT /api/me/password", s.user(s.changePassword))
	m.HandleFunc("GET /api/me/sessions", s.user(s.listSessions))
	m.HandleFunc("DELETE /api/me/sessions/{id}", s.user(s.revokeSession))
	m.HandleFunc("GET /api/me/tokens", s.user(s.listTokens))
	m.HandleFunc("POST /api/me/tokens", s.user(s.createToken))
	m.HandleFunc("DELETE /api/me/tokens/{id}", s.user(s.deleteToken))
	m.HandleFunc("GET /api/users", s.admin(s.listUsers))
	m.HandleFunc("POST /api/users", s.admin(s.createUser))
	m.HandleFunc("PUT /api/users/{id}", s.admin(s.updateUser))
	m.HandleFunc("DELETE /api/users/{id}", s.admin(s.deleteUser))

	m.HandleFunc("GET /api/recipes", s.user(s.listRecipes))
	m.HandleFunc("GET /api/search", s.user(s.search))
	m.HandleFunc("POST /api/recipes", s.user(s.createRecipe))
	m.HandleFunc("GET /api/recipes/{id}", s.user(s.getRecipe))
	m.HandleFunc("PUT /api/recipes/{id}", s.user(s.updateRecipe))
	m.HandleFunc("DELETE /api/recipes/{id}", s.user(s.deleteRecipe))
	m.HandleFunc("POST /api/recipes/{id}/image", s.user(s.uploadImage))
	m.HandleFunc("DELETE /api/recipes/{id}/image", s.user(s.removeImage))
	m.HandleFunc("PUT /api/recipes/{id}/favorite", s.user(s.setFavorite))
	m.HandleFunc("POST /api/recipes/{id}/cooked", s.user(s.markCooked))
	m.HandleFunc("PUT /api/recipes/{id}/rating", s.user(s.setRating))
	m.HandleFunc("PUT /api/recipes/{id}/share", s.user(s.shareRecipe))
	m.HandleFunc("PUT /api/recipes/{id}/cookbooks", s.user(s.setRecipeCookbooks))
	m.HandleFunc("GET /api/recipes/{id}/versions", s.user(s.listVersions))
	m.HandleFunc("POST /api/recipes/{id}/versions/{version}/restore", s.user(s.restoreVersion))
	m.HandleFunc("GET /api/food-rules", s.user(s.listFoodRules))
	m.HandleFunc("PUT /api/food-rules", s.user(s.saveFoodRule))
	m.HandleFunc("DELETE /api/food-rules/{id}", s.user(s.deleteFoodRule))
	m.HandleFunc("POST /api/food-rules/suggest", s.user(s.suggestFoods))
	m.HandleFunc("GET /api/ingredient-lines", s.user(s.ingredientLines))
	m.HandleFunc("GET /api/tags", s.user(s.listTags))
	m.HandleFunc("PUT /api/tags", s.user(s.retag))

	m.HandleFunc("GET /api/cookbooks", s.user(s.listCookbooks))
	m.HandleFunc("POST /api/cookbooks", s.user(s.saveCookbook))
	m.HandleFunc("GET /api/cookbooks/{id}", s.user(s.getCookbook))
	m.HandleFunc("PUT /api/cookbooks/{id}", s.user(s.saveCookbook))
	m.HandleFunc("DELETE /api/cookbooks/{id}", s.user(s.deleteCookbook))
	m.HandleFunc("PUT /api/cookbooks/{id}/share", s.user(s.shareCookbook))

	m.HandleFunc("GET /api/shopping", s.user(s.listShopping))
	m.HandleFunc("POST /api/shopping", s.user(s.addShopping))
	m.HandleFunc("PUT /api/shopping/{id}", s.user(s.updateShopping))
	m.HandleFunc("DELETE /api/shopping/{id}", s.user(s.deleteShopping))
	m.HandleFunc("POST /api/shopping/clear", s.user(s.clearShopping))

	m.HandleFunc("POST /api/import/url", s.user(s.importURL))
	m.HandleFunc("POST /api/import/text", s.user(s.importText))
	m.HandleFunc("POST /api/import/photo", s.user(s.importPhoto))
	m.HandleFunc("POST /api/import/mealie", s.admin(s.importMealieUpload))
	m.HandleFunc("POST /api/import/mealie-api", s.admin(s.importMealieAPI))
	m.HandleFunc("GET /api/import/local", s.admin(s.listLocalImports))
	m.HandleFunc("POST /api/import/local", s.admin(s.importLocal))
	m.HandleFunc("GET /api/jobs/{id}", s.user(s.getJob))
	m.HandleFunc("GET /api/settings/ai", s.admin(s.getAI))
	m.HandleFunc("PUT /api/settings/ai", s.admin(s.setAI))
	m.HandleFunc("GET /api/export", s.admin(s.exportAll))

	m.HandleFunc("GET /api/public/recipe/{token}", s.open(s.publicRecipe))
	m.HandleFunc("GET /api/public/cookbook/{token}", s.open(s.publicCookbook))
}

// ------------------------------------------------------------------ auth

func (s *Server) state(w http.ResponseWriter, r *http.Request) error {
	s.writeJSON(w, r, 200, map[string]any{"setup": s.store.CountUsers() == 0, "version": s.cfg.Version})
	return nil
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._@-]{1,39}$`)

type credentials struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (c *credentials) validate() error {
	c.Username = strings.TrimSpace(c.Username)
	c.Name = clipStr(strings.TrimSpace(c.Name), 60)
	if !usernameRe.MatchString(c.Username) {
		return fail(400, "Usernames are 2–40 letters, numbers, dots or dashes")
	}
	if c.Name == "" {
		c.Name = c.Username
	}
	return validPassword(c.Password)
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) error {
	if err := s.cop.Check(r); err != nil {
		return fail(403, "Cross-site request blocked")
	}
	var c credentials
	if err := readJSON(w, r, &c); err != nil {
		return err
	}
	if err := c.validate(); err != nil {
		return err
	}
	u, err := s.store.CreateUser(c.Username, c.Name, hashPassword(c.Password), "admin", true)
	if err != nil {
		return fail(409, err.Error())
	}
	if err := s.startSession(w, r, u); err != nil {
		return err
	}
	return s.me(w, r, u)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	if err := s.cop.Check(r); err != nil {
		return fail(403, "Cross-site request blocked")
	}
	var c credentials
	if err := readJSON(w, r, &c); err != nil {
		return err
	}
	ip := s.clientIP(r)
	ipKey, userKey := "login-ip:"+ip, "login-user:"+strings.ToLower(strings.TrimSpace(c.Username))
	for key, max := range map[string]int{ipKey: 12, userKey: 8} {
		if over, wait := s.limit.over(key, max); over {
			w.Header().Set("Retry-After", itoa(int(wait.Seconds())+1))
			return fail(429, "Too many attempts. Try again in "+humanWait(wait)+".")
		}
	}
	u, err := s.store.UserByName(strings.TrimSpace(c.Username))
	hash := dummyHash
	if err == nil {
		hash = u.passHash
	}
	if !checkPassword(hash, c.Password) || err != nil {
		s.limit.hit(ipKey, 12, 10*time.Minute)
		s.limit.hit(userKey, 8, 10*time.Minute)
		return fail(401, "That username and password don't match")
	}
	s.limit.clear(userKey)
	if err := s.startSession(w, r, u); err != nil {
		return err
	}
	return s.me(w, r, u)
}

func humanWait(d time.Duration) string {
	if d < time.Minute {
		return "a minute"
	}
	return itoa(int(d.Minutes())+1) + " minutes"
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(cookieName); err == nil && s.cop.Check(r) == nil {
		s.store.Exec(`DELETE FROM sessions WHERE id=?`, hashToken(c.Value))
	}
	s.clearCookie(w, r)
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, u *User) error {
	s.writeJSON(w, r, 200, map[string]any{
		"user": u, "version": s.cfg.Version, "origin": s.origin(r),
		"features": map[string]bool{"ai": s.aiConfig().ready()},
	})
	return nil
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct{ Name string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if in.Name = clipStr(strings.TrimSpace(in.Name), 60); in.Name == "" {
		return fail(400, "Enter a name")
	}
	if err := s.store.Exec(`UPDATE users SET name=? WHERE id=?`, in.Name, u.ID); err != nil {
		return err
	}
	u.Name = in.Name
	return s.me(w, r, u)
}

func (s *Server) updatePrefs(w http.ResponseWriter, r *http.Request, u *User) error {
	var in map[string]any
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	b, _ := json.Marshal(in)
	if len(b) > 8000 {
		return fail(400, "Preferences are too large")
	}
	if err := s.store.Exec(`UPDATE users SET prefs=? WHERE id=?`, string(b), u.ID); err != nil {
		return err
	}
	u.Prefs = b
	return s.me(w, r, u)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct{ Current, Password string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if !s.limit.hit("pw:"+itoa(int(u.ID)), 8, 10*time.Minute) {
		return fail(429, "Too many attempts. Try again in a few minutes.")
	}
	if !checkPassword(u.passHash, in.Current) {
		return fail(403, "Your current password isn't right")
	}
	if err := validPassword(in.Password); err != nil {
		return err
	}
	if err := s.store.Exec(`UPDATE users SET pass_hash=? WHERE id=?`, hashPassword(in.Password), u.ID); err != nil {
		return err
	}
	// Sign out every other device.
	keep := ""
	if c, err := r.Cookie(cookieName); err == nil {
		keep = hashToken(c.Value)
	}
	s.store.Exec(`DELETE FROM sessions WHERE user_id=? AND id<>?`, u.ID, keep)
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request, u *User) error {
	cur := ""
	if c, err := r.Cookie(cookieName); err == nil {
		cur = hashToken(c.Value)
	}
	s.writeJSON(w, r, 200, s.store.Sessions(u.ID, cur))
	return nil
}

func (s *Server) revokeSession(w http.ResponseWriter, r *http.Request, u *User) error {
	id := r.PathValue("id")
	if len(id) != 16 {
		return errNotFound
	}
	s.store.Exec(`DELETE FROM sessions WHERE user_id=? AND substr(id,1,16)=?`, u.ID, id)
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request, u *User) error {
	rows, err := s.store.db.Query(`SELECT id, name, created_at, last_used FROM tokens WHERE user_id=? ORDER BY id`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, created, used int64
		var name string
		rows.Scan(&id, &name, &created, &used)
		out = append(out, map[string]any{"id": id, "name": name, "createdAt": created, "lastUsed": used})
	}
	s.writeJSON(w, r, 200, out)
	return nil
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct{ Name string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if in.Name = clipStr(strings.TrimSpace(in.Name), 60); in.Name == "" {
		in.Name = "Token"
	}
	token := "fond_" + randToken(30)
	if err := s.store.Exec(`INSERT INTO tokens (user_id, name, hash, created_at) VALUES (?,?,?,?)`, u.ID, in.Name, hashToken(token), now()); err != nil {
		return err
	}
	s.writeJSON(w, r, 201, map[string]string{"token": token, "name": in.Name})
	return nil
}

func (s *Server) deleteToken(w http.ResponseWriter, r *http.Request, u *User) error {
	s.store.Exec(`DELETE FROM tokens WHERE id=? AND user_id=?`, pathID(r, "id"), u.ID)
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, _ *User) error {
	us, err := s.store.Users()
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, us)
	return nil
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request, _ *User) error {
	var c credentials
	if err := readJSON(w, r, &c); err != nil {
		return err
	}
	if err := c.validate(); err != nil {
		return err
	}
	if c.Role != "admin" {
		c.Role = "member"
	}
	u, err := s.store.CreateUser(c.Username, c.Name, hashPassword(c.Password), c.Role, false)
	if err != nil {
		return fail(409, err.Error())
	}
	s.writeJSON(w, r, 201, u)
	return nil
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, me *User) error {
	id := pathID(r, "id")
	var in struct{ Name, Role, Password string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	target, err := s.store.User(id)
	if err != nil {
		return errNotFound
	}
	if in.Name = clipStr(strings.TrimSpace(in.Name), 60); in.Name != "" {
		s.store.Exec(`UPDATE users SET name=? WHERE id=?`, in.Name, id)
	}
	if in.Role == "admin" || in.Role == "member" {
		if target.ID == me.ID && in.Role != "admin" {
			return fail(400, "You can't remove your own admin role")
		}
		s.store.Exec(`UPDATE users SET role=? WHERE id=?`, in.Role, id)
	}
	if in.Password != "" {
		if err := validPassword(in.Password); err != nil {
			return err
		}
		s.store.Exec(`UPDATE users SET pass_hash=? WHERE id=?`, hashPassword(in.Password), id)
		s.store.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
	}
	u, _ := s.store.User(id)
	s.writeJSON(w, r, 200, u)
	return nil
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, me *User) error {
	id := pathID(r, "id")
	if id == me.ID {
		return fail(400, "You can't delete your own account")
	}
	if err := s.store.Exec(`DELETE FROM users WHERE id=?`, id); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

// ------------------------------------------------------------------ recipes

func (s *Server) listRecipes(w http.ResponseWriter, r *http.Request, u *User) error {
	cards, err := s.store.Cards(u.ID, `ORDER BY r.created_at DESC, r.id DESC`)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, cards)
	return nil
}

func (s *Server) search(w http.ResponseWriter, r *http.Request, _ *User) error {
	s.writeJSON(w, r, 200, s.store.Search(r.URL.Query().Get("q")))
	return nil
}

func (s *Server) getRecipe(w http.ResponseWriter, r *http.Request, u *User) error {
	rec, err := s.store.Recipe(pathID(r, "id"), u.ID)
	if err != nil {
		return errNotFound
	}
	s.writeJSON(w, r, 200, rec)
	return nil
}

func (s *Server) createRecipe(w http.ResponseWriter, r *http.Request, u *User) error {
	var rec model.Recipe
	if err := readJSON(w, r, &rec); err != nil {
		return err
	}
	rec.ID, rec.Image, rec.ImageColor, rec.CookedCount, rec.LastCooked, rec.CreatedAt, rec.UpdatedAt = 0, "", "", 0, 0, 0, 0
	if err := s.store.SaveRecipe(&rec, u.ID, ""); err != nil {
		return fail(400, err.Error())
	}
	out, err := s.store.Recipe(rec.ID, u.ID)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 201, out)
	return nil
}

func (s *Server) updateRecipe(w http.ResponseWriter, r *http.Request, u *User) error {
	var rec model.Recipe
	if err := readJSON(w, r, &rec); err != nil {
		return err
	}
	rec.ID = pathID(r, "id")
	if err := s.store.SaveRecipe(&rec, u.ID, ""); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return errNotFound
		}
		return fail(400, err.Error())
	}
	out, err := s.store.Recipe(rec.ID, u.ID)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, out)
	return nil
}

func (s *Server) deleteRecipe(w http.ResponseWriter, r *http.Request, _ *User) error {
	img, err := s.store.DeleteRecipe(pathID(r, "id"))
	if err != nil {
		return err
	}
	if img != "" {
		s.images.Delete(img)
	}
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) setRecipeImage(id int64, data []byte) (string, string, error) {
	key, color, err := s.images.Save(data)
	if err != nil {
		return "", "", err
	}
	var old string
	s.store.db.QueryRow(`SELECT image FROM recipes WHERE id=?`, id).Scan(&old)
	if err := s.store.Exec(`UPDATE recipes SET image=?, image_color=?, updated_at=? WHERE id=?`, key, color, now(), id); err != nil {
		s.images.Delete(key)
		return "", "", err
	}
	if old != "" {
		s.images.Delete(old)
	}
	return key, color, nil
}

func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request, u *User) error {
	id := pathID(r, "id")
	if _, err := s.store.Recipe(id, u.ID); err != nil {
		return errNotFound
	}
	r.Body = http.MaxBytesReader(w, r.Body, 40<<20)
	var data []byte
	var err error
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		f, _, ferr := r.FormFile("file")
		if ferr != nil {
			return fail(400, "No image was sent")
		}
		defer f.Close()
		data, err = io.ReadAll(f)
	} else {
		data, err = io.ReadAll(r.Body)
	}
	if err != nil {
		return fail(413, "That image is too large (40 MB max)")
	}
	key, color, err := s.setRecipeImage(id, data)
	if err != nil {
		return fail(400, err.Error())
	}
	s.writeJSON(w, r, 200, map[string]string{"image": key, "imageColor": color})
	return nil
}

func (s *Server) removeImage(w http.ResponseWriter, r *http.Request, _ *User) error {
	id := pathID(r, "id")
	var old string
	s.store.db.QueryRow(`SELECT image FROM recipes WHERE id=?`, id).Scan(&old)
	s.store.Exec(`UPDATE recipes SET image='', image_color='', updated_at=? WHERE id=?`, now(), id)
	if old != "" {
		s.images.Delete(old)
	}
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) setFavorite(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct{ On bool }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	id := pathID(r, "id")
	var err error
	if in.On {
		err = s.store.Exec(`INSERT OR IGNORE INTO favorites (user_id, recipe_id) SELECT ?, id FROM recipes WHERE id=?`, u.ID, id)
	} else {
		err = s.store.Exec(`DELETE FROM favorites WHERE user_id=? AND recipe_id=?`, u.ID, id)
	}
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]bool{"favorite": in.On})
	return nil
}

func (s *Server) markCooked(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct{ Undo bool }
	readJSON(w, r, &in)
	id := pathID(r, "id")
	if in.Undo {
		s.store.Exec(`UPDATE recipes SET cooked_count=MAX(cooked_count-1,0) WHERE id=?`, id)
	} else {
		s.store.Exec(`UPDATE recipes SET cooked_count=cooked_count+1, last_cooked=? WHERE id=?`, now(), id)
	}
	rec, err := s.store.Recipe(id, u.ID)
	if err != nil {
		return errNotFound
	}
	s.writeJSON(w, r, 200, map[string]any{"cookedCount": rec.CookedCount, "lastCooked": rec.LastCooked})
	return nil
}

func (s *Server) setRating(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct{ Rating float64 }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if in.Rating < 0 || in.Rating > 5 {
		return fail(400, "Ratings go from 0 to 5")
	}
	if err := s.store.Exec(`UPDATE recipes SET rating=? WHERE id=?`, in.Rating, pathID(r, "id")); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]float64{"rating": in.Rating})
	return nil
}

func (s *Server) shareRecipe(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct{ On bool }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	id := pathID(r, "id")
	token := ""
	if in.On {
		s.store.db.QueryRow(`SELECT COALESCE(share_token,'') FROM recipes WHERE id=?`, id).Scan(&token)
		if token == "" {
			token = randToken(16)
			if err := s.store.Exec(`UPDATE recipes SET share_token=? WHERE id=?`, token, id); err != nil {
				return err
			}
		}
	} else if err := s.store.Exec(`UPDATE recipes SET share_token=NULL WHERE id=?`, id); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]string{"shareToken": token})
	return nil
}

func (s *Server) setRecipeCookbooks(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct{ IDs []int64 }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	id := pathID(r, "id")
	want := map[int64]bool{}
	for _, cb := range in.IDs {
		want[cb] = true
	}
	books, err := s.store.Cookbooks()
	if err != nil {
		return err
	}
	for _, b := range books {
		if err := s.store.SetMembership(b.ID, id, want[b.ID]); err != nil {
			return err
		}
	}
	s.writeJSON(w, r, 200, map[string]any{"cookbooks": in.IDs})
	return nil
}

func (s *Server) listTags(w http.ResponseWriter, r *http.Request, _ *User) error {
	rows, err := s.store.db.Query(`SELECT tags FROM recipes WHERE tags<>'[]'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts := map[string]int{}
	display := map[string]string{}
	for rows.Next() {
		var raw string
		rows.Scan(&raw)
		var tags []string
		json.Unmarshal([]byte(raw), &tags)
		for _, t := range tags {
			k := strings.ToLower(t)
			counts[k]++
			if _, ok := display[k]; !ok {
				display[k] = t
			}
		}
	}
	type tag struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	out := make([]tag, 0, len(counts))
	for k, n := range counts {
		out = append(out, tag{display[k], n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	s.writeJSON(w, r, 200, out)
	return nil
}

func (s *Server) retag(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct{ From, To string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	in.From, in.To = strings.TrimSpace(in.From), clipStr(model.CleanLine(in.To), 40)
	if in.From == "" {
		return fail(400, "Pick a tag to change")
	}
	n, err := s.store.RetagAll(in.From, in.To)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]int{"changed": n})
	return nil
}

// ------------------------------------------------------------------ cookbooks

func (s *Server) listCookbooks(w http.ResponseWriter, r *http.Request, _ *User) error {
	books, err := s.store.Cookbooks()
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, books)
	return nil
}

func (s *Server) getCookbook(w http.ResponseWriter, r *http.Request, _ *User) error {
	c, err := s.store.Cookbook(pathID(r, "id"))
	if err != nil {
		return errNotFound
	}
	if r.URL.Query().Get("full") != "" {
		recipes := s.cookbookRecipes(c)
		s.writeJSON(w, r, 200, map[string]any{"cookbook": c, "recipes": recipes})
		return nil
	}
	s.writeJSON(w, r, 200, c)
	return nil
}

func (s *Server) saveCookbook(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct {
		Cookbook
		Entries *[]CookbookItem `json:"entries"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	c := in.Cookbook
	c.ID = 0
	if r.Method == http.MethodPut {
		c.ID = pathID(r, "id")
		if _, err := s.store.Cookbook(c.ID); err != nil {
			return errNotFound
		}
	}
	var entries []CookbookItem
	if in.Entries != nil {
		entries = *in.Entries
		if entries == nil {
			entries = []CookbookItem{}
		}
	}
	if err := s.store.SaveCookbook(&c, entries); err != nil {
		return fail(400, err.Error())
	}
	out, err := s.store.Cookbook(c.ID)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, out)
	return nil
}

func (s *Server) deleteCookbook(w http.ResponseWriter, r *http.Request, _ *User) error {
	if err := s.store.Exec(`DELETE FROM cookbooks WHERE id=?`, pathID(r, "id")); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) shareCookbook(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct{ On bool }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	id := pathID(r, "id")
	token := ""
	if in.On {
		s.store.db.QueryRow(`SELECT COALESCE(share_token,'') FROM cookbooks WHERE id=?`, id).Scan(&token)
		if token == "" {
			token = randToken(16)
			if err := s.store.Exec(`UPDATE cookbooks SET share_token=? WHERE id=?`, token, id); err != nil {
				return err
			}
		}
	} else if err := s.store.Exec(`UPDATE cookbooks SET share_token=NULL WHERE id=?`, id); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]string{"shareToken": token})
	return nil
}

// ------------------------------------------------------------------ shopping

func (s *Server) listShopping(w http.ResponseWriter, r *http.Request, _ *User) error {
	s.writeJSON(w, r, 200, s.store.Shopping())
	return nil
}

func (s *Server) addShopping(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct {
		Items []ShopItem `json:"items"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if len(in.Items) > 300 {
		return fail(400, "Too many items at once")
	}
	for _, it := range in.Items {
		if t := clipStr(model.CleanLine(it.Text), 300); t != "" {
			s.store.Exec(`INSERT INTO shopping (text, recipe_id, recipe_title, created_at) VALUES (?,?,?,?)`, t, it.RecipeID, clipStr(it.RecipeTitle, 200), now())
		}
	}
	return s.listShopping(w, r, nil)
}

func (s *Server) updateShopping(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct {
		Text    *string `json:"text"`
		Checked *bool   `json:"checked"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	id := pathID(r, "id")
	if in.Text != nil {
		if t := clipStr(model.CleanLine(*in.Text), 300); t != "" {
			s.store.Exec(`UPDATE shopping SET text=? WHERE id=?`, t, id)
		}
	}
	if in.Checked != nil {
		s.store.Exec(`UPDATE shopping SET checked=? WHERE id=?`, *in.Checked, id)
	}
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) deleteShopping(w http.ResponseWriter, r *http.Request, _ *User) error {
	s.store.Exec(`DELETE FROM shopping WHERE id=?`, pathID(r, "id"))
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) clearShopping(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct{ All bool }
	readJSON(w, r, &in)
	if in.All {
		s.store.Exec(`DELETE FROM shopping`)
	} else {
		s.store.Exec(`DELETE FROM shopping WHERE checked=1`)
	}
	return s.listShopping(w, r, nil)
}

// ------------------------------------------------------------------ public sharing

func publicView(rec *model.Recipe) *model.Recipe {
	out := *rec
	out.Favorite, out.CookedCount, out.LastCooked, out.Rating, out.Cookbooks, out.Version = false, 0, 0, 0, nil, 0
	return &out
}

func (s *Server) publicLimit(r *http.Request) error {
	if !s.limit.hit("pub:"+s.clientIP(r), 240, time.Minute) {
		return fail(429, "Too many requests")
	}
	return nil
}

func (s *Server) publicRecipe(w http.ResponseWriter, r *http.Request) error {
	if err := s.publicLimit(r); err != nil {
		return err
	}
	rec, err := s.store.RecipeByShare(r.PathValue("token"))
	if err != nil {
		return errNotFound
	}
	// A shared recipe is weighed the way its household weighs it.
	b, _ := json.Marshal(publicView(rec))
	out := map[string]any{}
	json.Unmarshal(b, &out)
	out["foodRules"], _ = s.store.FoodRules()
	s.writeJSON(w, r, 200, out)
	return nil
}

func (s *Server) cookbookRecipes(c *Cookbook) []*model.Recipe {
	out := []*model.Recipe{}
	for _, e := range c.Entries {
		if rec, err := scanRecipe(s.store.db.QueryRow(`SELECT `+recipeCols+` FROM recipes WHERE id=?`, e.RecipeID)); err == nil {
			rec = publicView(rec)
			rec.ShareToken = ""
			out = append(out, rec)
		}
	}
	return out
}

func (s *Server) publicCookbook(w http.ResponseWriter, r *http.Request) error {
	if err := s.publicLimit(r); err != nil {
		return err
	}
	c, err := s.store.CookbookByShare(r.PathValue("token"))
	if err != nil {
		return errNotFound
	}
	rules, _ := s.store.FoodRules()
	s.writeJSON(w, r, 200, map[string]any{"cookbook": c, "recipes": s.cookbookRecipes(c), "foodRules": rules})
	return nil
}

func metaTags(title, desc, image, url string) string {
	var b strings.Builder
	e := html.EscapeString
	b.WriteString(`<meta property="og:type" content="article"><meta property="og:title" content="` + e(title) + `">`)
	b.WriteString(`<meta name="twitter:card" content="summary_large_image"><meta property="og:url" content="` + e(url) + `">`)
	if desc != "" {
		b.WriteString(`<meta property="og:description" content="` + e(clipStr(desc, 280)) + `"><meta name="description" content="` + e(clipStr(desc, 280)) + `">`)
	}
	if image != "" {
		b.WriteString(`<meta property="og:image" content="` + e(image) + `">`)
	}
	return b.String()
}

func (s *Server) sharedRecipePage(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.RecipeByShare(r.PathValue("token"))
	if err != nil || s.publicLimit(r) != nil {
		s.serveIndex(w, r, "")
		return
	}
	origin := s.origin(r)
	img := ""
	if rec.Image != "" {
		img = origin + "/img/" + rec.Image + ".jpg"
	}
	head := metaTags(rec.Title, rec.Description, img, origin+r.URL.Path)
	// schema.org markup lets other recipe apps import a shared Fond recipe.
	ld := map[string]any{"@context": "https://schema.org", "@type": "Recipe", "name": rec.Title, "description": rec.Description}
	if img != "" {
		ld["image"] = img
	}
	if rec.Yield != "" {
		ld["recipeYield"] = rec.Yield
	}
	if rec.Author != "" {
		ld["author"] = map[string]string{"@type": "Person", "name": rec.Author}
	}
	for k, v := range map[string]int{"prepTime": rec.PrepMin, "cookTime": rec.CookMin, "totalTime": rec.TotalMin} {
		if v > 0 {
			ld[k] = "PT" + itoa(v) + "M"
		}
	}
	var ings []string
	for _, l := range rec.Ingredients {
		if !l.H {
			ings = append(ings, l.Text)
		}
	}
	var steps []map[string]string
	for _, l := range rec.Steps {
		if !l.H {
			steps = append(steps, map[string]string{"@type": "HowToStep", "text": l.Text})
		}
	}
	ld["recipeIngredient"], ld["recipeInstructions"] = ings, steps
	if len(rec.Tags) > 0 {
		ld["keywords"] = strings.Join(rec.Tags, ", ")
	}
	b, _ := json.Marshal(ld)
	head += `<script type="application/ld+json">` + strings.ReplaceAll(string(b), "</", `<\/`) + `</script>`
	s.serveIndex(w, r, head)
}

func (s *Server) sharedCookbookPage(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.CookbookByShare(r.PathValue("token"))
	if err != nil || s.publicLimit(r) != nil {
		s.serveIndex(w, r, "")
		return
	}
	origin := s.origin(r)
	desc := c.Subtitle
	if desc == "" {
		desc = c.Description
	}
	img := ""
	var cover string
	s.store.db.QueryRow(`SELECT r.image FROM cookbook_recipes cr JOIN recipes r ON r.id=cr.recipe_id WHERE cr.cookbook_id=? AND r.image<>'' ORDER BY cr.position LIMIT 1`, c.ID).Scan(&cover)
	if cover != "" {
		img = origin + "/img/" + cover + ".jpg"
	}
	s.serveIndex(w, r, metaTags(c.Title, desc, img, origin+r.URL.Path))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
