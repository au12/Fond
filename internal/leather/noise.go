package leather

import "math"

// rng is splitmix64: tiny, fast, and the same on every machine, so a seed
// always draws the same hide.
type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// float returns a number in [0, 1).
func (r *rng) float() float64 { return float64(r.next()>>11) / (1 << 53) }

// in returns a number in [lo, hi).
func (r *rng) in(lo, hi float64) float64 { return lo + (hi-lo)*r.float() }

// seed returns a fresh seed for one noise field.
func (r *rng) seed() uint32 { return uint32(r.next() >> 32) }

func hash2(x, y int32, seed uint32) uint32 {
	h := uint32(x)*0x8da6b343 ^ uint32(y)*0xd8163841 ^ seed*0xcb1ab31f
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return h
}

var gradX, gradY [256]float64

func init() {
	for i := range gradX {
		a := 2 * math.Pi * (float64(i) + 0.5) / 256
		gradX[i], gradY[i] = math.Cos(a), math.Sin(a)
	}
}

// gnoise is gradient noise in about -1..1.
func gnoise(x, y float64, seed uint32) float64 {
	xf, yf := math.Floor(x), math.Floor(y)
	fx, fy := x-xf, y-yf
	ix, iy := int32(xf), int32(yf)
	u := fx * fx * fx * (fx*(fx*6-15) + 10)
	v := fy * fy * fy * (fy*(fy*6-15) + 10)
	g := func(cx, cy int32, dx, dy float64) float64 {
		h := hash2(cx, cy, seed) >> 8 & 255
		return gradX[h]*dx + gradY[h]*dy
	}
	a := g(ix, iy, fx, fy)
	b := g(ix+1, iy, fx-1, fy)
	c := g(ix, iy+1, fx, fy-1)
	d := g(ix+1, iy+1, fx-1, fy-1)
	return 1.5 * (a + u*(b-a) + v*(c-a) + u*v*(a-b-c+d))
}

// gnoiseD is gnoise with its slope in x and y.
func gnoiseD(x, y float64, seed uint32) (n, dx, dy float64) {
	xf, yf := math.Floor(x), math.Floor(y)
	fx, fy := x-xf, y-yf
	ix, iy := int32(xf), int32(yf)
	u := fx * fx * fx * (fx*(fx*6-15) + 10)
	v := fy * fy * fy * (fy*(fy*6-15) + 10)
	du := 30 * fx * fx * (fx - 1) * (fx - 1)
	dv := 30 * fy * fy * (fy - 1) * (fy - 1)
	ha, hb := hash2(ix, iy, seed)>>8&255, hash2(ix+1, iy, seed)>>8&255
	hc, hd := hash2(ix, iy+1, seed)>>8&255, hash2(ix+1, iy+1, seed)>>8&255
	a := gradX[ha]*fx + gradY[ha]*fy
	b := gradX[hb]*(fx-1) + gradY[hb]*fy
	c := gradX[hc]*fx + gradY[hc]*(fy-1)
	d := gradX[hd]*(fx-1) + gradY[hd]*(fy-1)
	k := a - b - c + d
	n = a + u*(b-a) + v*(c-a) + u*v*k
	dx = gradX[ha] + u*(gradX[hb]-gradX[ha]) + v*(gradX[hc]-gradX[ha]) + u*v*(gradX[ha]-gradX[hb]-gradX[hc]+gradX[hd]) + du*(b-a+v*k)
	dy = gradY[ha] + u*(gradY[hb]-gradY[ha]) + v*(gradY[hc]-gradY[ha]) + u*v*(gradY[ha]-gradY[hb]-gradY[hc]+gradY[hd]) + dv*(c-a+u*k)
	return 1.5 * n, 1.5 * dx, 1.5 * dy
}

// ridge is two octaves of gradient noise with their slope. Where it crosses
// zero it draws long wandering lines; the slope says how far away the line is.
func ridge(x, y float64, seed uint32) (n, dx, dy float64) {
	n, dx, dy = gnoiseD(x, y, seed)
	x2, y2 := 1.62*x+1.18*y+13.7, -1.18*x+1.62*y+4.1
	m, mx, my := gnoiseD(x2, y2, seed+7919)
	return n + 0.5*m, dx + 0.5*(1.62*mx-1.18*my), dy + 0.5*(1.18*mx+1.62*my)
}

// fbm sums octaves of gradient noise, each turned a little so the lattice never lines up. About -1..1.
func fbm(x, y float64, octaves int, seed uint32) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for i := 0; i < octaves; i++ {
		sum += amp * gnoise(x, y, seed+uint32(i)*7919)
		norm += amp
		x, y = 1.62*x+1.18*y+13.7, -1.18*x+1.62*y+4.1
		amp *= 0.5
	}
	return sum / norm
}

// cells is Worley noise over one jittered point per unit square. It returns
// how far (x, y) is from the border between its two nearest points, and a hash
// naming each of the two.
func cells(x, y float64, seed uint32) (border float64, id1, id2 uint32) {
	xf, yf := math.Floor(x), math.Floor(y)
	ix, iy := int32(xf), int32(yf)
	fx, fy := x-xf, y-yf
	d1, d2 := 9.0, 9.0
	var x1, y1, x2, y2 float64
	for j := int32(-1); j <= 1; j++ {
		for i := int32(-1); i <= 1; i++ {
			h := hash2(ix+i, iy+j, seed)
			dx := float64(i) + float64(h&0xffff)/65536 - fx
			dy := float64(j) + float64(h>>16)/65536 - fy
			d := dx*dx + dy*dy
			if d < d1 {
				d2, id2, x2, y2 = d1, id1, x1, y1
				d1, id1, x1, y1 = d, h, dx, dy
			} else if d < d2 {
				d2, id2, x2, y2 = d, h, dx, dy
			}
		}
	}
	// The border is the line midway between the two points. Measuring to it, rather than comparing the
	// two distances, keeps a border thin even where two points happen to sit close together.
	return (d2 - d1) / (2*math.Hypot(x2-x1, y2-y1) + 1e-9), id1, id2
}

// pair gives two numbers in 0..1 that belong to the border between two cells, whichever side it is seen from.
func pair(id1, id2 uint32) (float64, float64) {
	h := (id1 ^ id2) * 0x9e3779b1
	h ^= h >> 15
	h *= 0x85ebca6b
	h ^= h >> 13
	return float64(h&0xffff) / 65535, float64(h>>16) / 65535
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func smooth(a, b, x float64) float64 {
	t := clamp((x-a)/(b-a), 0, 1)
	return t * t * (3 - 2*t)
}

func mix(a, b, t float64) float64 { return a + (b-a)*t }

// soft takes a tone to the share of the lighter or deeper colour mixed in. It begins gently, gathers a
// little through the middle so a patch of colour has some shape, and eases off before it arrives.
func soft(t float64) float64 { return 0.5*(1-math.Exp(-1.1*t)) + 0.5*smooth(0, 1.1, t) }
