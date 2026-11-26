package app

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/au12/fond/internal/leather"
)

// Cookbook covers are bound in leather. Each book carries a seed, and the hide
// is drawn from the seed and the cover colour the first time it is asked for,
// in two sizes, and kept in the data folder from then on. Keeping it is what
// makes a cover permanent: a book shows the hide it was given even if the
// drawing is improved later.

const leatherWide = 800 // the large size; the small one is half that

// /leather/<seed>-<colour>-<s|l>.jpg
var leatherName = regexp.MustCompile(`^([0-9]{1,10})-([a-z]{1,20})-([sl])\.jpg$`)

// newLeather picks the seed for a new cover.
func newLeather() uint32 {
	var b [4]byte
	rand.Read(b[:])
	if n := binary.LittleEndian.Uint32(b[:]); n != 0 {
		return n
	}
	return 1
}

func (s *Server) handleLeather(w http.ResponseWriter, r *http.Request) {
	m := leatherName.FindStringSubmatch(r.PathValue("name"))
	if m == nil {
		http.NotFound(w, r)
		return
	}
	seed, err := strconv.ParseUint(m[1], 10, 32)
	if _, known := leather.Dyes[m[2]]; err != nil || seed == 0 || !known {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.cfg.DataDir, "leather", m[0])
	f, err := os.Open(path)
	if err != nil {
		// Not drawn yet, and drawing takes real work. Someone signed in may have any hide drawn: they
		// are choosing a cover. A visitor may only have the cover of a book that has been shared.
		if _, who := s.identify(r); who != nil {
			var shared int
			s.store.db.QueryRow(`SELECT COUNT(*) FROM cookbooks WHERE leather=? AND color=? AND share_token IS NOT NULL`, seed, m[2]).Scan(&shared)
			if shared == 0 {
				http.NotFound(w, r)
				return
			}
		}
		if err := s.drawLeather(uint32(seed), m[2]); err != nil {
			http.Error(w, "could not draw the cover", http.StatusInternalServerError)
			return
		}
		if f, err = os.Open(path); err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// drawLeather draws one hide in both sizes and keeps them. Two requests for
// the same hide draw it once, and no more than two hides are drawn at a time.
func (s *Server) drawLeather(seed uint32, color string) error {
	dir := filepath.Join(s.cfg.DataDir, "leather")
	name := func(size string) string { return filepath.Join(dir, fmt.Sprintf("%d-%s-%s.jpg", seed, color, size)) }

	lock, _ := s.hides.LoadOrStore(name(""), &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer func() {
		s.hides.Delete(name(""))
		lock.(*sync.Mutex).Unlock()
	}()
	if _, err := os.Stat(name("l")); err == nil {
		if _, err := os.Stat(name("s")); err == nil {
			return nil
		}
	}
	s.easel <- struct{}{}
	defer func() { <-s.easel }()

	large := leather.New(seed, color).Render(leatherWide)
	save := func(path string, img image.Image, quality int) error {
		tmp, err := os.CreateTemp(dir, "drawing-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: quality}); err != nil {
			tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		os.Chmod(tmp.Name(), 0o644)
		return os.Rename(tmp.Name(), path)
	}
	if err := save(name("l"), large, 82); err != nil {
		return err
	}
	return save(name("s"), leather.Half(large), 84)
}

// drawMissingLeather draws the cover of every book that has none on file, one
// after another: after an upgrade from before covers had hides, or when the
// leather folder has been emptied. It runs in the background at start, so the
// shelf does not have to wait for them.
func (s *Server) drawMissingLeather() {
	rows, err := s.store.db.Query(`SELECT leather, color FROM cookbooks`)
	if err != nil {
		return
	}
	type hide struct {
		seed  uint32
		color string
	}
	var hides []hide
	for rows.Next() {
		var h hide
		if rows.Scan(&h.seed, &h.color) == nil {
			hides = append(hides, h)
		}
	}
	rows.Close()
	for _, h := range hides {
		if _, known := leather.Dyes[h.color]; known && h.seed != 0 {
			s.drawLeather(h.seed, h.color)
		}
	}
}

// pruneLeather throws away hides no book is bound in: the ones tried and
// passed over while choosing a cover, and those of books since deleted.
func (s *Server) pruneLeather() {
	dir := filepath.Join(s.cfg.DataDir, "leather")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	rows, err := s.store.db.Query(`SELECT leather, color FROM cookbooks`)
	if err != nil {
		return
	}
	bound := map[string]bool{}
	for rows.Next() {
		var seed uint32
		var color string
		rows.Scan(&seed, &color)
		bound[fmt.Sprintf("%d-%s", seed, color)] = true
	}
	rows.Close()
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < 24*time.Hour {
			continue
		}
		if m := leatherName.FindStringSubmatch(e.Name()); m == nil || !bound[m[1]+"-"+m[2]] {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
