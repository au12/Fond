package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/au12/fond/internal/mealie"
	"github.com/au12/fond/internal/model"
	"github.com/au12/fond/internal/scrape"
)

// ------------------------------------------------------------------ link / text / photo import

func (s *Server) importLimit(u *User) error {
	if !s.limit.hit("import:"+itoa(int(u.ID)), 40, time.Minute) {
		return fail(429, "That's a lot of imports at once. Give it a minute.")
	}
	return nil
}

func (s *Server) finishImport(w http.ResponseWriter, r *http.Request, u *User, rec *model.Recipe, imageURL, method string) error {
	rec.ID = 0
	if err := s.store.SaveRecipe(rec, u.ID, ""); err != nil {
		return fail(422, err.Error())
	}
	if imageURL != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		if data, err := s.public.Bytes(ctx, imageURL, 30<<20); err == nil {
			if _, _, err := s.setRecipeImage(rec.ID, data); err != nil {
				log.Printf("import: image for %q skipped: %v", rec.Title, err)
			}
		}
		cancel()
	}
	out, err := s.store.Recipe(rec.ID, u.ID)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 201, map[string]any{"recipe": out, "method": method})
	return nil
}

func (s *Server) importURL(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct {
		URL   string `json:"url"`
		HTML  string `json:"html"`
		Clip  string `json:"clip"`
		Force bool   `json:"force"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if err := s.importLimit(u); err != nil {
		return err
	}
	target, err := scrape.CleanURL(in.URL)
	if err != nil {
		return fail(400, capitalize(err.Error()))
	}
	link := target.String()
	findDup := func(urls ...string) (map[string]any, bool) {
		for _, x := range urls {
			var id int64
			var slug, title string
			if s.store.db.QueryRow(`SELECT id, slug, title FROM recipes WHERE source_url=? LIMIT 1`, x).Scan(&id, &slug, &title) == nil {
				return map[string]any{"duplicate": true, "recipe": map[string]any{"id": id, "slug": slug, "title": title}}, true
			}
		}
		return nil, false
	}
	if !in.Force {
		if dup, ok := findDup(link, in.URL); ok {
			s.writeJSON(w, r, 200, dup)
			return nil
		}
	}
	page := []byte(in.HTML)
	if in.Clip != "" {
		if c, ok := s.clips.LoadAndDelete(in.Clip); ok {
			page = c.(*clip).html
		}
	}
	if len(page) == 0 {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		var final string
		page, final, err = s.public.Page(ctx, link)
		if err != nil {
			if errors.Is(err, scrape.ErrBlockedAddress) {
				return fail(400, "That link points to a private network address, which Fond doesn't fetch")
			}
			if strings.Contains(err.Error(), "HTTP 403") || strings.Contains(err.Error(), "HTTP 429") || strings.Contains(err.Error(), "HTTP 401") {
				return fail(502, "That site turns away automated requests. Open the page in your browser and use the “Save to Fond” bookmark or shortcut, which sends the page you already loaded. Or paste the recipe text.")
			}
			return fail(502, "Couldn't open that page: "+trimErr(err))
		}
		if final != "" {
			link = final
		}
	}
	res, err := scrape.Extract(page, link)
	method := ""
	if res != nil {
		method = res.Method
	}
	if err != nil && res != nil && s.aiConfig().ready() && len(res.Text) > 200 {
		if rec, aerr := s.aiExtract(r.Context(), "This is the text of a web page. Extract the recipe on it.\n\n"+res.Text, nil); aerr == nil {
			rec.SourceURL, rec.SourceName = res.Recipe.SourceURL, res.Recipe.SourceName
			if rec.Title == "" {
				rec.Title = res.Recipe.Title
			}
			res.Recipe, err, method = *rec, nil, "ai"
		}
	}
	if err != nil {
		draft := map[string]string{"sourceUrl": link}
		if res != nil {
			draft["title"], draft["description"] = res.Recipe.Title, res.Recipe.Description
		}
		s.writeJSON(w, r, 422, map[string]any{"error": "No recipe was found on that page. You can still save the link and fill it in yourself.", "draft": draft})
		return nil
	}
	if !in.Force {
		if dup, ok := findDup(res.Recipe.SourceURL); ok {
			s.writeJSON(w, r, 200, dup)
			return nil
		}
	}
	return s.finishImport(w, r, u, &res.Recipe, res.ImageURL, method)
}

func trimErr(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 && i < len(msg)-2 {
		msg = msg[i+2:]
	}
	return msg
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (s *Server) importText(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct {
		Text string `json:"text"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if err := s.importLimit(u); err != nil {
		return err
	}
	if len(strings.TrimSpace(in.Text)) < 20 {
		return fail(400, "Paste the whole recipe: a title, the ingredients and the steps")
	}
	rec, err := scrape.FromText(in.Text)
	method := "text"
	if s.aiConfig().ready() && (err != nil || len(rec.Ingredients) == 0 || len(rec.Steps) == 0) {
		if arec, aerr := s.aiExtract(r.Context(), "Extract the recipe from this text.\n\n"+clipStr(in.Text, 24000), nil); aerr == nil {
			rec, err, method = arec, nil, "ai"
		}
	}
	if err != nil {
		return fail(422, "Couldn't find a recipe in that text. Put the title on the first line, then an “Ingredients” line and an “Instructions” line.")
	}
	return s.finishImport(w, r, u, rec, "", method)
}

func (s *Server) importPhoto(w http.ResponseWriter, r *http.Request, u *User) error {
	if !s.aiConfig().ready() {
		return fail(400, "Reading recipes from photos needs an AI model. An admin can connect one in Settings.")
	}
	if err := s.importLimit(u); err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, 40<<20)
	f, _, err := r.FormFile("file")
	if err != nil {
		return fail(400, "No photo was sent")
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return fail(413, "That photo is too large")
	}
	// Normalise through the image store (orientation, size), then discard.
	key, _, err := s.images.Save(data)
	if err != nil {
		return fail(400, err.Error())
	}
	jpg, _ := os.ReadFile(filepath.Join(s.images.Dir, key+".jpg"))
	s.images.Delete(key)
	rec, err := s.aiExtract(r.Context(), "This photo shows a recipe (a cookbook page, a card or a screenshot). Transcribe it.", jpg)
	if err != nil {
		return fail(422, "The AI model couldn't read a recipe from that photo: "+trimErr(err))
	}
	return s.finishImport(w, r, u, rec, "", "ai")
}

// A clip is a page the user's own browser already loaded, handed over by the
// bookmarklet. It waits here briefly until the signed-in app collects it.
type clip struct {
	html    []byte
	expires time.Time
}

// receiveClip accepts the bookmarklet's cross-site form post. It is
// deliberately unauthenticated (the session cookie is not sent on a cross-site
// POST), so it only parks the page in memory, tightly bounded, and redirects
// to the app, where the signed-in user decides what happens to it.
func (s *Server) receiveClip(w http.ResponseWriter, r *http.Request) {
	if !s.limit.hit("clip:"+s.clientIP(r), 12, time.Minute) {
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "That page is too large to send", http.StatusRequestEntityTooLarge)
		return
	}
	target := "/share?url=" + url.QueryEscape(r.PostFormValue("url"))
	if page := r.PostFormValue("html"); page != "" {
		n := 0
		s.clips.Range(func(k, v any) bool {
			if time.Now().After(v.(*clip).expires) {
				s.clips.Delete(k)
			} else {
				n++
			}
			return true
		})
		if n < 24 {
			id := randToken(12)
			s.clips.Store(id, &clip{html: []byte(page), expires: time.Now().Add(3 * time.Minute)})
			target += "&clip=" + id
		}
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// ------------------------------------------------------------------ optional AI

type aiConfig struct {
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	Key     string `json:"apiKey"`
}

func (c aiConfig) ready() bool { return c.BaseURL != "" && c.Model != "" }

func (s *Server) aiConfig() aiConfig {
	c := aiConfig{BaseURL: s.cfg.AIBase, Model: s.cfg.AIModel, Key: s.cfg.AIKey}
	if raw := s.store.Setting("ai"); raw != "" {
		var saved aiConfig
		if json.Unmarshal([]byte(raw), &saved) == nil && saved.BaseURL != "" {
			c = saved
		}
	}
	return c
}

func (s *Server) getAI(w http.ResponseWriter, r *http.Request, _ *User) error {
	c := s.aiConfig()
	s.writeJSON(w, r, 200, map[string]any{"baseUrl": c.BaseURL, "model": c.Model, "hasKey": c.Key != ""})
	return nil
}

func (s *Server) setAI(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct {
		aiConfig
		KeepKey bool `json:"keepKey"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	c := in.aiConfig
	c.BaseURL, c.Model = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/"), strings.TrimSpace(c.Model)
	if in.KeepKey {
		c.Key = s.aiConfig().Key
	}
	if c.BaseURL == "" {
		s.store.SetSetting("ai", "")
	} else {
		if _, err := scrape.CleanURL(c.BaseURL); err != nil {
			return fail(400, "Enter the address of an OpenAI-compatible API, like http://ollama:11434/v1")
		}
		b, _ := json.Marshal(c)
		s.store.SetSetting("ai", string(b))
	}
	return s.getAI(w, r, nil)
}

const aiSystem = `You extract recipes. Reply with ONE JSON object and nothing else, using exactly these keys:
{"title": string, "description": string, "yield": string, "servings": number, "prepMinutes": number, "cookMinutes": number, "totalMinutes": number,
 "ingredients": [string], "steps": [string], "notes": string, "tags": [string]}
Rules: copy quantities and wording faithfully and never invent ingredients. One ingredient per array item, written as "quantity unit name, preparation".
One step per array item, without numbering. For a section heading inside ingredients or steps (like "For the sauce"), add an item that starts with "# ".
Use 0 or "" when something is unknown. At most 4 short tags (course or cuisine). If there is no recipe, reply {"title": ""}.`

func (s *Server) aiExtract(ctx context.Context, prompt string, jpeg []byte) (*model.Recipe, error) {
	c := s.aiConfig()
	var content any = prompt
	if jpeg != nil {
		content = []map[string]any{
			{"type": "text", "text": prompt},
			{"type": "image_url", "image_url": map[string]string{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg)}},
		}
	}
	body, _ := json.Marshal(map[string]any{
		"model": c.Model, "temperature": 0, "stream": false,
		"messages": []map[string]any{{"role": "system", "content": aiSystem}, {"role": "user", "content": content}},
	})
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	res, err := (&http.Client{Timeout: 185 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("the AI endpoint answered HTTP %d", res.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &out) != nil || len(out.Choices) == 0 {
		return nil, errors.New("the AI endpoint sent an unexpected reply")
	}
	text := out.Choices[0].Message.Content
	if i, j := strings.Index(text, "{"), strings.LastIndex(text, "}"); i >= 0 && j > i {
		text = text[i : j+1]
	}
	var got struct {
		Title, Description, Yield, Notes       string
		Servings                               float64
		PrepMinutes, CookMinutes, TotalMinutes float64
		Ingredients, Steps, Tags               []string
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		return nil, errors.New("the AI model didn't return valid JSON")
	}
	rec := &model.Recipe{Title: got.Title, Description: got.Description, Yield: got.Yield, Servings: got.Servings, Notes: got.Notes, Tags: got.Tags,
		PrepMin: int(got.PrepMinutes), CookMin: int(got.CookMinutes), TotalMin: int(got.TotalMinutes)}
	lines := func(in []string) []model.Line {
		var out []model.Line
		for _, x := range in {
			x = strings.TrimSpace(x)
			if h := strings.HasPrefix(x, "#"); x != "" {
				out = append(out, model.Line{Text: strings.TrimSpace(strings.TrimLeft(x, "# ")), H: h})
			}
		}
		return out
	}
	rec.Ingredients, rec.Steps = lines(got.Ingredients), lines(got.Steps)
	rec.Normalize()
	if rec.Title == "" || len(rec.Ingredients)+len(rec.Steps) == 0 {
		return nil, scrape.ErrNoRecipe
	}
	return rec, nil
}

// ------------------------------------------------------------------ Mealie migration jobs

// Job tracks a long-running import so the client can show progress.
type Job struct {
	mu        sync.Mutex
	ID        string   `json:"id"`
	State     string   `json:"state"` // running | done | failed
	Done      int      `json:"done"`
	Total     int      `json:"total"`
	Current   string   `json:"current"`
	Imported  int      `json:"imported"`
	Skipped   int      `json:"skipped"`
	Failed    int      `json:"failed"`
	Cookbooks int      `json:"cookbooks"`
	Error     string   `json:"error,omitempty"`
	Problems  []string `json:"problems"`

	s      *Server
	userID int64
	ids    map[string]int64
}

func (j *Job) Progress(done, total int, current string) {
	j.mu.Lock()
	j.Done, j.Total, j.Current = done, total, current
	j.mu.Unlock()
}

func (j *Job) problem(msg string) {
	j.Failed++
	if len(j.Problems) < 50 {
		j.Problems = append(j.Problems, msg)
	}
}

func (j *Job) Recipe(it mealie.Item) error {
	origin := "mealie:" + it.MealieID
	var existing int64
	if it.MealieID != "" && j.s.store.db.QueryRow(`SELECT id FROM recipes WHERE origin=?`, origin).Scan(&existing) == nil {
		j.mu.Lock()
		j.ids[it.MealieID] = existing
		j.Skipped++
		j.mu.Unlock()
		return nil
	}
	rec := it.Recipe
	if it.Image != nil {
		if key, color, err := j.s.images.Save(it.Image); err == nil {
			rec.Image, rec.ImageColor = key, color
		}
	}
	err := j.s.store.SaveRecipe(&rec, j.userID, origin)
	j.mu.Lock()
	defer j.mu.Unlock()
	if err != nil {
		if rec.Image != "" {
			j.s.images.Delete(rec.Image)
		}
		j.problem(fmt.Sprintf("%s: %v", it.Recipe.Title, err))
		return nil
	}
	j.Imported++
	j.ids[it.MealieID] = rec.ID
	if it.Favorite {
		j.s.store.Exec(`INSERT OR IGNORE INTO favorites (user_id, recipe_id) VALUES (?,?)`, j.userID, rec.ID)
	}
	return nil
}

func (j *Job) Cookbook(b mealie.Book) {
	var n int
	j.s.store.db.QueryRow(`SELECT COUNT(*) FROM cookbooks WHERE title=?`, b.Name).Scan(&n)
	if n > 0 {
		return
	}
	entries := []CookbookItem{}
	j.mu.Lock()
	for _, id := range b.RecipeIDs {
		if rid, ok := j.ids[id]; ok {
			entries = append(entries, CookbookItem{RecipeID: rid})
		}
	}
	j.mu.Unlock()
	c := &Cookbook{Title: b.Name, Description: b.Description}
	if err := j.s.store.SaveCookbook(c, entries); err == nil {
		j.mu.Lock()
		j.Cookbooks++
		j.mu.Unlock()
	}
}

func (s *Server) startJob(u *User, run func(*Job) error) *Job {
	j := &Job{ID: randToken(9), State: "running", Problems: []string{}, s: s, userID: u.ID, ids: map[string]int64{}}
	s.jobs.Store(j.ID, j)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				j.mu.Lock()
				j.State, j.Error = "failed", fmt.Sprint(rec)
				j.mu.Unlock()
			}
		}()
		err := run(j)
		j.mu.Lock()
		if err != nil {
			j.State, j.Error = "failed", capitalize(err.Error())
		} else {
			j.State = "done"
		}
		j.mu.Unlock()
		time.AfterFunc(2*time.Hour, func() { s.jobs.Delete(j.ID) })
	}()
	return j
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request, _ *User) error {
	v, ok := s.jobs.Load(r.PathValue("id"))
	if !ok {
		return errNotFound
	}
	j := v.(*Job)
	j.mu.Lock()
	defer j.mu.Unlock()
	s.writeJSON(w, r, 200, j)
	return nil
}

func (s *Server) importMealieUpload(w http.ResponseWriter, r *http.Request, u *User) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<30)
	mr, err := r.MultipartReader()
	if err != nil {
		return fail(400, "Send the zip as a file upload")
	}
	tmp, err := os.CreateTemp(filepath.Join(s.cfg.DataDir, "tmp"), "mealie-*.zip")
	if err != nil {
		return err
	}
	found := false
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		if part.FormName() == "file" {
			if _, err := io.Copy(tmp, part); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return fail(400, "The upload was interrupted")
			}
			found = true
		}
		part.Close()
	}
	tmp.Close()
	if !found {
		os.Remove(tmp.Name())
		return fail(400, "No file was sent")
	}
	j := s.startJob(u, func(j *Job) error {
		defer os.Remove(tmp.Name())
		return mealie.ImportZip(tmp.Name(), j)
	})
	s.writeJSON(w, r, 202, map[string]string{"job": j.ID})
	return nil
}

func (s *Server) listLocalImports(w http.ResponseWriter, r *http.Request, _ *User) error {
	out := []map[string]any{}
	entries, _ := os.ReadDir(filepath.Join(s.cfg.DataDir, "import"))
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			out = append(out, map[string]any{"name": e.Name(), "size": info.Size()})
		}
	}
	s.writeJSON(w, r, 200, out)
	return nil
}

func (s *Server) importLocal(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct {
		File string `json:"file"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	name := filepath.Base(in.File)
	full := filepath.Join(s.cfg.DataDir, "import", name)
	if st, err := os.Stat(full); err != nil || st.IsDir() || name != in.File {
		return errNotFound
	}
	j := s.startJob(u, func(j *Job) error { return mealie.ImportZip(full, j) })
	s.writeJSON(w, r, 202, map[string]string{"job": j.ID})
	return nil
}

func (s *Server) importMealieAPI(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.URL) == "" || strings.TrimSpace(in.Token) == "" {
		return fail(400, "Enter your Mealie address and an API token")
	}
	j := s.startJob(u, func(j *Job) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		return mealie.ImportAPI(ctx, s.private, in.URL, in.Token, j)
	})
	s.writeJSON(w, r, 202, map[string]string{"job": j.ID})
	return nil
}

// exportAll streams every recipe and photo as one zip — a portable backup.
func (s *Server) exportAll(w http.ResponseWriter, r *http.Request, u *User) error {
	rows, err := s.store.db.Query(`SELECT ` + recipeCols + ` FROM recipes ORDER BY id`)
	if err != nil {
		return err
	}
	var recipes []*model.Recipe
	for rows.Next() {
		if rec, err := scanRecipe(rows); err == nil {
			rec.ShareToken = ""
			recipes = append(recipes, rec)
		}
	}
	rows.Close()
	books, _ := s.store.Cookbooks()
	for _, b := range books {
		s.store.cookbookEntries(b)
		b.ShareToken = ""
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="fond-export-`+time.Now().Format("2006-01-02")+`.zip"`)
	zw := zip.NewWriter(w)
	defer zw.Close()
	f, _ := zw.Create("fond.json")
	json.NewEncoder(f).Encode(map[string]any{"version": 1, "exportedAt": now(), "recipes": recipes, "cookbooks": books})
	for _, rec := range recipes {
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
