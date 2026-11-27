package leather

import (
	"bytes"
	"testing"
)

func BenchmarkRender(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New(uint32(i+1), "chestnut").Render(960)
	}
}

// A seed is a promise: the same seed and colour must always draw the same hide.
func TestSameSeedSameHide(t *testing.T) {
	a := New(20261126, "tomato").Render(120)
	b := New(20261126, "tomato").Render(120)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("the same seed drew two different hides")
	}
	c := New(20261127, "tomato").Render(120)
	if bytes.Equal(a.Pix, c.Pix) {
		t.Fatal("two seeds drew the same hide")
	}
}

// The finish is drawn at fixed odds: about 65 matte, 25 satin and 10 shiny in a hundred.
func TestFinishOdds(t *testing.T) {
	const n = 40000
	var got [3]int
	for seed := uint32(1); seed <= n; seed++ {
		got[FinishOf(seed)]++
	}
	for f, want := range [3]float64{0.65, 0.25, 0.10} {
		if share := float64(got[f]) / n; share < want-0.012 || share > want+0.012 {
			t.Errorf("%v: %.3f of hides, want about %.2f", Finish(f), share, want)
		}
	}
}

// One hide in five is lightly scratched, whatever its finish, and the scratches change nothing else about it.
func TestScuffOdds(t *testing.T) {
	const n = 40000
	var scuffed, shinyScuffed, shiny int
	for seed := uint32(1); seed <= n; seed++ {
		h := New(seed, "bay")
		if h.Scuffed != (len(h.scuffs) > 0) {
			t.Fatalf("seed %d: scuffed %v with %d scratches", seed, h.Scuffed, len(h.scuffs))
		}
		if h.Scuffed {
			scuffed++
		}
		if h.Finish == Shiny {
			shiny++
			if h.Scuffed {
				shinyScuffed++
			}
		}
	}
	if share := float64(scuffed) / n; share < 0.188 || share > 0.212 {
		t.Errorf("%.3f of hides are scratched, want about 0.20", share)
	}
	if share := float64(shinyScuffed) / float64(shiny); share < 0.16 || share > 0.24 {
		t.Errorf("%.3f of shiny hides are scratched, want about 0.20", share)
	}
}

// Every cover colour has dyes, and an unknown colour still binds a book.
func TestEveryColourDraws(t *testing.T) {
	for color := range Dyes {
		if im := New(7, color).Render(60); im.Bounds().Dx() != 60 || im.Bounds().Dy() != 80 {
			t.Errorf("%s: drew %v", color, im.Bounds())
		}
	}
	if New(7, "no-such-colour").base != New(7, "bay").base {
		t.Error("an unknown colour should be bound in bay")
	}
}

// Seeds next to each other must not look alike: neighbours should differ in kind, finish or dye.
func TestNeighbouringSeedsDiffer(t *testing.T) {
	same := 0
	for seed := uint32(1); seed < 2000; seed++ {
		a, b := New(seed, "chestnut"), New(seed+1, "chestnut")
		if a.Character == b.Character && a.Finish == b.Finish && a.base == b.base {
			same++
		}
	}
	if same > 0 {
		t.Errorf("%d neighbouring seeds drew the same kind, finish and dye", same)
	}
}
