package leather

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSheet writes a contact sheet of hides for looking at while tuning:
//
//	LEATHER_SHEET=/tmp/sheet.png go test ./internal/leather -run Sheet
//
// LEATHER_COLORS picks the colours (comma separated), LEATHER_N how many of
// each, LEATHER_W the width of each, LEATHER_FROM the first seed, and
// LEATHER_KIND and LEATHER_FINISH keep only hides of that character or finish,
// and LEATHER_SCUFFED=1 only the ones with scratches.
// LEATHER_POLISH=1 draws each hide three times, matte, satin and shiny, to compare the finishes.
func TestSheet(t *testing.T) {
	out := os.Getenv("LEATHER_SHEET")
	if out == "" {
		t.Skip("set LEATHER_SHEET to a file name to draw a contact sheet")
	}
	colors := []string{"bay", "tomato", "butter", "ocean", "plum", "charcoal", "paper", "chestnut"}
	if s := os.Getenv("LEATHER_COLORS"); s != "" {
		colors = strings.Split(s, ",")
	}
	num := func(key string, def int) int {
		if n, err := strconv.Atoi(os.Getenv(key)); err == nil {
			return n
		}
		return def
	}
	n, w, from := num("LEATHER_N", 6), num("LEATHER_W", 300), num("LEATHER_FROM", 1)
	h := w * 4 / 3
	sheet := image.NewRGBA(image.Rect(0, 0, n*(w+8), len(colors)*(h+8)))
	var took time.Duration
	seed := uint32(from) - 1
	kind, finish := os.Getenv("LEATHER_KIND"), os.Getenv("LEATHER_FINISH")
	for j, c := range colors {
		for i := 0; i < n; i++ {
			var hide *Hide
			for {
				seed++
				hide = New(seed, c)
				if (kind == "" || hide.Character == kind) && (finish == "" || hide.Finish.String() == finish) && (os.Getenv("LEATHER_SCUFFED") == "" || hide.Scuffed) {
					break
				}
			}
			if os.Getenv("LEATHER_POLISH") != "" {
				hide.Finish = Finish(i % 3)
				hide.polish(0.5, 0.5)
				if i%3 != 2 {
					seed--
				}
			}
			start := time.Now()
			var im *image.RGBA
			if os.Getenv("LEATHER_HALF") != "" {
				im = Half(hide.Render(2 * w)) // as the small covers are made
			} else {
				im = hide.Render(w)
			}
			took += time.Since(start)
			draw.Draw(sheet, image.Rect(i*(w+8), j*(h+8), i*(w+8)+w, j*(h+8)+h), im, image.Point{}, draw.Src)
			fmt.Printf("%s %d: %s, %s%s\n", c, seed, hide.Character, hide.Finish, map[bool]string{true: ", scuffed"}[hide.Scuffed])
		}
	}
	fmt.Printf("%v per hide at %d wide\n", took/time.Duration(n*len(colors)), w)
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, sheet); err != nil {
		t.Fatal(err)
	}
}
