package leather

import "math"

// rgb is a colour in linear light.
type rgb [3]float64

func hex(s string) rgb {
	var v [3]float64
	for i := range v {
		var n int
		for _, c := range s[1+2*i : 3+2*i] {
			n *= 16
			switch {
			case c >= 'a':
				n += int(c-'a') + 10
			case c >= 'A':
				n += int(c-'A') + 10
			default:
				n += int(c - '0')
			}
		}
		c := float64(n) / 255
		if c <= 0.04045 {
			v[i] = c / 12.92
		} else {
			v[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return v
}

func encode(c float64) float64 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

func (a rgb) mix(b rgb, t float64) rgb {
	return rgb{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// lch is OKLCH: lightness 0..1, chroma, hue in degrees. Tones of one dye are
// worked out here, where "lighter" and "warmer" mean what the eye means.
type lch struct{ l, c, h float64 }

func (a rgb) lch() lch {
	l := math.Cbrt(0.4122214708*a[0] + 0.5363325363*a[1] + 0.0514459929*a[2])
	m := math.Cbrt(0.2119034982*a[0] + 0.6806995451*a[1] + 0.1073969566*a[2])
	s := math.Cbrt(0.0883024619*a[0] + 0.2817188376*a[1] + 0.6299787005*a[2])
	L := 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	A := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	B := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	h := math.Atan2(B, A) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return lch{L, math.Hypot(A, B), h}
}

func (c lch) rgb() rgb {
	a, b := c.c*math.Cos(c.h*math.Pi/180), c.c*math.Sin(c.h*math.Pi/180)
	l := c.l + 0.3963377774*a + 0.2158037573*b
	m := c.l - 0.1055613458*a - 0.0638541728*b
	s := c.l - 0.0894841775*a - 1.2914855480*b
	l, m, s = l*l*l, m*m*m, s*s*s
	out := rgb{
		4.0767416621*l - 3.3077115913*m + 0.2309699292*s,
		-1.2684380046*l + 2.6097574011*m - 0.3413193965*s,
		-0.0041960863*l - 0.7034186147*m + 1.7076147010*s,
	}
	for i := range out {
		out[i] = clamp(out[i], 0, 1)
	}
	return out
}

// blend mixes two colours the way the eye would, part of the way from a to b.
func (a lch) blend(b lch, t float64) lch {
	ax, ay := a.c*math.Cos(a.h*math.Pi/180), a.c*math.Sin(a.h*math.Pi/180)
	bx, by := b.c*math.Cos(b.h*math.Pi/180), b.c*math.Sin(b.h*math.Pi/180)
	x, y := ax+(bx-ax)*t, ay+(by-ay)*t
	h := math.Atan2(y, x) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return lch{a.l + (b.l-a.l)*t, math.Hypot(x, y), h}
}

// toward turns a hue by up to max degrees toward a target hue, the short way round.
func toward(h, target, max float64) float64 {
	d := math.Mod(target-h+540, 360) - 180
	return h + clamp(d, -max, max)
}

// Dyes are the hides each cover colour can be cut from. A seed picks one and
// shifts it a little, so two green books are rarely the same green.
var Dyes = map[string][]string{
	"bay":      {"#24362b", "#2b4431", "#363f26", "#1d3632", "#3f472b", "#263b34"},
	"tomato":   {"#5a2620", "#552421", "#6e3023", "#65321e", "#5e1f1b", "#722b27"},
	"butter":   {"#ad8440", "#ac8b45", "#a2802c", "#b48e52", "#ba9248", "#a3834c"},
	"ocean":    {"#1b3140", "#1d404b", "#304456", "#19283c", "#244555", "#293a50"},
	"plum":     {"#392536", "#462233", "#4f2d45", "#3e2841", "#54283a", "#37283c"},
	"charcoal": {"#262729", "#2b2826", "#303337", "#282b2a", "#363432", "#222428"},
	"paper":    {"#c9bb9c", "#c7a87e", "#b6af9e", "#d1c6ac", "#c0ac8a", "#bbb29a"},
	"chestnut": {"#84481f", "#683719", "#462918", "#76512d", "#7f4a22", "#59371f", "#7b431d", "#664123"},
}
