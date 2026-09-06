package main

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/sixel"
)

func TestParseProbe(t *testing.T) {
	def := termInfo{cell: image.Pt(10, 20)}
	for _, tc := range []struct {
		name  string
		reply string
		sixel bool
		cell  image.Point
	}{
		{"xterm with sixel", "\x1b[6;20;10t\x1b[?63;1;2;4;6;9;15;22c", true, image.Pt(10, 20)},
		{"foot", "\x1b[6;23;11t\x1b[?62;4;22c", true, image.Pt(11, 23)},
		{"gnome-terminal", "\x1b[?65;1;9c", false, image.Pt(10, 20)},
		{"no reply at all", "", false, image.Pt(10, 20)},
		{"attribute 4 only inside another number", "\x1b[?64;14;41c", false, image.Pt(10, 20)},
	} {
		got := parseProbe(tc.reply, def)
		if got.sixel != tc.sixel || got.cell != tc.cell {
			t.Errorf("%s: got sixel=%v cell=%v, want sixel=%v cell=%v",
				tc.name, got.sixel, got.cell, tc.sixel, tc.cell)
		}
	}
}

// The payload has to survive Bubble Tea's renderer, which measures every line
// and truncates it to the terminal width.
func TestSixelBlockFitsTheRenderer(t *testing.T) {
	block := sixelBlock(0.3, image.Pt(10, 20))
	lines := strings.Split(block, "\n")
	if len(lines) != cellsH {
		t.Errorf("block is %d lines, want %d", len(lines), cellsH)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 0 {
			t.Errorf("line %d measures %d cells wide, want 0", i, w)
		}
		if ansi.Truncate(line, 80, "") != line {
			t.Errorf("line %d is mangled by truncation to 80 columns", i)
		}
	}
	t.Logf("%d lines, %d KB per frame", len(lines), len(block)/1024)
}

// Decode the frame back out of the payload. Without a terminal that can draw
// sixels, this is what says the image is really in there — and it decodes with
// a parser written here rather than the one in x/ansi, whose v0.10.1 decoder
// returns a mostly-empty image for payloads ImageMagick reads correctly.
func TestSixelRoundTrip(t *testing.T) {
	const w, h = 120, 120
	src := sixelImage(0.3, w, h)

	var payload bytes.Buffer
	if err := (&sixel.Encoder{}).Encode(&payload, src); err != nil {
		t.Fatal(err)
	}
	got, written := decodeSixel(t, payload.String())

	if got.Bounds().Dx() != w || got.Bounds().Dy() != h {
		t.Fatalf("decoded %v, want %dx%d", got.Bounds(), w, h)
	}
	if written != w*h {
		t.Errorf("%d of %d pixels were written", written, w*h)
	}

	var worst, total float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r1, g1, b1, _ := src.At(x, y).RGBA()
			r2, g2, b2, _ := got.At(x, y).RGBA()
			d := (math.Abs(float64(r1)-float64(r2)) +
				math.Abs(float64(g1)-float64(g2)) +
				math.Abs(float64(b1)-float64(b2))) / 3 / 257
			total += d
			worst = math.Max(worst, d)
		}
	}
	mean := total / float64(w*h)
	t.Logf("mean error %.2f/255, worst %.0f/255 after 256-color quantization", mean, worst)
	if mean > 4 {
		t.Errorf("mean round-trip error %.2f/255 is too high", mean)
	}
}

// decodeSixel reads a sixel payload: a raster header, a palette of "#n;2;r;g;b"
// entries in percent, then bands of six pixel rows, where each data byte
// carries six vertical bits, "!" repeats one, "$" returns to the left margin
// and "-" starts the next band. It returns the image and the number of pixels
// actually written, so a sparse decode can't pass as a match.
func decodeSixel(t *testing.T, p string) (*image.RGBA, int) {
	t.Helper()
	var img *image.RGBA
	pal := map[int]color.RGBA{}
	seen := map[image.Point]bool{}
	x, band, pen := 0, 0, 0

	number := func(i int) (int, int) {
		j := i
		for j < len(p) && p[j] >= '0' && p[j] <= '9' {
			j++
		}
		n, _ := strconv.Atoi(p[i:j])
		return n, j
	}
	put := func(bits byte) {
		for row := 0; row < 6; row++ {
			if pt := (image.Point{X: x, Y: band*6 + row}); bits&(1<<row) != 0 && pt.In(img.Bounds()) {
				img.SetRGBA(pt.X, pt.Y, pal[pen])
				seen[pt] = true
			}
		}
		x++
	}

	for i := 0; i < len(p); {
		switch c := p[i]; {
		case c == '"': // raster attributes: "pan;pad;width;height
			var wd, ht, j int
			_, j = number(i + 1)
			_, j = number(j + 1)
			wd, j = number(j + 1)
			ht, j = number(j + 1)
			img = image.NewRGBA(image.Rect(0, 0, wd, ht))
			i = j
		case c == '#':
			n, j := number(i + 1)
			if j < len(p) && p[j] == ';' { // a palette entry: #n;2;r;g;b
				var r, g, b int
				_, j = number(j + 1) // color space, always 2 for RGB
				r, j = number(j + 1)
				g, j = number(j + 1)
				b, j = number(j + 1)
				pal[n], i = color.RGBA{pct(r), pct(g), pct(b), 255}, j
				continue
			}
			pen, i = n, j // selecting a color
		case c == '!':
			n, j := number(i + 1)
			for k := 0; k < n; k++ {
				put(p[j] - 63)
			}
			i = j + 1
		case c == '$':
			x, i = 0, i+1
		case c == '-':
			x, band, i = 0, band+1, i+1
		case c >= '?' && c <= '~':
			put(c - 63)
			i++
		default:
			i++
		}
	}
	if img == nil {
		t.Fatal("payload carried no raster attribute")
	}
	return img, len(seen)
}

func pct(v int) uint8 { return uint8(math.Round(float64(v) * 255 / 100)) }
