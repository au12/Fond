package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/au12/fond/internal/model"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  name TEXT NOT NULL DEFAULT '',
  pass_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'member',
  prefs TEXT NOT NULL DEFAULT '{}',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL,
  last_seen INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  ua TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS sessions_user ON sessions(user_id);
CREATE TABLE IF NOT EXISTS tokens (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  hash TEXT NOT NULL UNIQUE,
  created_at INTEGER NOT NULL,
  last_used INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS recipes (
  id INTEGER PRIMARY KEY,
  slug TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  image TEXT NOT NULL DEFAULT '',
  image_color TEXT NOT NULL DEFAULT '',
  source_url TEXT NOT NULL DEFAULT '',
  source_name TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  yield TEXT NOT NULL DEFAULT '',
  servings REAL NOT NULL DEFAULT 0,
  prep_min INTEGER NOT NULL DEFAULT 0,
  cook_min INTEGER NOT NULL DEFAULT 0,
  total_min INTEGER NOT NULL DEFAULT 0,
  ingredients TEXT NOT NULL DEFAULT '[]',
  steps TEXT NOT NULL DEFAULT '[]',
  notes TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '[]',
  nutrition TEXT NOT NULL DEFAULT '{}',
  extra TEXT NOT NULL DEFAULT '',
  rating REAL NOT NULL DEFAULT 0,
  cooked_count INTEGER NOT NULL DEFAULT 0,
  last_cooked INTEGER NOT NULL DEFAULT 0,
  share_token TEXT UNIQUE,
  origin TEXT NOT NULL DEFAULT '',
  created_by INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS recipes_source ON recipes(source_url);
CREATE INDEX IF NOT EXISTS recipes_origin ON recipes(origin);
CREATE TABLE IF NOT EXISTS favorites (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipe_id INTEGER NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, recipe_id)
);
CREATE TABLE IF NOT EXISTS cookbooks (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL,
  subtitle TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  color TEXT NOT NULL DEFAULT 'bay',
  style TEXT NOT NULL DEFAULT 'classic',
  chapters TEXT NOT NULL DEFAULT '[]',
  share_token TEXT UNIQUE,
  position INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS cookbook_recipes (
  cookbook_id INTEGER NOT NULL REFERENCES cookbooks(id) ON DELETE CASCADE,
  recipe_id INTEGER NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
  chapter TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (cookbook_id, recipe_id)
);
CREATE INDEX IF NOT EXISTS cookbook_recipes_recipe ON cookbook_recipes(recipe_id);
CREATE TABLE IF NOT EXISTS shopping (
  id INTEGER PRIMARY KEY,
  text TEXT NOT NULL,
  checked INTEGER NOT NULL DEFAULT 0,
  recipe_id INTEGER REFERENCES recipes(id) ON DELETE SET NULL,
  recipe_title TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS recipe_versions (
  recipe_id INTEGER NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
  version INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  yield TEXT NOT NULL DEFAULT '',
  servings REAL NOT NULL DEFAULT 0,
  ingredients TEXT NOT NULL DEFAULT '[]',
  steps TEXT NOT NULL DEFAULT '[]',
  note TEXT NOT NULL DEFAULT '',
  user_id INTEGER,
  user_name TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  PRIMARY KEY (recipe_id, version)
);
CREATE TABLE IF NOT EXISTS food_rules (
  id INTEGER PRIMARY KEY,
  phrase TEXT NOT NULL UNIQUE COLLATE NOCASE,
  action TEXT NOT NULL,
  food_id TEXT NOT NULL DEFAULT '',
  g_per_ml REAL NOT NULL DEFAULT 0,
  g_each REAL NOT NULL DEFAULT 0,
  source TEXT NOT NULL DEFAULT 'user',
  user_id INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
`

// Store wraps the SQLite database.
type Store struct {
	db  *sql.DB
	fts bool
}

func now() int64 { return time.Now().Unix() }

// OpenStore opens (and migrates) the database in dir.
func OpenStore(dir string) (*Store, error) {
	dsn := "file:" + filepath.Join(dir, "fond.db") +
		"?_pragma=busy_timeout(8000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_pragma=temp_store(MEMORY)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	_, err = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS recipes_fts USING fts5(title, tags, body, tokenize='unicode61 remove_diacritics 2', prefix='2 3')`)
	s.fts = err == nil
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// migrate brings a database made by an earlier release up to date.
func (s *Store) migrate() error {
	if !s.hasColumn("recipes", "version") {
		if _, err := s.db.Exec(`ALTER TABLE recipes ADD COLUMN version INTEGER NOT NULL DEFAULT 1`); err != nil {
			return err
		}
	}
	// Every recipe has a history, starting with the recipe as it stands.
	_, err := s.db.Exec(`INSERT INTO recipe_versions (recipe_id, version, title, yield, servings, ingredients, steps, note, user_id, user_name, created_at)
		SELECT r.id, r.version, r.title, r.yield, r.servings, r.ingredients, r.steps, '', r.created_by, COALESCE(u.name, ''), r.updated_at
		FROM recipes r LEFT JOIN users u ON u.id = r.created_by
		WHERE NOT EXISTS (SELECT 1 FROM recipe_versions v WHERE v.recipe_id = r.id)`)
	return err
}

func (s *Store) hasColumn(table, column string) bool {
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil && name == column {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------------ users

type User struct {
	ID       int64           `json:"id"`
	Username string          `json:"username"`
	Name     string          `json:"name"`
	Role     string          `json:"role"`
	Prefs    json.RawMessage `json:"prefs"`
	Created  int64           `json:"createdAt"`
	passHash string
}

const userCols = `id, username, name, pass_hash, role, prefs, created_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	u := &User{}
	var prefs string
	if err := row.Scan(&u.ID, &u.Username, &u.Name, &u.passHash, &u.Role, &prefs, &u.Created); err != nil {
		return nil, err
	}
	u.Prefs = json.RawMessage(prefs)
	return u, nil
}

func (s *Store) CountUsers() (n int) {
	s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return
}

func (s *Store) CreateUser(username, name, hash, role string, onlyIfEmpty bool) (*User, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if onlyIfEmpty {
		var n int
		tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
		if n > 0 {
			return nil, errors.New("setup has already been completed")
		}
	}
	res, err := tx.Exec(`INSERT INTO users (username, name, pass_hash, role, created_at) VALUES (?,?,?,?,?)`, username, name, hash, role, now())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, errors.New("that username is already taken")
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.User(id)
}

func (s *Store) User(id int64) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id=?`, id))
}

func (s *Store) UserByName(name string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE username=?`, name))
}

func (s *Store) Users() ([]*User, error) {
	rows, err := s.db.Query(`SELECT ` + userCols + ` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		if u, err := scanUser(rows); err == nil {
			out = append(out, u)
		}
	}
	return out, nil
}

func (s *Store) Exec(q string, args ...any) error {
	_, err := s.db.Exec(q, args...)
	return err
}

// ------------------------------------------------------------------ sessions & tokens

type Session struct {
	ID       string `json:"-"`
	Created  int64  `json:"createdAt"`
	LastSeen int64  `json:"lastSeen"`
	UA       string `json:"userAgent"`
	IP       string `json:"ip"`
	Current  bool   `json:"current"`
	Handle   string `json:"id"`
}

func (s *Store) CreateSession(hash string, userID int64, ttl time.Duration, ua, ip string) error {
	t := now()
	return s.Exec(`INSERT INTO sessions (id, user_id, created_at, last_seen, expires_at, ua, ip) VALUES (?,?,?,?,?,?,?)`,
		hash, userID, t, t, t+int64(ttl.Seconds()), clipStr(ua, 300), ip)
}

// SessionUser resolves a session hash to its user and slides the expiry.
func (s *Store) SessionUser(hash string, ttl time.Duration, ip string) (*User, error) {
	var uid, expires, lastSeen int64
	err := s.db.QueryRow(`SELECT user_id, expires_at, last_seen FROM sessions WHERE id=?`, hash).Scan(&uid, &expires, &lastSeen)
	if err != nil {
		return nil, err
	}
	t := now()
	if expires < t {
		s.Exec(`DELETE FROM sessions WHERE id=?`, hash)
		return nil, sql.ErrNoRows
	}
	if t-lastSeen > 600 {
		s.Exec(`UPDATE sessions SET last_seen=?, expires_at=?, ip=? WHERE id=?`, t, t+int64(ttl.Seconds()), ip, hash)
	}
	return s.User(uid)
}

func (s *Store) Sessions(userID int64, currentHash string) []Session {
	rows, err := s.db.Query(`SELECT id, created_at, last_seen, ua, ip FROM sessions WHERE user_id=? AND expires_at>? ORDER BY last_seen DESC`, userID, now())
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var x Session
		rows.Scan(&x.ID, &x.Created, &x.LastSeen, &x.UA, &x.IP)
		x.Current = x.ID == currentHash
		x.Handle = x.ID[:16]
		out = append(out, x)
	}
	return out
}

func (s *Store) TokenUser(hash string) (*User, error) {
	var uid, id int64
	if err := s.db.QueryRow(`SELECT id, user_id FROM tokens WHERE hash=?`, hash).Scan(&id, &uid); err != nil {
		return nil, err
	}
	s.Exec(`UPDATE tokens SET last_used=? WHERE id=?`, now(), id)
	return s.User(uid)
}

// ------------------------------------------------------------------ recipes

// Card is the compact recipe shape used by the library grid.
type Card struct {
	ID         int64    `json:"id"`
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Image      string   `json:"image"`
	Color      string   `json:"color"`
	Tags       []string `json:"tags"`
	Total      int      `json:"total"`
	Fav        bool     `json:"fav"`
	Rating     float64  `json:"rating"`
	Cooked     int      `json:"cooked"`
	LastCooked int64    `json:"lastCooked"`
	Source     string   `json:"source"`
	Created    int64    `json:"created"`
	Updated    int64    `json:"updated"`
}

func (s *Store) Cards(userID int64, where string, args ...any) ([]Card, error) {
	q := `SELECT r.id, r.slug, r.title, r.image, r.image_color, r.tags, r.total_min, r.rating, r.cooked_count, r.last_cooked, r.source_name, r.created_at, r.updated_at,
	      EXISTS(SELECT 1 FROM favorites f WHERE f.recipe_id=r.id AND f.user_id=?) FROM recipes r ` + where
	rows, err := s.db.Query(q, append([]any{userID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Card{}
	for rows.Next() {
		var c Card
		var tags string
		if err := rows.Scan(&c.ID, &c.Slug, &c.Title, &c.Image, &c.Color, &tags, &c.Total, &c.Rating, &c.Cooked, &c.LastCooked, &c.Source, &c.Created, &c.Updated, &c.Fav); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tags), &c.Tags)
		if c.Tags == nil {
			c.Tags = []string{}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const recipeCols = `id, slug, title, description, image, image_color, source_url, source_name, author, yield, servings, prep_min, cook_min, total_min,
 ingredients, steps, notes, tags, nutrition, extra, rating, cooked_count, last_cooked, COALESCE(share_token,''), created_at, updated_at, version`

func scanRecipe(row interface{ Scan(...any) error }) (*model.Recipe, error) {
	r := &model.Recipe{}
	var ing, steps, tags, nut, extra string
	err := row.Scan(&r.ID, &r.Slug, &r.Title, &r.Description, &r.Image, &r.ImageColor, &r.SourceURL, &r.SourceName, &r.Author, &r.Yield, &r.Servings,
		&r.PrepMin, &r.CookMin, &r.TotalMin, &ing, &steps, &r.Notes, &tags, &nut, &extra, &r.Rating, &r.CookedCount, &r.LastCooked, &r.ShareToken, &r.CreatedAt, &r.UpdatedAt, &r.Version)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(ing), &r.Ingredients)
	json.Unmarshal([]byte(steps), &r.Steps)
	json.Unmarshal([]byte(tags), &r.Tags)
	json.Unmarshal([]byte(nut), &r.Nutrition)
	if extra != "" {
		r.Extra = json.RawMessage(extra)
	}
	if r.Ingredients == nil {
		r.Ingredients = []model.Line{}
	}
	if r.Steps == nil {
		r.Steps = []model.Line{}
	}
	if r.Tags == nil {
		r.Tags = []string{}
	}
	r.Cookbooks = []int64{}
	return r, nil
}

// Recipe loads one recipe with the per-user favourite flag and cookbook ids.
func (s *Store) Recipe(id, userID int64) (*model.Recipe, error) {
	r, err := scanRecipe(s.db.QueryRow(`SELECT `+recipeCols+` FROM recipes WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM favorites WHERE recipe_id=? AND user_id=?)`, id, userID).Scan(&r.Favorite)
	rows, err := s.db.Query(`SELECT cookbook_id FROM cookbook_recipes WHERE recipe_id=?`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cb int64
			rows.Scan(&cb)
			r.Cookbooks = append(r.Cookbooks, cb)
		}
	}
	return r, nil
}

func (s *Store) RecipeByShare(token string) (*model.Recipe, error) {
	if token == "" {
		return nil, sql.ErrNoRows
	}
	return scanRecipe(s.db.QueryRow(`SELECT `+recipeCols+` FROM recipes WHERE share_token=?`, token))
}

func js(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func ftsBody(r *model.Recipe) string {
	var b strings.Builder
	b.WriteString(r.Description)
	for _, l := range r.Ingredients {
		b.WriteString("\n" + l.Text)
	}
	for _, l := range r.Steps {
		b.WriteString("\n" + l.Text)
	}
	b.WriteString("\n" + r.Notes + "\n" + r.SourceName + " " + r.Author)
	return b.String()
}

func (s *Store) index(tx *sql.Tx, r *model.Recipe) {
	if !s.fts {
		return
	}
	tx.Exec(`DELETE FROM recipes_fts WHERE rowid=?`, r.ID)
	tx.Exec(`INSERT INTO recipes_fts (rowid, title, tags, body) VALUES (?,?,?,?)`, r.ID, r.Title, strings.Join(r.Tags, " , "), ftsBody(r))
}

// SaveRecipe inserts (ID == 0) or updates a recipe. Image fields, sharing and
// cook history are managed by their own methods and are left untouched here.
//
// A save that really changes the ingredients or the steps starts a new
// version, and the recipe as it then stands is added to its history.
func (s *Store) SaveRecipe(r *model.Recipe, userID int64, origin string) error {
	r.Normalize()
	if r.Title == "" {
		return errors.New("a recipe needs a title")
	}
	r.Slug = model.Slugify(r.Title)
	nut := r.Nutrition
	if nut == nil {
		nut = map[string]string{}
	}
	note := r.VersionNote
	r.VersionNote = ""
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	t := now()
	snapshot := false
	if r.ID == 0 {
		if r.CreatedAt == 0 {
			r.CreatedAt = t
		}
		if r.UpdatedAt == 0 {
			r.UpdatedAt = t
		}
		r.Version, snapshot = 1, true
		res, err := tx.Exec(`INSERT INTO recipes (slug, title, description, image, image_color, source_url, source_name, author, yield, servings, prep_min, cook_min, total_min,
			ingredients, steps, notes, tags, nutrition, extra, rating, cooked_count, last_cooked, origin, created_by, created_at, updated_at, version)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
			r.Slug, r.Title, r.Description, r.Image, r.ImageColor, r.SourceURL, r.SourceName, r.Author, r.Yield, r.Servings, r.PrepMin, r.CookMin, r.TotalMin,
			js(r.Ingredients), js(r.Steps), r.Notes, js(r.Tags), js(nut), string(r.Extra), r.Rating, r.CookedCount, r.LastCooked, origin, userID, r.CreatedAt, r.UpdatedAt)
		if err != nil {
			return err
		}
		r.ID, _ = res.LastInsertId()
	} else {
		var oldIng, oldSteps string
		if err := tx.QueryRow(`SELECT ingredients, steps, version FROM recipes WHERE id=?`, r.ID).Scan(&oldIng, &oldSteps, &r.Version); err != nil {
			return sql.ErrNoRows
		}
		var was struct{ ing, steps []model.Line }
		json.Unmarshal([]byte(oldIng), &was.ing)
		json.Unmarshal([]byte(oldSteps), &was.steps)
		if !model.SameLines(was.ing, r.Ingredients) || !model.SameLines(was.steps, r.Steps) {
			r.Version++
			snapshot = true
		}
		r.UpdatedAt = t
		res, err := tx.Exec(`UPDATE recipes SET slug=?, title=?, description=?, source_url=?, source_name=?, author=?, yield=?, servings=?, prep_min=?, cook_min=?, total_min=?,
			ingredients=?, steps=?, notes=?, tags=?, nutrition=?, extra=?, rating=?, updated_at=?, version=? WHERE id=?`,
			r.Slug, r.Title, r.Description, r.SourceURL, r.SourceName, r.Author, r.Yield, r.Servings, r.PrepMin, r.CookMin, r.TotalMin,
			js(r.Ingredients), js(r.Steps), r.Notes, js(r.Tags), js(nut), string(r.Extra), r.Rating, t, r.Version, r.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return sql.ErrNoRows
		}
	}
	if snapshot {
		var who string
		tx.QueryRow(`SELECT name FROM users WHERE id=?`, userID).Scan(&who)
		if _, err := tx.Exec(`INSERT OR REPLACE INTO recipe_versions (recipe_id, version, title, yield, servings, ingredients, steps, note, user_id, user_name, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.Version, r.Title, r.Yield, r.Servings, js(r.Ingredients), js(r.Steps), note, userID, who, t); err != nil {
			return err
		}
		// Keep the first version and the latest hundred.
		tx.Exec(`DELETE FROM recipe_versions WHERE recipe_id=? AND version>1 AND version<=?`, r.ID, r.Version-maxVersions)
	}
	s.index(tx, r)
	return tx.Commit()
}

func (s *Store) DeleteRecipe(id int64) (image string, err error) {
	s.db.QueryRow(`SELECT image FROM recipes WHERE id=?`, id).Scan(&image)
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM recipes WHERE id=?`, id); err != nil {
		return "", err
	}
	if s.fts {
		tx.Exec(`DELETE FROM recipes_fts WHERE rowid=?`, id)
	}
	return image, tx.Commit()
}

// Search returns matching recipe ids, best first.
func (s *Store) Search(q string) []int64 {
	var terms []string
	for _, f := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(terms) < 8 {
			terms = append(terms, f)
		}
	}
	out := []int64{}
	if len(terms) == 0 {
		return out
	}
	var rows *sql.Rows
	var err error
	if s.fts {
		for i, t := range terms {
			terms[i] = `"` + t + `"*`
		}
		rows, err = s.db.Query(`SELECT rowid FROM recipes_fts WHERE recipes_fts MATCH ? ORDER BY bm25(recipes_fts, 10.0, 5.0, 1.0) LIMIT 300`, strings.Join(terms, " "))
	} else {
		conds, args := []string{}, []any{}
		for _, t := range terms {
			conds = append(conds, `(title LIKE ? OR ingredients LIKE ? OR tags LIKE ?)`)
			args = append(args, "%"+t+"%", "%"+t+"%", "%"+t+"%")
		}
		rows, err = s.db.Query(`SELECT id FROM recipes WHERE `+strings.Join(conds, " AND ")+` LIMIT 300`, args...)
	}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

// RetagAll renames (to != "") or removes a tag across every recipe.
func (s *Store) RetagAll(from, to string) (int, error) {
	rows, err := s.db.Query(`SELECT id, tags FROM recipes WHERE tags LIKE ?`, "%"+strings.ReplaceAll(js(from), `%`, ``)+"%")
	if err != nil {
		return 0, err
	}
	type upd struct {
		id   int64
		tags []string
	}
	var updates []upd
	for rows.Next() {
		var id int64
		var raw string
		rows.Scan(&id, &raw)
		var tags []string
		json.Unmarshal([]byte(raw), &tags)
		changed := false
		for i, t := range tags {
			if strings.EqualFold(t, from) {
				tags[i] = to
				changed = true
			}
		}
		if changed {
			updates = append(updates, upd{id, model.CleanTags(tags)})
		}
	}
	rows.Close()
	for _, u := range updates {
		s.Exec(`UPDATE recipes SET tags=?, updated_at=? WHERE id=?`, js(u.tags), now(), u.id)
		if s.fts {
			s.Exec(`UPDATE recipes_fts SET tags=? WHERE rowid=?`, strings.Join(u.tags, " , "), u.id)
		}
	}
	return len(updates), nil
}

// ------------------------------------------------------------------ cookbooks

type Cookbook struct {
	ID          int64          `json:"id"`
	Title       string         `json:"title"`
	Subtitle    string         `json:"subtitle"`
	Description string         `json:"description"`
	Color       string         `json:"color"`
	Style       string         `json:"style"`
	Chapters    []string       `json:"chapters"`
	ShareToken  string         `json:"shareToken,omitempty"`
	Count       int            `json:"count"`
	Covers      []string       `json:"covers"`
	Entries     []CookbookItem `json:"entries,omitempty"`
	Created     int64          `json:"createdAt"`
	Updated     int64          `json:"updatedAt"`
}

type CookbookItem struct {
	RecipeID int64  `json:"recipeId"`
	Chapter  string `json:"chapter"`
}

const cookbookCols = `id, title, subtitle, description, color, style, chapters, COALESCE(share_token,''), created_at, updated_at`

func (s *Store) scanCookbook(row interface{ Scan(...any) error }) (*Cookbook, error) {
	c := &Cookbook{}
	var chapters string
	if err := row.Scan(&c.ID, &c.Title, &c.Subtitle, &c.Description, &c.Color, &c.Style, &chapters, &c.ShareToken, &c.Created, &c.Updated); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(chapters), &c.Chapters)
	if c.Chapters == nil {
		c.Chapters = []string{}
	}
	c.Covers = []string{}
	return c, nil
}

func (s *Store) Cookbooks() ([]*Cookbook, error) {
	rows, err := s.db.Query(`SELECT ` + cookbookCols + ` FROM cookbooks ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	out := []*Cookbook{}
	for rows.Next() {
		if c, err := s.scanCookbook(rows); err == nil {
			out = append(out, c)
		}
	}
	rows.Close()
	for _, c := range out {
		s.db.QueryRow(`SELECT COUNT(*) FROM cookbook_recipes WHERE cookbook_id=?`, c.ID).Scan(&c.Count)
		ir, err := s.db.Query(`SELECT r.image FROM cookbook_recipes cr JOIN recipes r ON r.id=cr.recipe_id WHERE cr.cookbook_id=? AND r.image<>'' ORDER BY cr.position LIMIT 4`, c.ID)
		if err == nil {
			for ir.Next() {
				var img string
				ir.Scan(&img)
				c.Covers = append(c.Covers, img)
			}
			ir.Close()
		}
	}
	return out, nil
}

func (s *Store) cookbookEntries(c *Cookbook) {
	c.Entries = []CookbookItem{}
	rows, err := s.db.Query(`SELECT recipe_id, chapter FROM cookbook_recipes WHERE cookbook_id=? ORDER BY position`, c.ID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var e CookbookItem
		rows.Scan(&e.RecipeID, &e.Chapter)
		c.Entries = append(c.Entries, e)
	}
	c.Count = len(c.Entries)
}

func (s *Store) Cookbook(id int64) (*Cookbook, error) {
	c, err := s.scanCookbook(s.db.QueryRow(`SELECT `+cookbookCols+` FROM cookbooks WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	s.cookbookEntries(c)
	return c, nil
}

func (s *Store) CookbookByShare(token string) (*Cookbook, error) {
	if token == "" {
		return nil, sql.ErrNoRows
	}
	c, err := s.scanCookbook(s.db.QueryRow(`SELECT `+cookbookCols+` FROM cookbooks WHERE share_token=?`, token))
	if err != nil {
		return nil, err
	}
	s.cookbookEntries(c)
	return c, nil
}

// SaveCookbook inserts or updates a cookbook; when entries is non-nil the
// membership list is replaced in the given order.
func (s *Store) SaveCookbook(c *Cookbook, entries []CookbookItem) error {
	c.Title = clipStr(strings.TrimSpace(c.Title), 120)
	if c.Title == "" {
		return errors.New("a cookbook needs a title")
	}
	c.Subtitle, c.Description = clipStr(strings.TrimSpace(c.Subtitle), 200), clipStr(strings.TrimSpace(c.Description), 4000)
	if c.Color == "" {
		c.Color = "bay"
	}
	if c.Style == "" {
		c.Style = "classic"
	}
	c.Color, c.Style = clipStr(c.Color, 20), clipStr(c.Style, 20)
	if c.Chapters == nil {
		c.Chapters = []string{}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	t := now()
	if c.ID == 0 {
		res, err := tx.Exec(`INSERT INTO cookbooks (title, subtitle, description, color, style, chapters, position, created_at, updated_at)
			VALUES (?,?,?,?,?,?,(SELECT COALESCE(MAX(position),0)+1 FROM cookbooks),?,?)`, c.Title, c.Subtitle, c.Description, c.Color, c.Style, js(c.Chapters), t, t)
		if err != nil {
			return err
		}
		c.ID, _ = res.LastInsertId()
		c.Created = t
	} else if _, err := tx.Exec(`UPDATE cookbooks SET title=?, subtitle=?, description=?, color=?, style=?, chapters=?, updated_at=? WHERE id=?`,
		c.Title, c.Subtitle, c.Description, c.Color, c.Style, js(c.Chapters), t, c.ID); err != nil {
		return err
	}
	c.Updated = t
	if entries != nil {
		if _, err := tx.Exec(`DELETE FROM cookbook_recipes WHERE cookbook_id=?`, c.ID); err != nil {
			return err
		}
		for i, e := range entries {
			tx.Exec(`INSERT OR IGNORE INTO cookbook_recipes (cookbook_id, recipe_id, chapter, position) SELECT ?, id, ?, ? FROM recipes WHERE id=?`, c.ID, clipStr(e.Chapter, 120), i, e.RecipeID)
		}
	}
	return tx.Commit()
}

// SetMembership adds or removes one recipe from a cookbook.
func (s *Store) SetMembership(cookbookID, recipeID int64, in bool) error {
	if !in {
		return s.Exec(`DELETE FROM cookbook_recipes WHERE cookbook_id=? AND recipe_id=?`, cookbookID, recipeID)
	}
	return s.Exec(`INSERT OR IGNORE INTO cookbook_recipes (cookbook_id, recipe_id, chapter, position)
		VALUES (?, ?, '', (SELECT COALESCE(MAX(position),0)+1 FROM cookbook_recipes WHERE cookbook_id=?))`, cookbookID, recipeID, cookbookID)
}

// ------------------------------------------------------------------ shopping

type ShopItem struct {
	ID          int64  `json:"id"`
	Text        string `json:"text"`
	Checked     bool   `json:"checked"`
	RecipeID    *int64 `json:"recipeId"`
	RecipeTitle string `json:"recipeTitle"`
}

func (s *Store) Shopping() []ShopItem {
	out := []ShopItem{}
	rows, err := s.db.Query(`SELECT id, text, checked, recipe_id, recipe_title FROM shopping ORDER BY id`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var x ShopItem
		rows.Scan(&x.ID, &x.Text, &x.Checked, &x.RecipeID, &x.RecipeTitle)
		out = append(out, x)
	}
	return out
}

// ------------------------------------------------------------------ settings

func (s *Store) Setting(key string) string {
	var v string
	s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	return v
}

func (s *Store) SetSetting(key, value string) error {
	return s.Exec(`INSERT INTO settings (key, value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
}

func clipStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	for len(string(r)) > n {
		r = r[:len(r)-1]
	}
	return string(r)
}

// ResetPassword sets a new password and signs the user out everywhere.
func ResetPassword(s *Store, username, password string) error {
	u, err := s.UserByName(username)
	if err != nil {
		return fmt.Errorf("no user named %q", username)
	}
	if err := s.Exec(`UPDATE users SET pass_hash=? WHERE id=?`, hashPassword(password), u.ID); err != nil {
		return err
	}
	return s.Exec(`DELETE FROM sessions WHERE user_id=?`, u.ID)
}
