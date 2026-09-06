package main

import "math"

// Proportions of the furnace, as fractions of its inradius.
const (
	shellR  = 0.95 // octagonal shell
	rimW    = 0.06 // brass band around the rim
	innerR  = 0.40 // inner octagon enclosing the outlet
	ringW   = 0.04
	outletR = 0.30 // master spark outlet
	bezelW  = 0.04
)

var (
	wood    = rgb{0.42, 0.24, 0.13}
	brass   = rgb{0.85, 0.65, 0.26}
	engrave = rgb{0.96, 0.88, 0.66}
	spark   = rgb{1.00, 0.97, 0.88}
	ember   = rgb{1.00, 0.62, 0.16}
	vantage = rgb{0.09, 0.06, 0.05}
)

// Materials, in priority order: a pixel takes the highest-numbered material
// among its samples, so thin engraving never dissolves into the wood.
const (
	matNone = iota
	matWood
	matRim
	matRing
	matEngrave
	matYin
	matYang
)

// The eight trigrams in Fu Xi (Earlier Heaven) order, clockwise from the top
// face, as three bits each: bit 0 is the bottom line, 1 is solid yang and 0 is
// broken yin. This is the arrangement in which every trigram faces its own
// complement — opposite faces always sum to 7, so ☰ 111 sits across from
// ☷ 000, ☴ 011 across from ☳ 100, and so on.
var bagua = [8]int{7, 3, 2, 1, 0, 4, 5, 6}

// trigram returns the trigram engraved on a face, numbered from the +x face.
func trigram(face int) int { return bagua[((face+2)%8+8)%8] } // face 6 points up

// The outward normals of the eight faces, precomputed: the shell's distance
// function is evaluated a few hundred thousand times per frame.
var faceNormal = func() (n [8][2]float64) {
	for k := range n {
		s, c := math.Sincos(float64(k) * math.Pi / 4)
		n[k] = [2]float64{c, s}
	}
	return
}()

// octagon returns the signed distance to a regular octagon of the given
// inradius: negative inside, and the value is the depth below the nearest face.
func octagon(x, y, r float64) float64 {
	d := math.Inf(-1)
	for _, n := range faceNormal {
		d = math.Max(d, x*n[0]+y*n[1])
	}
	return d - r
}

// A frame hoists everything that depends only on the rotation out of the
// per-sample path: the whole animation costs three trig calls per frame.
type frame struct {
	sin, cos       float64 // of -t, to spin samples backwards
	lightC, lightS float64 // of the light angle, for the per-face sheen
	pulse          float64
}

func newFrame(t float64) frame {
	sin, cos := math.Sincos(-t)
	lc, ls := math.Sincos(t + 3*math.Pi/4)
	return frame{sin, cos, ls, lc, 0.85 + 0.15*math.Sin(t*7)}
}

// sample shades one point of the unit square [-1,1]^2, and reports which
// material it hit (matNone where the furnace isn't).
func sample(sx, sy float64, f frame) (rgb, int) {
	x, y := sx*f.cos-sy*f.sin, sx*f.sin+sy*f.cos

	// One pass over the face normals yields both the shell's distance function
	// and the nearest face, which the rim, ring, chamber and engraving share.
	d, face := math.Inf(-1), 0
	for k, n := range faceNormal {
		if v := x*n[0] + y*n[1]; v > d {
			d, face = v, k
		}
	}
	if d > shellR {
		return rgb{}, matNone
	}
	// One flat tone per face, lit from the upper left in screen space: the
	// facets stay crisp and the highlight sweeps around as the shell turns.
	// cos(faceAngle + lightAngle), expanded over the face normal.
	n := faceNormal[face]
	sheen := 0.58 + 0.42*(0.5+0.5*(n[0]*f.lightC-n[1]*f.lightS))

	col, mat := wood.scale(sheen), matWood
	switch rad := math.Hypot(x, y); {
	case rad < outletR-bezelW:
		return taiji(x, y, outletR-bezelW, f)
	case rad < outletR: // brass bezel around the outlet
		return brass.scale(sheen * 1.1), matRing
	case d > shellR-rimW:
		col, mat = brass.scale(sheen*0.9), matRim
	case onTrigram(x, y, face):
		col, mat = engrave.scale(sheen), matEngrave
	case math.Abs(d-innerR) < ringW:
		col, mat = brass.scale(sheen), matRing
	case d < innerR:
		col = wood.scale(sheen * 0.5) // recessed chamber
	}
	return col, mat
}

// taiji shades the yin-yang in the outlet. Its dividing S is two half circles
// of radius r/2, stacked: each one bulges across the diameter into the other
// half, and each lobe carries an eye of the opposite color.
func taiji(x, y, r float64, f frame) (rgb, int) {
	head, tail := math.Hypot(x, y-r/2), math.Hypot(x, y+r/2)

	yang := tail >= r/2 && x < 0 || head < r/2
	if head < r/5 || tail < r/5 { // the eyes invert their lobe
		yang = tail < r/5
	}
	if yang {
		return spark.mix(ember, math.Hypot(x, y)/r*0.5).scale(f.pulse), matYang
	}
	return vantage, matYin
}

// onTrigram reports whether the point falls on one of the engraved bars. The
// trigram is read from the center outwards, as it is on a bagua: its bottom
// line is the bar nearest the outlet.
func onTrigram(x, y float64, face int) bool {
	c, s := faceNormal[face][0], faceNormal[face][1]
	v, u := x*c+y*s, -x*s+y*c // v runs outward along the face normal

	bits := trigram(face)
	for line := 0; line < 3; line++ {
		if math.Abs(v-(0.545+0.145*float64(line))) > 0.042 || math.Abs(u) > 0.225 {
			continue
		}
		if bits&(1<<line) != 0 {
			return true // yang: one unbroken bar
		}
		return math.Abs(u) > 0.065 // yin: two bars with a gap
	}
	return false
}
