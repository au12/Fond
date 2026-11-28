package app

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/au12/fond/internal/model"
)

// ------------------------------------------------------------------ meal plan

// Meals a day can hold, in the order a day is read.
var meals = map[string]int{"breakfast": 0, "lunch": 1, "dinner": 2, "snack": 3}

var dayRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// PlanEntry is one meal on one day: a recipe from the library, or a note ("Leftovers", "Eating out").
type PlanEntry struct {
	ID       int64   `json:"id"`
	Day      string  `json:"day"`
	Meal     string  `json:"meal"`
	RecipeID *int64  `json:"recipeId"`
	Text     string  `json:"text"`
	Servings float64 `json:"servings"`
	Position int     `json:"position"`
}

func validDay(d string) bool {
	if !dayRe.MatchString(d) {
		return false
	}
	_, err := time.Parse("2006-01-02", d)
	return err == nil
}

func (s *Store) Plan(from, to string) ([]PlanEntry, error) {
	rows, err := s.db.Query(`SELECT id, day, meal, recipe_id, text, servings, position FROM meal_plan WHERE day>=? AND day<=? ORDER BY day, position, id`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlanEntry{}
	for rows.Next() {
		var e PlanEntry
		if err := rows.Scan(&e.ID, &e.Day, &e.Meal, &e.RecipeID, &e.Text, &e.Servings, &e.Position); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) PlanEntry(id int64) (*PlanEntry, error) {
	var e PlanEntry
	err := s.db.QueryRow(`SELECT id, day, meal, recipe_id, text, servings, position FROM meal_plan WHERE id=?`, id).
		Scan(&e.ID, &e.Day, &e.Meal, &e.RecipeID, &e.Text, &e.Servings, &e.Position)
	return &e, err
}

func (s *Server) listPlan(w http.ResponseWriter, r *http.Request, _ *User) error {
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if !validDay(from) || !validDay(to) || to < from {
		return fail(400, "Ask for a range of days as from=YYYY-MM-DD&to=YYYY-MM-DD")
	}
	a, _ := time.Parse("2006-01-02", from)
	b, _ := time.Parse("2006-01-02", to)
	if b.Sub(a) > 62*24*time.Hour {
		return fail(400, "Ask for two months at most")
	}
	out, err := s.store.Plan(from, to)
	if err != nil {
		return err
	}
	s.writeJSON(w, r, 200, out)
	return nil
}

type planInput struct {
	Day      *string  `json:"day"`
	Meal     *string  `json:"meal"`
	RecipeID *int64   `json:"recipeId"`
	Text     *string  `json:"text"`
	Servings *float64 `json:"servings"`
	Position *int     `json:"position"`
}

func (in *planInput) apply(e *PlanEntry) error {
	if in.Day != nil {
		if !validDay(*in.Day) {
			return fail(400, "That isn't a day")
		}
		e.Day = *in.Day
	}
	if in.Meal != nil {
		m := strings.ToLower(strings.TrimSpace(*in.Meal))
		if _, ok := meals[m]; !ok {
			return fail(400, "Meals are breakfast, lunch, dinner or snack")
		}
		e.Meal = m
	}
	if in.Text != nil {
		e.Text = clipStr(model.CleanLine(*in.Text), 200)
	}
	if in.Servings != nil {
		if *in.Servings < 0 || *in.Servings > 1000 {
			return fail(400, "Servings go from 0 to 1000")
		}
		e.Servings = *in.Servings
	}
	if in.Position != nil {
		e.Position = *in.Position
	}
	return nil
}

func (s *Server) addPlan(w http.ResponseWriter, r *http.Request, u *User) error {
	var in planInput
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	e := PlanEntry{Meal: "dinner"}
	if err := in.apply(&e); err != nil {
		return err
	}
	if e.Day == "" {
		return fail(400, "Pick a day")
	}
	if in.RecipeID != nil && *in.RecipeID > 0 {
		var id int64
		if s.store.db.QueryRow(`SELECT id FROM recipes WHERE id=?`, *in.RecipeID).Scan(&id) != nil {
			return errNotFound
		}
		e.RecipeID = &id
	}
	if e.RecipeID == nil && e.Text == "" {
		return fail(400, "Pick a recipe or write a note")
	}
	var n int
	s.store.db.QueryRow(`SELECT COUNT(*) FROM meal_plan WHERE day=?`, e.Day).Scan(&n)
	if n >= 40 {
		return fail(400, "That day is full")
	}
	if in.Position == nil {
		s.store.db.QueryRow(`SELECT COALESCE(MAX(position),0)+1 FROM meal_plan WHERE day=?`, e.Day).Scan(&e.Position)
	}
	res, err := s.store.db.Exec(`INSERT INTO meal_plan (day, meal, recipe_id, text, servings, position, created_by, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		e.Day, e.Meal, e.RecipeID, e.Text, e.Servings, e.Position, u.ID, now())
	if err != nil {
		return err
	}
	e.ID, _ = res.LastInsertId()
	s.writeJSON(w, r, 201, e)
	return nil
}

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in planInput
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	e, err := s.store.PlanEntry(pathID(r, "id"))
	if err == sql.ErrNoRows {
		return errNotFound
	} else if err != nil {
		return err
	}
	if err := in.apply(e); err != nil {
		return err
	}
	if e.RecipeID == nil && e.Text == "" {
		return fail(400, "Write a note")
	}
	if err := s.store.Exec(`UPDATE meal_plan SET day=?, meal=?, text=?, servings=?, position=? WHERE id=?`, e.Day, e.Meal, e.Text, e.Servings, e.Position, e.ID); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, e)
	return nil
}

func (s *Server) deletePlan(w http.ResponseWriter, r *http.Request, _ *User) error {
	s.store.Exec(`DELETE FROM meal_plan WHERE id=?`, pathID(r, "id"))
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

// reorderPlan sets the order of the meals on one day, and moves any of them there from another day.
func (s *Server) reorderPlan(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct {
		Day  string  `json:"day"`
		Meal string  `json:"meal"`
		IDs  []int64 `json:"ids"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if !validDay(in.Day) {
		return fail(400, "That isn't a day")
	}
	if _, ok := meals[in.Meal]; !ok {
		return fail(400, "Meals are breakfast, lunch, dinner or snack")
	}
	if len(in.IDs) > 40 {
		return fail(400, "Too many at once")
	}
	tx, err := s.store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range in.IDs {
		if _, err := tx.Exec(`UPDATE meal_plan SET day=?, meal=?, position=? WHERE id=?`, in.Day, in.Meal, i+1, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

// ------------------------------------------------------------------ shopping: several lines at once

func (s *Server) groupShopping(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct {
		IDs   []int64 `json:"ids"`
		Group string  `json:"group"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if len(in.IDs) == 0 || len(in.IDs) > 500 {
		return fail(400, "Pick some lines")
	}
	in.Group = clipStr(strings.TrimSpace(in.Group), 40)
	tx, err := s.store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range in.IDs {
		tx.Exec(`UPDATE shopping SET grp=? WHERE id=?`, in.Group, id)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.listShopping(w, r, nil)
}

func (s *Server) bulkShopping(w http.ResponseWriter, r *http.Request, _ *User) error {
	var in struct {
		IDs     []int64 `json:"ids"`
		Checked *bool   `json:"checked"`
		Delete  bool    `json:"delete"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if len(in.IDs) > 500 {
		return fail(400, "Too many lines at once")
	}
	tx, err := s.store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range in.IDs {
		switch {
		case in.Delete:
			tx.Exec(`DELETE FROM shopping WHERE id=?`, id)
		case in.Checked != nil:
			tx.Exec(`UPDATE shopping SET checked=? WHERE id=?`, *in.Checked, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.listShopping(w, r, nil)
}
