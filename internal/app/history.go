package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/au12/fond/internal/model"
)

// ------------------------------------------------------------------ recipe history

// maxVersions is how many versions of a recipe are kept, besides the first.
const maxVersions = 100

// Version is one saved state of a recipe's ingredients and steps.
type Version struct {
	Version     int          `json:"version"`
	Title       string       `json:"title"`
	Yield       string       `json:"yield"`
	Servings    float64      `json:"servings"`
	Ingredients []model.Line `json:"ingredients"`
	Steps       []model.Line `json:"steps"`
	Note        string       `json:"note"`
	User        string       `json:"user"`
	At          int64        `json:"at"`
}

const versionCols = `version, title, yield, servings, ingredients, steps, note, user_name, created_at`

func scanVersion(row interface{ Scan(...any) error }) (*Version, error) {
	v := &Version{}
	var ing, steps string
	if err := row.Scan(&v.Version, &v.Title, &v.Yield, &v.Servings, &ing, &steps, &v.Note, &v.User, &v.At); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(ing), &v.Ingredients)
	json.Unmarshal([]byte(steps), &v.Steps)
	if v.Ingredients == nil {
		v.Ingredients = []model.Line{}
	}
	if v.Steps == nil {
		v.Steps = []model.Line{}
	}
	return v, nil
}

// Versions lists a recipe's history, newest first.
func (s *Store) Versions(recipeID int64) ([]*Version, error) {
	rows, err := s.db.Query(`SELECT `+versionCols+` FROM recipe_versions WHERE recipe_id=? ORDER BY version DESC`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Version{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) Version(recipeID int64, version int) (*Version, error) {
	return scanVersion(s.db.QueryRow(`SELECT `+versionCols+` FROM recipe_versions WHERE recipe_id=? AND version=?`, recipeID, version))
}

func (s *Server) listVersions(w http.ResponseWriter, r *http.Request, u *User) error {
	rec, err := s.store.Recipe(pathID(r, "id"), u.ID)
	if err != nil {
		return errNotFound
	}
	versions, err := s.store.Versions(rec.ID)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]any{"version": rec.Version, "versions": versions})
	return nil
}

// restoreVersion puts an earlier version's ingredients, steps and yield back.
// Nothing is lost: the restored recipe becomes a new version of its own.
func (s *Server) restoreVersion(w http.ResponseWriter, r *http.Request, u *User) error {
	rec, err := s.store.Recipe(pathID(r, "id"), u.ID)
	if err != nil {
		return errNotFound
	}
	n, _ := strconv.Atoi(r.PathValue("version"))
	v, err := s.store.Version(rec.ID, n)
	if err != nil {
		return errNotFound
	}
	rec.Ingredients, rec.Steps, rec.Yield, rec.Servings = v.Ingredients, v.Steps, v.Yield, v.Servings
	rec.VersionNote = fmt.Sprintf("Restored version %d", v.Version)
	if err := s.store.SaveRecipe(rec, u.ID, ""); err != nil {
		return fail(400, err.Error())
	}
	out, err := s.store.Recipe(rec.ID, u.ID)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, out)
	return nil
}

// ------------------------------------------------------------------ household food rules

// FoodRule is a household's correction to how an ingredient is weighed: treat
// a phrase as a food in the dictionary, give it a weight of the household's
// own, or leave it alone.
type FoodRule struct {
	ID      int64   `json:"id"`
	Phrase  string  `json:"phrase"`
	Action  string  `json:"action"` // map | custom | skip
	FoodID  string  `json:"foodId,omitempty"`
	GPerMl  float64 `json:"gPerMl,omitempty"`
	GEach   float64 `json:"gEach,omitempty"`
	Source  string  `json:"source"` // user | ai
	Updated int64   `json:"updated"`
}

var foodIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

func (f *FoodRule) validate() error {
	f.Phrase = clipStr(strings.ToLower(model.CleanLine(f.Phrase)), 120)
	if f.Phrase == "" {
		return fail(400, "Name the ingredient this is for")
	}
	if f.Source != "ai" {
		f.Source = "user"
	}
	switch f.Action {
	case "map":
		if !foodIDRe.MatchString(f.FoodID) {
			return fail(400, "Choose a food")
		}
		f.GPerMl, f.GEach = 0, 0
	case "custom":
		f.FoodID = ""
		if f.GPerMl < 0 || f.GPerMl > 5 || f.GEach < 0 || f.GEach > 20000 || (f.GPerMl == 0 && f.GEach == 0) {
			return fail(400, "Enter what it weighs")
		}
	case "skip":
		f.FoodID, f.GPerMl, f.GEach = "", 0, 0
	default:
		return fail(400, "Unknown kind of rule")
	}
	return nil
}

func (s *Store) FoodRules() ([]*FoodRule, error) {
	rows, err := s.db.Query(`SELECT id, phrase, action, food_id, g_per_ml, g_each, source, updated_at FROM food_rules ORDER BY phrase`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*FoodRule{}
	for rows.Next() {
		f := &FoodRule{}
		if err := rows.Scan(&f.ID, &f.Phrase, &f.Action, &f.FoodID, &f.GPerMl, &f.GEach, &f.Source, &f.Updated); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SaveFoodRule adds a rule, or replaces the one already kept for its phrase.
func (s *Store) SaveFoodRule(f *FoodRule, userID int64) error {
	t := now()
	if err := s.Exec(`INSERT INTO food_rules (phrase, action, food_id, g_per_ml, g_each, source, user_id, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(phrase) DO UPDATE SET action=excluded.action, food_id=excluded.food_id, g_per_ml=excluded.g_per_ml, g_each=excluded.g_each,
		source=excluded.source, user_id=excluded.user_id, updated_at=excluded.updated_at`,
		f.Phrase, f.Action, f.FoodID, f.GPerMl, f.GEach, f.Source, userID, t, t); err != nil {
		return err
	}
	f.Updated = t
	return s.db.QueryRow(`SELECT id FROM food_rules WHERE phrase=?`, f.Phrase).Scan(&f.ID)
}

func (s *Server) listFoodRules(w http.ResponseWriter, r *http.Request, _ *User) error {
	rules, err := s.store.FoodRules()
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, rules)
	return nil
}

func (s *Server) saveFoodRule(w http.ResponseWriter, r *http.Request, u *User) error {
	var f FoodRule
	if err := readJSON(w, r, &f); err != nil {
		return err
	}
	if err := f.validate(); err != nil {
		return err
	}
	var n int
	s.store.db.QueryRow(`SELECT COUNT(*) FROM food_rules`).Scan(&n)
	if n >= 5000 {
		return fail(400, "That is more corrections than Fond keeps. Remove some first.")
	}
	if err := s.store.SaveFoodRule(&f, u.ID); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, f)
	return nil
}

func (s *Server) deleteFoodRule(w http.ResponseWriter, r *http.Request, _ *User) error {
	if err := s.store.Exec(`DELETE FROM food_rules WHERE id=?`, pathID(r, "id")); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

// ------------------------------------------------------------------ every ingredient line

// IngredientLine is one distinct ingredient line and where it is used.
type IngredientLine struct {
	Text    string  `json:"t"`
	Count   int     `json:"n"`
	Recipes []int64 `json:"r"`
}

// IngredientLines gathers every distinct ingredient line in the library, so
// that the client can report which ones it does not know how to weigh.
func (s *Store) IngredientLines() ([]*IngredientLine, error) {
	rows, err := s.db.Query(`SELECT id, ingredients FROM recipes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]*IngredientLine{}
	out := []*IngredientLine{}
	for rows.Next() {
		var id int64
		var raw string
		if rows.Scan(&id, &raw) != nil {
			continue
		}
		var lines []model.Line
		json.Unmarshal([]byte(raw), &lines)
		for _, l := range lines {
			if l.H || l.Text == "" {
				continue
			}
			e := seen[l.Text]
			if e == nil {
				if len(out) >= 100000 {
					continue
				}
				e = &IngredientLine{Text: l.Text}
				seen[l.Text] = e
				out = append(out, e)
			}
			e.Count++
			if len(e.Recipes) < 3 && (len(e.Recipes) == 0 || e.Recipes[len(e.Recipes)-1] != id) {
				e.Recipes = append(e.Recipes, id)
			}
		}
	}
	return out, rows.Err()
}

func (s *Server) ingredientLines(w http.ResponseWriter, r *http.Request, _ *User) error {
	lines, err := s.store.IngredientLines()
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]any{"lines": lines})
	return nil
}

// ------------------------------------------------------------------ AI suggestions

const foodSystem = `You help a recipe app weigh ingredients. You are given a list of foods the app knows, as "id: name" lines, and a list of ingredient names it does not know.
For each ingredient pick the ONE food from the list that would weigh about the same per cup or per spoon, or "" if nothing in the list is close.
Reply with ONE JSON object and nothing else, mapping each ingredient exactly as written to the chosen id, like {"icing sugar": "sugar-powdered", "gold leaf": ""}.
Only use ids from the list. Prefer "" to a poor match: a wrong weight is worse than none.`

// suggestFoods asks the configured AI model which dictionary foods some
// unknown ingredient names resemble. The answers are only suggestions: the
// client shows them for the cook to confirm and never applies them itself.
func (s *Server) suggestFoods(w http.ResponseWriter, r *http.Request, u *User) error {
	if !s.aiConfig().ready() {
		return fail(400, "AI assist isn't set up")
	}
	if !s.limit.hit("ai-foods:"+itoa(int(u.ID)), 20, time.Minute) {
		return fail(429, "That's a lot of questions at once. Give it a minute.")
	}
	var in struct {
		Phrases []string    `json:"phrases"`
		Foods   [][2]string `json:"foods"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if len(in.Phrases) == 0 || len(in.Phrases) > 40 || len(in.Foods) == 0 || len(in.Foods) > 1000 {
		return fail(400, "Send between 1 and 40 ingredients, and the foods to choose from")
	}
	known := map[string]bool{}
	var b strings.Builder
	b.WriteString("Foods:\n")
	for _, f := range in.Foods {
		id, name := f[0], clipStr(model.CleanLine(f[1]), 80)
		if !foodIDRe.MatchString(id) || name == "" {
			continue
		}
		known[id] = true
		b.WriteString(id + ": " + name + "\n")
	}
	b.WriteString("\nIngredients:\n")
	asked := map[string]bool{}
	for _, p := range in.Phrases {
		if p = clipStr(model.CleanLine(p), 120); p != "" && !asked[p] {
			asked[p] = true
			b.WriteString(p + "\n")
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	text, err := s.aiChat(ctx, foodSystem, b.String())
	if err != nil {
		return fail(502, "The AI model didn't answer: "+err.Error())
	}
	got := map[string]string{}
	if err := json.Unmarshal([]byte(jsonObject(text)), &got); err != nil {
		return fail(502, "The AI model didn't return a usable answer")
	}
	out := map[string]string{}
	for p, id := range got {
		if asked[p] && known[id] {
			out[p] = id
		}
	}
	s.writeJSON(w, r, 200, map[string]any{"suggestions": out})
	return nil
}

// jsonObject cuts the outermost {...} out of a model's reply.
func jsonObject(text string) string {
	if i, j := strings.Index(text, "{"), strings.LastIndex(text, "}"); i >= 0 && j > i {
		return text[i : j+1]
	}
	return text
}

var errAIReply = errors.New("the AI endpoint sent an unexpected reply")
