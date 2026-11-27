// Package leather draws the hide a cookbook is bound in. Every seed gives a
// different one: the dye, how unevenly it took, where the leather has been
// flexed and has creased pale, the grain, and whether the finish is matte,
// satin or slightly shiny. The same seed always gives the same hide.
//
// The look is pull-up leather: dyed through, so that wherever the hide is
// worked the dye thins and the warm tan underneath shows. That gives long
// creases, a fine mesh of smaller ones gathered round them, and colour that
// lifts and deepens in soft patches. Nothing here is a scratch or a stain.
//
// Changing anything in this package changes what a seed draws. The server
// keeps each hide it has drawn in the data folder, so books already bound keep
// their covers; only hides drawn afterwards differ.
package leather

import (
	"image"
	"math"
	"runtime"
	"sync"
)

// Finish is how the surface takes the light.
type Finish int

const (
	Matte Finish = iota // soft and flat: about two hides in three
	Satin               // a low sheen: one in four
	Shiny               // lightly polished: one in ten
)

func (f Finish) String() string { return [...]string{"matte", "satin", "shiny"}[f] }

// vein is one family of long creases.
type vein struct {
	freq, aniso, cos, sin float64 // how many cross the cover, how drawn out they are, and which way they run
	width, amount         float64
	seed, mask            uint32
}

// web is one scale of the crease mesh.
type web struct {
	freq, aniso, cos, sin float64
	width, amount, depth  float64
	bend, shake           float64 // how far its lines sway between junctions, and tremble along the way
	seed                  uint32
}

// Hide is one piece of leather, fully decided, not yet drawn.
type Hide struct {
	Finish    Finish
	Character string

	base, light, dark, crease rgb

	warpF, warpA           float64
	cloudF, cloudA         float64 // broad uneven dye
	patchF, patchA         float64 // smaller patches
	flexF, flexBias, flexA float64 // where the hide has been worked: creases gather and the colour lifts
	bands, runCos, runSin  float64 // how far that working runs in bands along the hide rather than in patches
	veins                  [2]vein
	mesh, hair             web     // the mesh of creases, and the hair-fine grain
	relief, kd             float64 // how deep the creases are cut, and how hard the light models them
	sheen, sheenW, glint   float64 // a band of light lying across the cover: how much it lifts the colour, how wide it is, how much pure reflection rides on it
	deep                   float64 // a polished surface reads a little deeper in colour
	hollow                 float64 // how much deeper the dye lies between creases than along them

	sWarp, sCloud, sPatch, sFlex, sLine, sSwell, sDither uint32
}

// FinishOf is the finish a seed draws, without deciding the rest.
func FinishOf(seed uint32) Finish { return New(seed, "").Finish }

// New decides a hide from a seed and a cover colour. An unknown colour is bound in bay.
func New(seed uint32, color string) *Hide {
	r := &rng{s: uint64(seed)*0x9e3779b97f4a7c15 + 0x5851f42d4c957f2d}
	h := &Hide{}

	// The finish is drawn first and on its own, so the odds are exact.
	switch u := r.float(); {
	case u < 0.65:
		h.Finish = Matte
	case u < 0.90:
		h.Finish = Satin
	default:
		h.Finish = Shiny
	}

	run := r.in(0, math.Pi) // the way the hide's creases mostly run
	h.runCos, h.runSin = math.Cos(run), math.Sin(run)

	// What kind of leather this is. Each kind sets the broad balance; everything is then drawn within it.
	var veinA, veinAniso [2]float64
	var meshA, hairA float64
	reach := 1.0 // how far apart the hide's lightest and deepest tones stand
	switch u := r.float(); {
	case u < 0.30:
		h.Character = "pull-up" // a bright mesh of creases over a deeper ground
		veinA, veinAniso = [2]float64{r.in(0.55, 1), r.in(0.2, 0.7)}, [2]float64{r.in(0.45, 1), r.in(0.5, 1)}
		meshA, hairA = r.in(0.42, 0.68), r.in(0.24, 0.42)
		h.cloudA, h.patchA, h.flexA = r.in(0.12, 0.26), r.in(0.04, 0.12), r.in(0.16, 0.3)
	case u < 0.50:
		h.Character = "creased" // long folds running one way, mesh gathered along them
		veinA, veinAniso = [2]float64{r.in(0.7, 1), r.in(0.2, 0.55)}, [2]float64{r.in(0.1, 0.28), r.in(0.25, 0.6)}
		meshA, hairA = r.in(0.32, 0.58), r.in(0.2, 0.36)
		h.cloudA, h.patchA, h.flexA = r.in(0.1, 0.24), r.in(0.04, 0.1), r.in(0.14, 0.26)
	case u < 0.82:
		h.Character = "waxed" // clouded, two-toned, nearly smooth
		veinA, veinAniso = [2]float64{r.in(0, 0.35), r.in(0, 0.2)}, [2]float64{r.in(0.3, 1), r.in(0.4, 1)}
		meshA, hairA = r.in(0.1, 0.3), r.in(0.16, 0.32)
		h.cloudA, h.patchA, h.flexA = r.in(0.34, 0.58), r.in(0.08, 0.18), r.in(0.1, 0.2)
		reach = r.in(1.0, 1.2)
	default:
		h.Character = "calm" // even and quiet: a fine grain and little else
		veinA, veinAniso = [2]float64{r.in(0, 0.25), r.in(0, 0.15)}, [2]float64{r.in(0.3, 1), r.in(0.4, 1)}
		meshA, hairA = r.in(0.2, 0.4), r.in(0.24, 0.4)
		h.cloudA, h.patchA, h.flexA = r.in(0.08, 0.18), r.in(0.03, 0.08), r.in(0.06, 0.14)
	}

	h.warpF, h.warpA = r.in(1.6, 3.2), r.in(0.025, 0.06)
	h.cloudF, h.patchF = r.in(0.55, 1.3), r.in(2.4, 5)
	h.flexF, h.flexBias, h.bands = r.in(1.1, 2.6), r.in(-0.15, 0.2), r.in(0, 0.6)
	r.float() // a draw once spent on hard-edged clouds; it is still made so that a seed keeps its hide

	vf := r.in(3.2, 6.5)
	for i := range h.veins {
		// Long creases lie along the run: the noise is stretched that way, and its lines follow the stretch.
		a := run + math.Pi/2 + r.in(-0.3, 0.3) + float64(i)*r.in(-0.5, 0.5)
		h.veins[i] = vein{
			freq: vf, aniso: veinAniso[i], cos: math.Cos(a), sin: math.Sin(a),
			width: [2]float64{15, 11}[i] * r.in(0.8, 1.25), amount: veinA[i], seed: r.seed(), mask: r.seed(),
		}
		vf *= r.in(1.5, 2.3)
	}
	weave := func(freq, half, amount, depth float64) web {
		a := run + r.in(-0.4, 0.4)
		return web{
			freq: freq, aniso: r.in(0.5, 1), cos: math.Cos(a), sin: math.Sin(a),
			width: freq * half * r.in(0.85, 1.2), amount: amount, depth: depth,
			bend: r.in(0.15, 0.4) / freq, shake: r.in(0.06, 0.12) / freq, seed: r.seed(),
		}
	}
	cellsAcross := r.in(32, 62)
	h.mesh = weave(cellsAcross, 0.0018, meshA, 0.6)
	h.hair = weave(cellsAcross*r.in(2.2, 3), 0.001, hairA, 0.3)
	h.hair.aniso *= 0.75

	h.polish(r.float(), r.float())

	// The dye, and the tones it breaks into.
	dyes, ok := Dyes[color]
	if !ok {
		dyes = Dyes["bay"]
	}
	d := hex(dyes[int(r.float()*float64(len(dyes)))]).lch()
	d.l += r.in(-0.02, 0.02)
	d.c *= r.in(0.88, 1.12)
	d.h += r.in(-5, 5)
	h.base = d.rgb()
	// Where leather is worked the dye thins: the colour lifts, and along a crease the warm tan of the
	// hide underneath begins to show. Where it has been held it deepens. A warm dye grows lighter and
	// richer as it lifts. A cool or neutral one would only turn dusty, so it shifts in hue instead, the
	// way blue leather wears toward teal and green toward olive. A pale hide has little room to lift,
	// so it mellows.
	warm := 0.5 + 0.5*math.Cos((d.h-70)*math.Pi/180)
	if d.c < 0.03 {
		warm = 0.15     // near-neutral dyes have no hue to speak of,
		h.cloudA *= 0.7 // and their lighter patches would read as smoke
	}
	lift := r.in(0.7, 1.0) * reach
	room := clamp(0.9-d.l, 0.04, 0.6)
	light := lch{
		d.l + lift*mix(0.02+0.04*room, 0.032+0.09*room, warm),
		d.c*mix(r.in(1.05, 1.25), r.in(1.05, 1.22), warm) + (1-warm)*r.in(0.004, 0.014)*clamp(d.c/0.03, 0.3, 1),
		toward(d.h, 72, mix(r.in(10, 26), r.in(3, 10), warm)),
	}
	crease := lch{
		d.l + lift*mix(0.045+0.1*room, 0.055+0.15*room, warm),
		d.c * mix(0.9, 1.1, warm) * r.in(0.92, 1.08),
		toward(d.h, 76, mix(r.in(8, 20), r.in(5, 14), warm)),
	}
	crust := lch{math.Max(r.in(0.74, 0.86), crease.l), r.in(0.07, 0.12) * (0.4 + 0.6*warm), r.in(64, 84)}
	dark := lch{d.l - (0.5+0.5*reach)*r.in(0.024, 0.052)*(0.5+d.l), d.c * r.in(1.0, 1.12), toward(d.h, 25, r.in(0, 7))}
	h.hollow = 1
	// On a pale hide a patch of deeper colour reads as a stain, so its colour is kept nearly even.
	if pale := smooth(0.5, 0.75, d.l); pale > 0 {
		dark = dark.blend(lch{d.l - r.in(0.04, 0.07), d.c*1.25 + 0.012, toward(d.h, 66, 14)}, pale)
		h.cloudA *= 1 - 0.78*pale
		h.patchA *= 1 - 0.78*pale
		h.hollow = 1 + 1.6*pale // a pale hide shows its creases by the tone between them
	}
	h.light, h.dark = light.rgb(), dark.rgb()
	h.crease = crease.blend(crust, r.in(0.1, 0.24)*(0.5+0.5*warm)).rgb()

	h.sWarp, h.sCloud, h.sPatch, h.sFlex = r.seed(), r.seed(), r.seed(), r.seed()
	h.sLine, h.sSwell, h.sDither = r.seed(), r.seed(), r.seed()
	return h
}

// polish sets how the surface takes the light for the hide's finish; a and b vary it within the finish.
func (h *Hide) polish(a, b float64) {
	switch h.Finish {
	case Matte:
		h.relief, h.kd, h.sheen, h.sheenW, h.glint, h.deep = 0.7, 0.8, 0, 1, 0, 1
	case Satin:
		h.relief, h.kd, h.sheen, h.sheenW, h.glint, h.deep = 1.0, 1.0, mix(0.3, 0.42, a), mix(0.5, 0.68, b), 0.004, 0.95
	default:
		h.relief, h.kd, h.sheen, h.sheenW, h.glint, h.deep = 1.3, 1.25, mix(0.55, 0.75, a), mix(0.36, 0.5, b), 0.014, 0.88
	}
}

// The smooth quantities of a hide are worked out on a coarse grid and read between its points.
const (
	fWarpX = iota // a slow drift of the whole surface, so nothing lies on a lattice
	fWarpY
	fCloud // broad uneven dye
	fPatch // smaller patches
	fFlex  // where the hide has been worked
	fBendX // a sway that bends the mesh between junctions
	fBendY
	fRuns  // where creases run on strongly
	fVein0 // where each family of long creases shows
	fVein1
	fSwell // the surface rising and falling a little
	fields
)

type grid struct {
	w, step int
	v       [fields][]float32
}

// spot is a place between four points of the grid, and how much each of them counts there.
type spot struct {
	p          int
	a, b, c, d float32
}

func (g *grid) spot(x, y int) spot {
	i, j := x/g.step, y/g.step
	tx, ty := float32(x-i*g.step)/float32(g.step), float32(y-j*g.step)/float32(g.step)
	return spot{j*g.w + i, (1 - tx) * (1 - ty), tx * (1 - ty), (1 - tx) * ty, tx * ty}
}

func (g *grid) at(f int, s spot) float64 {
	v := g.v[f]
	return float64(v[s.p]*s.a + v[s.p+1]*s.b + v[s.p+g.w]*s.c + v[s.p+g.w+1]*s.d)
}

// Render draws the hide w pixels wide and four-thirds as tall, the shape of a cover.
func (h *Hide) Render(w int) *image.RGBA {
	ht := w * 4 / 3
	px := 1 / float64(w)

	each := func(n int, fn func(i int)) {
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); fn(i) }()
		}
		wg.Wait()
	}
	rows := func(fn func(y0, y1 int)) {
		n := min(runtime.NumCPU(), 4)
		each(n, func(i int) { fn(ht*i/n, ht*(i+1)/n) })
	}

	g := &grid{step: max(2, w/240)}
	g.w = w/g.step + 2
	gh := ht/g.step + 2
	smoothly := [fields]func(u, v float64) float64{
		fWarpX: func(u, v float64) float64 { return h.warpA * fbm(u*h.warpF, v*h.warpF, 3, h.sWarp) },
		fWarpY: func(u, v float64) float64 { return h.warpA * fbm(u*h.warpF+31.4, v*h.warpF+7.9, 3, h.sWarp+1) },
		fCloud: func(u, v float64) float64 {
			wx, wy := 0.13*fbm(u*1.7+3, v*1.7, 2, h.sCloud+9), 0.13*fbm(u*1.7, v*1.7+5, 2, h.sCloud+10)
			return fbm((u+wx)*h.cloudF, (v+wy)*h.cloudF, 2, h.sCloud)
		},
		fPatch: func(u, v float64) float64 { return fbm(u*h.patchF, v*h.patchF, 2, h.sPatch) },
		fFlex: func(u, v float64) float64 {
			blobs := 0.7*fbm(u*h.flexF, v*h.flexF, 3, h.sFlex) + 0.3*fbm(u*h.flexF*2.9, v*h.flexF*2.9, 2, h.sFlex+3)
			// Bands: broad soft ridges lying along the run of the hide.
			su, sv := u+0.16*fbm(u*2.1, v*2.1, 2, h.sFlex+11), v+0.16*fbm(u*2.1+8, v*2.1+3, 2, h.sFlex+12)
			a, b := (su*h.runCos+sv*h.runSin)*h.flexF*0.55, (sv*h.runCos-su*h.runSin)*h.flexF*1.6
			band := 1 - 2.2*math.Abs(fbm(a, b, 3, h.sFlex+7))
			return mix(blobs, 0.6*band-0.1+0.4*blobs, h.bands*0.7)
		},
		fBendX: func(u, v float64) float64 { return fbm(u*13, v*13, 2, h.sLine) },
		fBendY: func(u, v float64) float64 { return fbm(u*13+17.3, v*13+2.6, 2, h.sLine+1) },
		fRuns:  func(u, v float64) float64 { return fbm(u*9, v*9, 3, h.sLine+21) },
		fVein0: func(u, v float64) float64 { return fbm(u*2.3, v*2.3, 2, h.veins[0].mask) },
		fVein1: func(u, v float64) float64 { return fbm(u*2.3, v*2.3, 2, h.veins[1].mask) },
		fSwell: func(u, v float64) float64 { return fbm(u*1.3, v*1.3, 2, h.sSwell) },
	}
	each(fields, func(f int) {
		v := make([]float32, g.w*gh)
		for j := 0; j < gh; j++ {
			for i := 0; i < g.w; i++ {
				v[j*g.w+i] = float32(smoothly[f](float64(i*g.step)*px, float64(j*g.step)*px))
			}
		}
		g.v[f] = v
	})

	height := make([]float32, w*ht)
	facet := make([]float32, w*ht) // which way each grain of the surface happens to tilt
	albedo := make([]float32, w*ht*3)

	// The surface: its colour and its height at every point.
	rows(func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				u, v := (float64(x)+0.5)*px, (float64(y)+0.5)*px
				at := g.spot(x, y)
				qx, qy := u+g.at(fWarpX, at), v+g.at(fWarpY, at)

				cl := g.at(fCloud, at) * 1.7
				// 0 where the hide lies quiet, 1 where it has been flexed; a patch that has lifted in colour has been worked too.
				worked := clamp(smooth(-0.3, 0.4, g.at(fFlex, at)+h.flexBias)+0.35*h.cloudA*math.Max(cl, 0), 0, 1)
				run := smooth(-0.25, 0.45, g.at(fRuns, at))
				creased, fine, lifted, hollow, groove, glow := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0

				// A long fold is not drawn as a line: a line across a cover reads as a scratch. It is a band of
				// strain. Within it the colour lifts softly and the mesh gathers, and the borders that lie right
				// along it crease a little harder than their neighbours.
				strain, near := 0.0, 0.0
				for i := range h.veins {
					vn := &h.veins[i]
					if vn.amount < 0.03 {
						continue
					}
					m := smooth(-0.45, 0.25, g.at(fVein0+i, at)) * (0.55 + 0.45*run)
					if m < 0.02 {
						continue
					}
					fx := (qx*vn.cos + qy*vn.sin) * vn.freq
					fy := (qy*vn.cos - qx*vn.sin) * vn.freq * vn.aniso
					n, gx, gy := ridge(fx, fy, vn.seed)
					// How far this point is from the line, in thousandths of the cover's width.
					slope := math.Hypot(gx, gy*vn.aniso)
					dist := 1000 * math.Abs(n) / (vn.freq * (slope + 0.3))
					if dist > 70 {
						continue
					}
					a := vn.amount * m * smooth(0.3, 0.9, slope) // where the noise lies flat it has no line, only a smear: leave it out
					strain = math.Max(strain, 0.2*a*math.Exp(-dist*dist/(vn.width*vn.width)))
					near = math.Max(near, a*math.Exp(-dist*dist/1800))
				}
				worked = math.Max(worked, 0.9*near)
				glow = 0.5 * near

				// The mesh sways between junctions and trembles along the way, so no line of it runs ruler-straight.
				n1, n2, _ := gnoiseD(u*h.mesh.freq*1.3, v*h.mesh.freq*1.3, h.sLine+5)
				n2 = clamp(0.3*n2, -1, 1) // the slope of the same noise does for the second direction
				bx, by := g.at(fBendX, at), g.at(fBendY, at)
				grainTone := 0.0
				for i, wb := range [2]*web{&h.mesh, &h.hair} {
					if wb.amount < 0.02 {
						continue
					}
					wx, wy := qx+bx*wb.bend+n1*wb.shake, qy+by*wb.bend+n2*wb.shake
					cx := (wx*wb.cos + wy*wb.sin) * wb.freq
					cy := (wy*wb.cos - wx*wb.sin) * wb.freq * wb.aniso
					e, id1, id2 := cells(cx, cy, wb.seed)
					// Each border between two cells has its own strength and width, and some never creased at all.
					r1, r2 := pair(id1, id2)
					var a, wide float64
					if i == 0 {
						a = (0.4 + 0.6*worked) * (0.45 + 0.55*run) * (0.75 + 0.25*r1) * (0.65 + 0.35*smooth(-0.35, 0.25, n2)) // it fades along its length
						if r2 < 0.2 {
							a *= strain // a border that never creased still does where a long crease crosses it
						}
						a = wb.amount * math.Max(a, strain*(0.8+0.2*r1))
						wide = (0.75 + 0.6*r2) * (0.85 + 0.4*n1) * (1 + 0.6*strain)
					} else {
						grainTone = float64(id1>>8&0xffff)/65535 - 0.5 // each grain took the dye a little differently
						facet[y*w+x] = float32(id1&0xff)/127.5 - 1
						a = wb.amount * (0.6 + 0.4*worked) * (0.7 + 0.3*r1)
						wide = 0.75 + 0.5*r2
					}
					if a < 0.01 {
						continue
					}
					// A crease has no edge. The colour is palest along its line, falls away quickly at first and
					// then slowly, and is still lifting the leather a little halfway across the cell.
					width := wb.width * wide
					core := math.Exp(-e * e / (width * width))
					shoulder := math.Exp(-e / (2.6 * width))
					bloom := math.Exp(-e / (7 * width))
					groove += wb.depth * a * (0.6*core + 0.4*shoulder)
					if i == 1 {
						// The grain is texture, not creasing: its lines lift a little and the grains between them sit a little deeper.
						fine = a * (0.6*core + 0.5*shoulder + 0.15*bloom - 0.3)
						continue
					}
					creased = 1 - (1-creased)*(1-a*(0.4*core+0.3*shoulder))
					lifted = a * (0.5*shoulder + 0.4*bloom)    // round the crease the leather passes through its lighter tone on the way
					hollow = 0.15 * h.hollow * a * (1 - bloom) // between its creases the dye lies a little deeper
					glow = math.Max(glow, 0.5*a*shoulder)
				}

				// Uneven colour: broad clouds, smaller patches, lift where worked, and grain by grain so nothing is airbrushed.
				tone := h.cloudA*cl + h.patchA*g.at(fPatch, at)*1.5 + h.flexA*(worked*(0.85+0.15*run)-0.5) + 0.2*glow
				tone += (0.08+0.15*math.Abs(tone))*grainTone - hollow + 0.6*lifted
				c := h.base
				if tone > 0 {
					c = c.mix(h.light, soft(tone))
				} else {
					c = c.mix(h.dark, soft(-tone))
				}
				if fine > 0 {
					c = c.mix(h.light, fine)
				} else {
					c = c.mix(h.dark, -fine)
				}
				c = c.mix(h.crease, clamp(creased, 0, 1))
				p := (y*w + x) * 3
				albedo[p], albedo[p+1], albedo[p+2] = float32(c[0]), float32(c[1]), float32(c[2])
				height[y*w+x] = float32(-0.0011*h.relief*groove + 0.01*g.at(fSwell, at))
			}
		}
	})

	// The light: a soft key from the upper left that models the creases. Satin and shiny hides also catch
	// a broad band of reflected light lying across the upper left of the cover, which every raised edge
	// that faces it picks up a little early.
	img := image.NewRGBA(image.Rect(0, 0, w, ht))
	lx, ly, lz := -0.5, -0.62, 0.6
	ll := math.Sqrt(lx*lx + ly*ly + lz*lz)
	lx, ly, lz = lx/ll, ly/ll, lz/ll
	const bandX, bandY = 0.6, 0.8 // the band lies across this direction
	var curve [4097]float32
	for i := range curve {
		curve[i] = float32(encode(float64(i) / 4096))
	}
	rows(func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				xl, xr, yu, yd := max(x-1, 0), min(x+1, w-1), max(y-1, 0), min(y+1, ht-1)
				nx := -float64(height[y*w+xr]-height[y*w+xl]) / (float64(xr-xl) * px)
				ny := -float64(height[yd*w+x]-height[yu*w+x]) / (float64(yd-yu) * px)
				nl := 1 / math.Sqrt(nx*nx+ny*ny+1)
				nx, ny, nz := nx*nl, ny*nl, nl

				lit := h.deep * (1 + h.kd*((nx*lx+ny*ly+nz*lz)-lz)/lz*0.5)
				glare := 0.0
				if h.sheen > 0 {
					// Where the band falls depends on where the point is and which way its surface leans:
					// every crease edge and every grain catches it a little early or late, so it sparkles.
					u, v := (float64(x)+0.5)*px, (float64(y)+0.5)*px
					tilt := nx*bandX + ny*bandY + 0.04*float64(facet[y*w+x])
					t := (u*bandX + v*bandY - 0.3 + 1.6*tilt) / h.sheenW
					band := math.Exp(-t * t)
					lit *= 1 + h.sheen*band
					glare = h.glint * band * (1 + 2.5*smooth(0.03, 0.3, -tilt))
				}

				p := (y*w + x) * 3
				dither := (float64(hash2(int32(x), int32(y), h.sDither)&255)/255 - 0.5) / 255
				o := img.PixOffset(x, y)
				for i, tint := range [3]float64{1, 0.97, 0.92} {
					lin := clamp(float64(albedo[p+i])*lit+glare*tint, 0, 1) * 4096
					k := int(lin)
					enc := float64(curve[k]) + float64(curve[min(k+1, 4096)]-curve[k])*(lin-float64(k))
					img.Pix[o+i] = uint8(clamp(enc+dither, 0, 1)*255 + 0.5)
				}
				img.Pix[o+3] = 255
			}
		}
	})
	return img
}

// Half shrinks a drawing to half its size, averaging four pixels into one.
func Half(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx()/2, b.Dy()/2
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := dst.PixOffset(x, y)
			a, c := src.PixOffset(2*x, 2*y), src.PixOffset(2*x, 2*y+1)
			for i := 0; i < 4; i++ {
				dst.Pix[o+i] = uint8((int(src.Pix[a+i]) + int(src.Pix[a+4+i]) + int(src.Pix[c+i]) + int(src.Pix[c+4+i]) + 2) / 4)
			}
		}
	}
	return dst
}
