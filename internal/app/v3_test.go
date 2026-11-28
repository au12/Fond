package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"
)

func (c *client) raw(method, path string, body any) (int, []byte) {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, c.base+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.base)
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func (c *client) upload(path string, name string, data []byte, fields map[string]string) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write(data)
	mw.Close()
	req, _ := http.NewRequest("POST", c.base+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", c.base)
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// waitJob polls an import job until it finishes.
func (c *client) waitJob(id string) map[string]any {
	c.t.Helper()
	for i := 0; i < 200; i++ {
		_, j := c.do("GET", "/api/jobs/"+id, nil)
		if j["state"] != "running" {
			return j
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatal("job did not finish")
	return nil
}

func TestTOTPVectors(t *testing.T) {
	// RFC 6238, appendix B, SHA-1, truncated to six digits.
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	for at, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		if got := totpAt(secret, at/30); got != want {
			t.Errorf("code at %d = %s, want %s", at, got, want)
		}
	}
	at := time.Unix(1111111109, 0)
	if step := checkTOTP(secret, "081 804", at, 0); step != 1111111109/30 {
		t.Errorf("a spaced code was refused")
	}
	if step := checkTOTP(secret, "081804", at, 1111111109/30); step != 0 {
		t.Errorf("a used step was accepted again")
	}
	if step := checkTOTP(secret, totpAt(secret, 1111111109/30-1), at, 0); step == 0 {
		t.Errorf("the code from the step before was refused")
	}
	if step := checkTOTP(secret, totpAt(secret, 1111111109/30-3), at, 0); step != 0 {
		t.Errorf("a code from a minute and a half ago was accepted")
	}
}

func TestTwoFactorSignIn(t *testing.T) {
	c, srv := setupCook(t)
	_, setup := c.do("POST", "/api/me/2fa/setup", nil)
	secret, _ := setup["secret"].(string)
	if secret == "" || !bytes.Contains([]byte(setup["uri"].(string)), []byte("otpauth://totp/")) {
		t.Fatalf("setup = %v", setup)
	}
	if code, _ := c.do("POST", "/api/me/2fa/enable", map[string]string{"secret": secret, "code": "000000"}); code != 400 {
		t.Fatalf("a wrong code turned it on: %d", code)
	}
	now := time.Now().Unix() / 30
	code, on := c.do("POST", "/api/me/2fa/enable", map[string]string{"secret": secret, "code": totpAt(secret, now)})
	if code != 200 {
		t.Fatalf("enable = %d %v", code, on)
	}
	codes := on["recovery"].([]any)
	if len(codes) != recoveryCount {
		t.Fatalf("got %d recovery codes", len(codes))
	}
	c.do("POST", "/api/auth/logout", nil)

	// The password alone no longer signs in.
	code, step := c.do("POST", "/api/auth/login", map[string]string{"username": "cook", "password": "correct horse"})
	if code != 200 || step["twoFactor"] != true || step["user"] != nil {
		t.Fatalf("login = %d %v", code, step)
	}
	if code, _ := c.do("GET", "/api/me", nil); code != 401 {
		t.Fatalf("signed in without a code: %d", code)
	}
	ticket := step["ticket"].(string)
	// The code that turned it on was spent; it can't sign in as well.
	if code, _ := c.do("POST", "/api/auth/code", map[string]string{"ticket": ticket, "code": totpAt(secret, now)}); code != 401 {
		t.Fatalf("a spent code signed in: %d", code)
	}
	// A recovery code works once.
	rc := codes[0].(string)
	if code, _ := c.do("POST", "/api/auth/code", map[string]string{"ticket": ticket, "code": rc}); code != 200 {
		t.Fatalf("recovery code = %d", code)
	}
	if code, _ := c.do("GET", "/api/me", nil); code != 200 {
		t.Fatalf("not signed in after the code: %d", code)
	}
	c.do("POST", "/api/auth/logout", nil)
	_, step = c.do("POST", "/api/auth/login", map[string]string{"username": "cook", "password": "correct horse"})
	if code, _ := c.do("POST", "/api/auth/code", map[string]string{"ticket": step["ticket"].(string), "code": rc}); code != 401 {
		t.Fatalf("a recovery code worked twice: %d", code)
	}
	// Five wrong tries spend the ticket.
	for i := 0; i < 5; i++ {
		c.do("POST", "/api/auth/code", map[string]string{"ticket": step["ticket"].(string), "code": "123456"})
	}
	if code, _ := c.do("POST", "/api/auth/code", map[string]string{"ticket": step["ticket"].(string), "code": codes[1].(string)}); code != 401 {
		t.Fatalf("the ticket outlived five wrong codes: %d", code)
	}
	_, step = c.do("POST", "/api/auth/login", map[string]string{"username": "cook", "password": "correct horse"})
	if code, _ := c.do("POST", "/api/auth/code", map[string]string{"ticket": step["ticket"].(string), "code": codes[1].(string)}); code != 200 {
		t.Fatalf("second recovery code = %d", code)
	}
	if _, st := c.do("GET", "/api/me/2fa", nil); st["recoveryLeft"].(float64) != recoveryCount-2 {
		t.Fatalf("status = %v", st)
	}
	if code, _ := c.do("POST", "/api/me/2fa/disable", map[string]string{"password": "wrong"}); code != 403 {
		t.Fatalf("turned off with a wrong password: %d", code)
	}
	if code, _ := c.do("POST", "/api/me/2fa/disable", map[string]string{"password": "correct horse"}); code != 200 {
		t.Fatalf("disable = %d", code)
	}
	if u, _ := srv.store.UserByName("cook"); u.TwoFactor {
		t.Fatal("still on")
	}
}

func TestMealPlan(t *testing.T) {
	c, _ := setupCook(t)
	_, rec := c.do("POST", "/api/recipes", map[string]any{"title": "Soup", "ingredients": lines("1 onion"), "steps": lines("Simmer.")})
	rid := rec["id"].(float64)
	if code, _ := c.do("POST", "/api/plan", map[string]any{"day": "2026-02-30", "recipeId": rid}); code != 400 {
		t.Fatalf("an impossible day was accepted: %d", code)
	}
	if code, _ := c.do("POST", "/api/plan", map[string]any{"day": "2026-12-01"}); code != 400 {
		t.Fatalf("an empty meal was accepted: %d", code)
	}
	_, a := c.do("POST", "/api/plan", map[string]any{"day": "2026-12-01", "recipeId": rid, "servings": 4})
	_, b := c.do("POST", "/api/plan", map[string]any{"day": "2026-12-01", "meal": "lunch", "text": "Leftovers"})
	if a["meal"] != "dinner" || b["meal"] != "lunch" {
		t.Fatalf("meals = %v %v", a, b)
	}
	c.do("PUT", "/api/plan/"+itoa(int(b["id"].(float64))), map[string]any{"day": "2026-12-03"})
	c.do("POST", "/api/plan/order", map[string]any{"day": "2026-12-02", "meal": "dinner", "ids": []float64{a["id"].(float64)}})
	_, body := c.raw("GET", "/api/plan?from=2026-12-01&to=2026-12-07", nil)
	var got []PlanEntry
	json.Unmarshal(body, &got)
	if len(got) != 2 || got[0].Day != "2026-12-02" || got[1].Day != "2026-12-03" || got[1].Text != "Leftovers" || got[0].Servings != 4 {
		t.Fatalf("plan = %s", body)
	}
	// A recipe deleted from the library leaves the plan.
	c.do("DELETE", "/api/recipes/"+itoa(int(rid)), nil)
	_, body = c.raw("GET", "/api/plan?from=2026-12-01&to=2026-12-07", nil)
	json.Unmarshal(body, &got)
	if len(got) != 1 {
		t.Fatalf("plan after delete = %s", body)
	}
}

func TestShoppingGroups(t *testing.T) {
	c, _ := setupCook(t)
	_, body := c.raw("POST", "/api/shopping", map[string]any{"items": []map[string]any{{"text": "1 cup flour"}, {"text": "200 g flour"}, {"text": "2 eggs"}}})
	var items []ShopItem
	json.Unmarshal(body, &items)
	if len(items) != 3 {
		t.Fatalf("items = %s", body)
	}
	_, body = c.raw("POST", "/api/shopping/group", map[string]any{"ids": []int64{items[0].ID, items[2].ID}, "group": "m:abc"})
	json.Unmarshal(body, &items)
	if items[0].Group != "m:abc" || items[1].Group != "" || items[2].Group != "m:abc" {
		t.Fatalf("groups = %s", body)
	}
	_, body = c.raw("POST", "/api/shopping/bulk", map[string]any{"ids": []int64{items[0].ID, items[2].ID}, "checked": true})
	json.Unmarshal(body, &items)
	if !items[0].Checked || items[1].Checked || !items[2].Checked {
		t.Fatalf("checked = %s", body)
	}
	_, body = c.raw("POST", "/api/shopping/bulk", map[string]any{"ids": []int64{items[0].ID, items[2].ID}, "delete": true})
	json.Unmarshal(body, &items)
	if len(items) != 1 || items[0].Text != "200 g flour" {
		t.Fatalf("after delete = %s", body)
	}
}

func TestBackupRoundTrip(t *testing.T) {
	c, srv := setupCook(t)
	_, a := c.do("POST", "/api/recipes", map[string]any{"title": "Bread", "ingredients": lines("500 g flour"), "steps": lines("Bake."), "rating": 4})
	aid := itoa(int(a["id"].(float64)))
	c.do("PUT", "/api/recipes/"+aid, map[string]any{"title": "Bread", "ingredients": lines("500 g flour", "10 g salt"), "steps": lines("Bake."), "rating": 4})
	c.do("PUT", "/api/recipes/"+aid+"/favorite", map[string]any{"on": true})
	_, b := c.do("POST", "/api/recipes", map[string]any{"title": "Butter", "ingredients": lines("1 pint cream"), "steps": lines("Churn.")})
	bid := b["id"].(float64)
	c.do("POST", "/api/cookbooks", map[string]any{"title": "Dairy", "leather": 1234, "color": "plum", "chapters": []string{"Fresh"}, "entries": []map[string]any{{"recipeId": bid, "chapter": "Fresh"}}})
	c.do("PUT", "/api/food-rules", map[string]any{"phrase": "mystery flour", "action": "map", "foodId": "flour-ap"})
	c.do("POST", "/api/plan", map[string]any{"day": "2026-12-05", "recipeId": a["id"]})
	c.raw("POST", "/api/shopping", map[string]any{"items": []map[string]any{{"text": "6 eggs"}}})
	srv.store.Exec(`UPDATE recipes SET origin='mealie:abc' WHERE id=?`, bid)

	code, zipped := c.raw("GET", "/api/export", nil)
	if code != 200 {
		t.Fatalf("export = %d", code)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil {
		t.Fatal(err)
	}
	var bk backup
	f, _ := zr.Open("fond.json")
	json.NewDecoder(f).Decode(&bk)
	if bk.Version != 3 || len(bk.Recipes) != 2 || len(bk.Favorites) != 1 || len(bk.MealPlan) != 1 || len(bk.Shopping) != 1 || bk.Origins[itoa(int(bid))] != "mealie:abc" {
		t.Fatalf("backup = %+v", bk)
	}

	// Merging the backup into the library it came from adds nothing.
	code, res := c.upload("/api/import/file", "fond-export.zip", zipped, nil)
	if code != 202 || res["kind"] != "fond" {
		t.Fatalf("import = %d %v", code, res)
	}
	j := c.waitJob(res["job"].(string))
	if j["state"] != "done" || j["imported"].(float64) != 0 || j["skipped"].(float64) != 2 {
		t.Fatalf("merge job = %v", j)
	}

	// Replacing clears the library and puts the backup back as it was, in a fresh household.
	c2, _ := setupCook(t)
	c2.do("POST", "/api/recipes", map[string]any{"title": "Doomed", "ingredients": lines("1 thing"), "steps": lines("Go.")})
	code, res = c2.upload("/api/import/file", "fond-export.zip", zipped, map[string]string{"mode": "replace"})
	if code != 202 {
		t.Fatalf("replace = %d %v", code, res)
	}
	j = c2.waitJob(res["job"].(string))
	if j["state"] != "done" || j["imported"].(float64) != 2 || j["cookbooks"].(float64) != 1 {
		t.Fatalf("replace job = %v", j)
	}
	_, body := c2.raw("GET", "/api/recipes", nil)
	var cards []Card
	json.Unmarshal(body, &cards)
	if len(cards) != 2 {
		t.Fatalf("library = %s", body)
	}
	var bread Card
	for _, cd := range cards {
		if cd.Title == "Bread" {
			bread = cd
		}
	}
	if !bread.Fav || bread.Rating != 4 {
		t.Fatalf("bread = %+v", bread)
	}
	cur, nums, _ := versionNumbers(t, c2, itoa(int(bread.ID)))
	if cur != 2 || len(nums) != 2 {
		t.Fatalf("history = %v %v", cur, nums)
	}
	_, books := c2.raw("GET", "/api/cookbooks", nil)
	var bs []Cookbook
	json.Unmarshal(books, &bs)
	if len(bs) != 1 || bs[0].Leather != 1234 || bs[0].Color != "plum" || bs[0].Count != 1 {
		t.Fatalf("cookbooks = %s", books)
	}
	_, plan := c2.raw("GET", "/api/plan?from=2026-12-01&to=2026-12-07", nil)
	var pe []PlanEntry
	json.Unmarshal(plan, &pe)
	if len(pe) != 1 || pe[0].RecipeID == nil || *pe[0].RecipeID != bread.ID {
		t.Fatalf("plan = %s", plan)
	}
	_, rules := c2.raw("GET", "/api/food-rules", nil)
	if !bytes.Contains(rules, []byte("mystery flour")) {
		t.Fatalf("rules = %s", rules)
	}
}

func TestImportRefusesStrangers(t *testing.T) {
	c, _ := setupCook(t)
	if code, res := c.upload("/api/import/file", "notes.txt", []byte("just some text"), nil); code != 400 {
		t.Fatalf("a text file was accepted: %d %v", code, res)
	}
}
