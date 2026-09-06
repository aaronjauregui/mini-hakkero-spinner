package main

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const supersample = 3 // NxN samples per subpixel, resolved by majority vote

// Size of the drawing, in text cells. A cell is twice as tall as it is wide,
// so half as many rows as columns makes the canvas square on screen.
var cellsW, cellsH = 36, 18

// setSize resizes the drawing to the given width in cells, keeping it square.
func setSize(width int) {
	cellsW, cellsH = width, (width+1)/2
}

// A glyphSet carves each text cell into a w x h grid of subpixels and names the
// rune that lights an arbitrary subset of them. Bit i of a mask is the subpixel
// at (i%w, i/w); lit subpixels take the foreground color, the rest the
// background. Two colors per cell is the hard limit a terminal gives us.
type glyphSet struct {
	name  string
	w, h  int
	glyph func(mask int) rune
}

var halfBlocks = glyphSet{"half-blocks", 1, 2, func(m int) rune {
	return [...]rune{' ', '▀', '▄', '█'}[m]
}}

// Sextants triple the vertical detail of half blocks and double the
// horizontal. VTE draws them itself, so they render even in fonts that lack
// the Symbols for Legacy Computing block.
var sextants = glyphSet{"sextants", 2, 3, func(m int) rune {
	// U+1FB00.. holds every 2x3 pattern except the four Unicode already had.
	switch m {
	case 0:
		return ' '
	case 21:
		return '▌'
	case 42:
		return '▐'
	case 63:
		return '█'
	}
	i := m - 1
	if m > 21 {
		i--
	}
	if m > 42 {
		i--
	}
	return rune(0x1FB00 + i)
}}

// shade resolves one subpixel to a single material rather than averaging
// colors across boundaries: edges stay hard, and supersampling only decides
// which material wins and shades within it.
func shade(x0, y0, w, h float64, f frame) (rgb, int) {
	var sums [matYang + 1]rgb
	var hits [matYang + 1]int
	for sy := 0; sy < supersample; sy++ {
		for sx := 0; sx < supersample; sx++ {
			c, m := sample(
				x0+w*(float64(sx)+0.5)/supersample,
				y0+h*(float64(sy)+0.5)/supersample, f)
			sums[m], hits[m] = sums[m].add(c), hits[m]+1
		}
	}
	best := matNone
	for m := matWood; m <= matYang; m++ {
		if hits[m] > 0 && hits[m] >= hits[best] {
			best = m // ties go to the finer detail
		}
	}
	return sums[best].scale(1 / float64(hits[best])), best
}

// Render draws one frame, spun by t radians.
var cellCache = map[cellKey]string{}

type cellKey struct {
	mask   int
	fg, bg lipgloss.Color
}

func Render(t float64, g glyphSet) string {
	f := newFrame(t)
	// Subpixel size in cell widths, and the half-extent that both axes span:
	// cells are twice as tall as wide, so the canvas is square on screen.
	sw, sh := 1/float64(g.w), 2/float64(g.h)
	half := math.Min(float64(cellsW)/2, float64(cellsH)) // fit the shorter axis

	var b strings.Builder
	cols, rows := make([]rgb, g.w*g.h), make([]int, g.w*g.h)
	for cy := 0; cy < cellsH; cy++ {
		for cx := 0; cx < cellsW; cx++ {
			for sy := 0; sy < g.h; sy++ {
				for sx := 0; sx < g.w; sx++ {
					x := (float64(cx*g.w+sx)*sw - half) / half
					y := (float64(cy*g.h+sy)*sh - float64(cellsH)) / half
					i := sy*g.w + sx
					cols[i], rows[i] = shade(x, y, sw/half, sh/half, f)
				}
			}
			b.WriteString(cell(cols, rows, g))
		}
		if cy < cellsH-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// cell picks the two colors and the glyph that best describe one cell's
// subpixels. Materials are the clustering key: the two most common win, and
// any others join whichever of the two they look more like.
func cell(cols []rgb, mats []int, g glyphSet) string {
	var count [matYang + 1]int
	for _, m := range mats {
		count[m]++
	}
	if count[matNone] == len(mats) {
		return " "
	}

	// Empty subpixels must land in the background, which we leave unset so the
	// terminal shows through instead of a dark box around the silhouette.
	fg, bg, mask := pick(count, matNone), matNone, 0
	if count[matNone] == 0 {
		bg = pick(count, fg)
	}
	fgMean, bgMean := mean(cols, mats, fg), mean(cols, mats, bg)
	for i, m := range mats {
		var lit bool
		switch {
		case m == matNone: // stays in the (unset) background
		case m == fg || bg == matNone:
			lit = true
		case m == bg:
		default: // a third material joins whichever color it resembles
			lit = fgMean.dist(cols[i]) <= bgMean.dist(cols[i])
		}
		if lit {
			mask |= 1 << i
		}
	}

	key := cellKey{mask: mask, fg: blend(cols, mats, mask, true).color()}
	if bg != matNone {
		key.bg = blend(cols, mats, mask, false).color()
	}
	if s, ok := cellCache[key]; ok {
		return s
	}
	st := lipgloss.NewStyle().Foreground(key.fg)
	if key.bg != "" {
		st = st.Background(key.bg)
	}
	s := st.Render(string(g.glyph(mask)))
	cellCache[key] = s
	return s
}

// pick returns the most common material, skipping one already taken.
func pick(count [matYang + 1]int, taken int) int {
	best := matNone
	for m := matWood; m <= matYang; m++ {
		if m != taken && count[m] > count[best] {
			best = m
		}
	}
	return best
}

func mean(cols []rgb, mats []int, m int) rgb {
	var sum rgb
	var n float64
	for i, mat := range mats {
		if mat == m {
			sum, n = sum.add(cols[i]), n+1
		}
	}
	if n == 0 {
		return sum
	}
	return sum.scale(1 / n)
}

// blend averages the subpixels the mask assigned to one of the two colors.
func blend(cols []rgb, mats []int, mask int, lit bool) rgb {
	var sum rgb
	var n float64
	for i := range cols {
		if mats[i] != matNone && (mask>>i&1 == 1) == lit {
			sum, n = sum.add(cols[i]), n+1
		}
	}
	if n == 0 {
		return sum
	}
	return sum.scale(1 / n)
}

type rgb struct{ r, g, b float64 }

func (c rgb) scale(k float64) rgb { return rgb{c.r * k, c.g * k, c.b * k} }
func (c rgb) add(d rgb) rgb       { return rgb{c.r + d.r, c.g + d.g, c.b + d.b} }
func (c rgb) dist(d rgb) float64  { return math.Abs(c.r-d.r) + math.Abs(c.g-d.g) + math.Abs(c.b-d.b) }
func (c rgb) mix(d rgb, k float64) rgb {
	return rgb{c.r + (d.r-c.r)*k, c.g + (d.g-c.g)*k, c.b + (d.b-c.b)*k}
}

func (c rgb) color() lipgloss.Color {
	const digits = "0123456789abcdef"
	var s [7]byte
	s[0] = '#'
	for i, v := range [3]float64{c.r, c.g, c.b} {
		n := int(math.Round(255 * math.Max(0, math.Min(1, v))))
		s[1+2*i], s[2+2*i] = digits[n>>4], digits[n&15]
	}
	return lipgloss.Color(string(s[:]))
}
