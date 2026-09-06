package main

import (
	"math"
	"strings"
	"testing"
)

// The Fu Xi arrangement's defining property: every face carries a distinct
// trigram, and each one sits opposite its own complement.
func TestBaguaArrangement(t *testing.T) {
	seen := map[int]bool{}
	for face := 0; face < 8; face++ {
		v, opposite := trigram(face), trigram(face+4)
		if v+opposite != 7 {
			t.Errorf("face %d: %03b faces %03b, not its complement", face, v, opposite)
		}
		seen[v] = true
	}
	if len(seen) != 8 {
		t.Errorf("got %d distinct trigrams, want 8", len(seen))
	}
	if trigram(6) != 0b111 { // face 6 points up
		t.Errorf("Qian is not on the top face: got %03b", trigram(6))
	}
}

// Read the engraving back out of the rendered geometry by scanning across each
// bar, so the check is on what sample draws rather than on the bit table.
func TestEngravingReadsBack(t *testing.T) {
	for face := 0; face < 8; face++ {
		s, c := math.Sincos(float64(face) * math.Pi / 4)
		drawn := 0
		for line := 0; line < 3; line++ { // line 0 is the bottom, drawn innermost
			v := 0.545 + 0.145*float64(line)
			var bar strings.Builder
			for i := -12; i <= 12; i++ {
				u := float64(i) / 12 * 0.2
				_, m := sample(v*c-u*s, v*s+u*c, newFrame(0))
				bar.WriteByte(map[bool]byte{true: '#', false: '.'}[m == matEngrave])
			}
			r := bar.String()
			if !strings.HasPrefix(r, "#") || !strings.HasSuffix(r, "#") {
				t.Errorf("face %d line %d does not span the face: %s", face, line+1, r)
			}
			if !strings.Contains(r, ".") { // no gap: an unbroken yang line
				drawn |= 1 << line
			}
		}
		if drawn != trigram(face) {
			t.Errorf("face %d draws %03b, want %03b", face, drawn, trigram(face))
		}
	}
}
