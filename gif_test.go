package main

import (
	"flag"
	"image"
	"image/color"
	"image/gif"
	"os"
	"strconv"
	"strings"
	"testing"
)

var gifOut = flag.String("gif", "", "write the README animation to this path")

// TestReadmeGif redraws the animation for the README: go test -run Gif -gif demo.gif
//
// It goes through Render, the same path the terminal gets, and paints each
// sextant cell the way a terminal would, so the GIF shows the real output
// rather than an idealised picture of it.
func TestReadmeGif(t *testing.T) {
	if *gifOut == "" {
		t.Skip("pass -gif <path> to write it")
	}
	const cw, ch = 12, 24 // cell size in pixels; each sextant is 6x8
	const pad = 2         // cells of terminal around the furnace
	term := color.RGBA{0x16, 0x16, 0x1a, 0xff}

	mask := map[rune]int{}
	for m := 0; m < 64; m++ {
		mask[sextants.glyph(m)] = m
	}

	anim := &gif.GIF{}
	most := 0
	for step := 0; step < turnSteps; step++ {
		// Each frame gets its own palette: the whole turn uses more than 256
		// colors, but no single frame does, so nothing is dithered or rounded.
		pal := map[color.RGBA]uint8{term: 0}
		img := image.NewPaletted(image.Rect(0, 0, (cellsW+2*pad)*cw, (cellsH+2*pad)*ch), color.Palette{term})
		index := func(c color.RGBA) uint8 {
			i, ok := pal[c]
			if !ok {
				if len(img.Palette) == 256 {
					t.Fatalf("frame %d has more than 256 colors", step)
				}
				i = uint8(len(img.Palette))
				pal[c], img.Palette = i, append(img.Palette, c)
			}
			return i
		}
		fg, bg := term, term
		cx, cy := pad, pad
		s := Render(float64(step)*6.283185307179586/turnSteps, sextants)
		for i := 0; i < len(s); {
			switch {
			case s[i] == '\x1b':
				end := i + strings.IndexByte(s[i:], 'm')
				fg, bg = sgr(s[i+2:end], fg, bg, term)
				i = end + 1
			case s[i] == '\n':
				cx, cy = pad, cy+1
				i++
			default:
				r, n := []rune(s[i:])[0], len(string([]rune(s[i:])[0]))
				m, ok := mask[r]
				if !ok {
					t.Fatalf("unexpected rune %q", r)
				}
				fi, bi := index(fg), index(bg)
				for py := 0; py < ch; py++ {
					for px := 0; px < cw; px++ {
						bit := (py*3/ch)*2 + px*2/cw
						c := bi
						if m>>bit&1 == 1 {
							c = fi
						}
						img.Pix[img.PixOffset(cx*cw+px, cy*ch+py)] = c
					}
				}
				cx++
				i += n
			}
		}
		anim.Image = append(anim.Image, img)
		anim.Delay = append(anim.Delay, 100/fps)
		most = max(most, len(img.Palette))
	}

	f, err := os.Create(*gifOut)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	if err := gif.EncodeAll(f, anim); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d frames, at most %d colors in one", len(anim.Image), most)
}

// sgr applies one SGR sequence's truecolor and reset parameters.
func sgr(params string, fg, bg, reset color.RGBA) (color.RGBA, color.RGBA) {
	p := strings.Split(params, ";")
	for i := 0; i < len(p); i++ {
		switch {
		case p[i] == "0" || p[i] == "":
			fg, bg = reset, reset
		case (p[i] == "38" || p[i] == "48") && i+4 < len(p) && p[i+1] == "2":
			v := func(s string) uint8 { n, _ := strconv.Atoi(s); return uint8(n) }
			c := color.RGBA{v(p[i+2]), v(p[i+3]), v(p[i+4]), 0xff}
			if p[i] == "38" {
				fg = c
			} else {
				bg = c
			}
			i += 4
		}
	}
	return fg, bg
}
