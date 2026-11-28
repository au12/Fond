package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ------------------------------------------------------------------ authenticator codes (RFC 6238)

const (
	totpPeriod = 30
	totpDigits = 6
	// A code from the step before or after also counts, for clocks a little apart.
	totpSkew      = 1
	recoveryCount = 10
	// How long the password step of a sign-in waits for its code.
	ticketTTL = 5 * time.Minute
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() string {
	b := make([]byte, 20)
	rand.Read(b)
	return b32.EncodeToString(b)
}

func totpAt(secret string, step int64) string {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return ""
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	code := itoa(int(n % 1000000))
	return strings.Repeat("0", totpDigits-len(code)) + code
}

// checkTOTP returns the time step the code belongs to, or 0. A step at or before
// after has been used already and does not count again.
func checkTOTP(secret, code string, at time.Time, after int64) int64 {
	code = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, code)
	if len(code) != totpDigits || secret == "" {
		return 0
	}
	now := at.Unix() / totpPeriod
	for d := int64(-totpSkew); d <= totpSkew; d++ {
		step := now + d
		if step > after && subtle.ConstantTimeCompare([]byte(totpAt(secret, step)), []byte(code)) == 1 {
			return step
		}
	}
	return 0
}

// Recovery codes look like "k7f2-9qmx": easy to read out, and each works once.
func newRecoveryCodes() (plain []string, hashed []string) {
	const alphabet = "23456789abcdefghjkmnpqrstuvwxyz"
	for i := 0; i < recoveryCount; i++ {
		b := make([]byte, 8)
		rand.Read(b)
		var sb strings.Builder
		for j, c := range b {
			if j == 4 {
				sb.WriteByte('-')
			}
			sb.WriteByte(alphabet[int(c)%len(alphabet)])
		}
		plain = append(plain, sb.String())
		hashed = append(hashed, hashToken(sb.String()))
	}
	return
}

func normRecovery(code string) string {
	c := strings.ToLower(strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return -1
		}
		return r
	}, code))
	if len(c) != 8 {
		return ""
	}
	return c[:4] + "-" + c[4:]
}

// useSecondFactor checks an authenticator or recovery code for u and spends it.
func (s *Server) useSecondFactor(u *User, code string) bool {
	if step := checkTOTP(u.totp, code, time.Now(), u.totpStep); step > 0 {
		// Only one sign-in per code: the step is remembered and later codes must be newer.
		res, err := s.store.db.Exec(`UPDATE users SET totp_step=? WHERE id=? AND totp_step<?`, step, u.ID, step)
		if err != nil {
			return false
		}
		n, _ := res.RowsAffected()
		return n == 1
	}
	rc := normRecovery(code)
	if rc == "" {
		return false
	}
	var codes []string
	json.Unmarshal([]byte(u.recovery), &codes)
	h := hashToken(rc)
	for i, c := range codes {
		if subtle.ConstantTimeCompare([]byte(c), []byte(h)) == 1 {
			rest := append(append([]string{}, codes[:i]...), codes[i+1:]...)
			res, err := s.store.db.Exec(`UPDATE users SET recovery=? WHERE id=? AND recovery=?`, js(rest), u.ID, u.recovery)
			if err != nil {
				return false
			}
			n, _ := res.RowsAffected()
			return n == 1
		}
	}
	return false
}

// ------------------------------------------------------------------ the second step of signing in

// A ticket says "this password was right" for a few minutes, so the code can follow in a second request.
type ticket struct {
	user    int64
	expires time.Time
	tries   int
}

type tickets struct {
	mu sync.Mutex
	m  map[string]*ticket
}

func (t *tickets) issue(user int64) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.m == nil {
		t.m = map[string]*ticket{}
	}
	now := time.Now()
	for k, v := range t.m {
		if now.After(v.expires) {
			delete(t.m, k)
		}
	}
	id := randToken(24)
	t.m[hashToken(id)] = &ticket{user: user, expires: now.Add(ticketTTL)}
	return id
}

// take returns the ticket's user, counting a try; a ticket allows five.
func (t *tickets) take(id string) (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := hashToken(id)
	v, ok := t.m[k]
	if !ok || time.Now().After(v.expires) {
		delete(t.m, k)
		return 0, false
	}
	v.tries++
	if v.tries > 5 {
		delete(t.m, k)
		return 0, false
	}
	return v.user, true
}

func (t *tickets) drop(id string) {
	t.mu.Lock()
	delete(t.m, hashToken(id))
	t.mu.Unlock()
}

func (s *Server) loginCode(w http.ResponseWriter, r *http.Request) error {
	if err := s.cop.Check(r); err != nil {
		return fail(403, "Cross-site request blocked")
	}
	var in struct{ Ticket, Code string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	ipKey := "login-ip:" + s.clientIP(r)
	if over, wait := s.limit.over(ipKey, 12); over {
		w.Header().Set("Retry-After", itoa(int(wait.Seconds())+1))
		return fail(429, "Too many attempts. Try again in "+humanWait(wait)+".")
	}
	uid, ok := s.tickets.take(in.Ticket)
	if !ok {
		return fail(401, "That sign-in has expired. Enter your password again.")
	}
	u, err := s.store.User(uid)
	if err != nil || !u.TwoFactor {
		s.tickets.drop(in.Ticket)
		return fail(401, "That sign-in has expired. Enter your password again.")
	}
	if !s.useSecondFactor(u, in.Code) {
		s.limit.hit(ipKey, 12, 10*time.Minute)
		return fail(401, "That code isn't right. Codes change every 30 seconds.")
	}
	s.tickets.drop(in.Ticket)
	if err := s.startSession(w, r, u); err != nil {
		return err
	}
	return s.me(w, r, u)
}

// ------------------------------------------------------------------ turning it on and off

func (s *Server) twoFactorStatus(w http.ResponseWriter, r *http.Request, u *User) error {
	var codes []string
	json.Unmarshal([]byte(u.recovery), &codes)
	s.writeJSON(w, r, 200, map[string]any{"on": u.TwoFactor, "recoveryLeft": len(codes)})
	return nil
}

// twoFactorSetup deals a new secret for the person to add to their authenticator app. Nothing changes
// until a code from it is confirmed.
func (s *Server) twoFactorSetup(w http.ResponseWriter, r *http.Request, u *User) error {
	if u.TwoFactor {
		return fail(409, "Two-step sign-in is already on")
	}
	secret := newTOTPSecret()
	issuer := "Fond"
	if host := strings.Split(strings.TrimPrefix(strings.TrimPrefix(s.origin(r), "https://"), "http://"), ":")[0]; host != "" && host != "localhost" {
		issuer = "Fond (" + host + ")"
	}
	uri := "otpauth://totp/" + url.PathEscape(issuer+":"+u.Username) + "?" + url.Values{
		"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"},
	}.Encode()
	s.writeJSON(w, r, 200, map[string]string{"secret": secret, "uri": uri})
	return nil
}

func (s *Server) twoFactorEnable(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct{ Secret, Code string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if u.TwoFactor {
		return fail(409, "Two-step sign-in is already on")
	}
	if !s.limit.hit("2fa:"+itoa(int(u.ID)), 10, 10*time.Minute) {
		return fail(429, "Too many attempts. Try again in a few minutes.")
	}
	secret := strings.ToUpper(strings.ReplaceAll(in.Secret, " ", ""))
	if _, err := b32.DecodeString(secret); err != nil || len(secret) < 26 {
		return fail(400, "Start again to get a new code")
	}
	step := checkTOTP(secret, in.Code, time.Now(), 0)
	if step == 0 {
		return fail(400, "That code doesn't match. Check the time on your phone and try the newest code.")
	}
	plain, hashed := newRecoveryCodes()
	if err := s.store.Exec(`UPDATE users SET totp_secret=?, totp_step=?, recovery=? WHERE id=?`, secret, step, js(hashed), u.ID); err != nil {
		return err
	}
	s.signOutOthers(r, u.ID)
	s.writeJSON(w, r, 200, map[string]any{"recovery": plain})
	return nil
}

// confirmIdentity asks for the password again (or a current code) before a change to how someone signs in.
func (s *Server) confirmIdentity(u *User, password string) error {
	if !s.limit.hit("pw:"+itoa(int(u.ID)), 8, 10*time.Minute) {
		return fail(429, "Too many attempts. Try again in a few minutes.")
	}
	if !checkPassword(u.passHash, password) {
		return fail(403, "Your password isn't right")
	}
	return nil
}

func (s *Server) twoFactorDisable(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct{ Password string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if err := s.confirmIdentity(u, in.Password); err != nil {
		return err
	}
	if err := s.store.Exec(`UPDATE users SET totp_secret='', totp_step=0, recovery='[]' WHERE id=?`, u.ID); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) twoFactorRecovery(w http.ResponseWriter, r *http.Request, u *User) error {
	var in struct{ Password string }
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	if !u.TwoFactor {
		return fail(409, "Two-step sign-in is off")
	}
	if err := s.confirmIdentity(u, in.Password); err != nil {
		return err
	}
	plain, hashed := newRecoveryCodes()
	if err := s.store.Exec(`UPDATE users SET recovery=? WHERE id=?`, js(hashed), u.ID); err != nil {
		return err
	}
	s.writeJSON(w, r, 200, map[string]any{"recovery": plain})
	return nil
}

// signOutOthers ends every session of the user but the one making the request.
func (s *Server) signOutOthers(r *http.Request, uid int64) {
	keep := ""
	if c, err := r.Cookie(cookieName); err == nil {
		keep = hashToken(c.Value)
	}
	s.store.Exec(`DELETE FROM sessions WHERE user_id=? AND id<>?`, uid, keep)
}

// ResetTwoFactor turns two-step sign-in off for a user, for when they have lost their phone and their codes.
func ResetTwoFactor(st *Store, username string) error {
	u, err := st.UserByName(username)
	if err != nil {
		return err
	}
	return st.Exec(`UPDATE users SET totp_secret='', totp_step=0, recovery='[]' WHERE id=?`, u.ID)
}
