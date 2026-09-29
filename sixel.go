package main

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/sixel"
	"golang.org/x/term"
)

// Sixel gives us real pixels instead of two colors per cell, so the shading
// can be averaged properly rather than resolved to one material per subpixel.
// The encoder has no notion of transparency — every pixel is written — so the
// ground outside the furnace is painted rather than left to the terminal.
var ground = rgb{0.05, 0.05, 0.06}

const sixelSamples = 2 // NxN samples per pixel, averaged

// termInfo is what the terminal will tell us about itself.
type termInfo struct {
	sixel bool
	cell  image.Point // size of a character cell, in pixels
}

// probeTerminal asks the terminal for its cell size (CSI 16 t) and its device
// attributes (CSI c), whose reply lists 4 when it can draw sixels.
//
// The conversation happens on /dev/tty rather than on stdin, for two reasons:
// Go will not poll stdin, so a read deadline there fails outright and a
// terminal that ignores the queries would hang us forever; and stdin belongs
// to Bubble Tea, which does its own startup handshake we must not eat.
func probeTerminal(timeout time.Duration) termInfo {
	info := termInfo{cell: image.Pt(10, 20)} // a common cell, if it won't say

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return info // no controlling terminal, or not a unix
	}
	defer tty.Close() //nolint:errcheck

	fd := int(tty.Fd())
	if !term.IsTerminal(fd) {
		return info
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return info
	}
	defer term.Restore(fd, state) //nolint:errcheck

	if err := tty.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return info
	}
	if _, err := tty.WriteString("\x1b[16t\x1b[c"); err != nil {
		return info
	}

	var reply []byte
	buf := make([]byte, 64)
	for !bytes.ContainsRune(reply, 'c') { // the device attributes reply ends in c
		n, err := tty.Read(buf)
		if reply = append(reply, buf[:n]...); err != nil {
			break // including the deadline passing
		}
	}
	return parseProbe(string(reply), info)
}

// parseProbe reads the two replies out of whatever the terminal sent back.
func parseProbe(reply string, info termInfo) termInfo {
	if _, rest, ok := strings.Cut(reply, "\x1b[6;"); ok { // CSI 6 ; height ; width t
		if size, _, ok := strings.Cut(rest, "t"); ok {
			if h, w, ok := strings.Cut(size, ";"); ok {
				height, errH := strconv.Atoi(h)
				width, errW := strconv.Atoi(w)
				if errH == nil && errW == nil && width > 0 && height > 0 {
					info.cell = image.Pt(width, height)
				}
			}
		}
	}
	if _, rest, ok := strings.Cut(reply, "\x1b[?"); ok { // CSI ? attrs c
		if attrs, _, ok := strings.Cut(rest, "c"); ok {
			for _, a := range strings.Split(attrs, ";") {
				info.sixel = info.sixel || a == "4"
			}
		}
	}
	return info
}

// sixelImage renders one frame at full pixel resolution, antialiased by
// averaging: with real pixels there is no two-colors-per-cell limit to dodge.
func sixelImage(t float64, w, h int) *image.RGBA {
	f := newFrame(t)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	half := math.Min(float64(w), float64(h)) / 2

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum rgb
			for sy := 0; sy < sixelSamples; sy++ {
				for sx := 0; sx < sixelSamples; sx++ {
					wx := (float64(x) + (float64(sx)+0.5)/sixelSamples - float64(w)/2) / half
					wy := (float64(y) + (float64(sy)+0.5)/sixelSamples - float64(h)/2) / half
					c, m := sample(wx, wy, f)
					if m == matNone {
						c = ground
					}
					sum = sum.add(c)
				}
			}
			// SetRGBA, not Set: Set boxes a color per pixel, which at this
			// resolution is hundreds of thousands of allocations a frame.
			img.SetRGBA(x, y, sum.scale(1/float64(sixelSamples*sixelSamples)).rgba())
		}
	}
	return img
}

// sixelBlock returns one frame as lines that occupy cellsH rows. The image is
// drawn from the last of them, stepping the cursor back up to the top of the
// block and returning it afterwards, so Bubble Tea's own line bookkeeping
// still holds: the payload measures zero cells wide, and the blank lines above
// are painted before the image lands on them rather than after.
func sixelBlock(t float64, cell image.Point) string {
	img := sixelImage(t, cellsW*cell.X, cellsH*cell.Y)

	var payload bytes.Buffer
	if err := (&sixel.Encoder{}).Encode(&payload, img); err != nil {
		return strings.Repeat("\n", cellsH-1) // fail quietly: draw nothing
	}
	return strings.Repeat("\n", cellsH-1) +
		ansi.SaveCursor + ansi.CursorUp(cellsH-1) +
		ansi.SixelGraphics(1, 1, 0, payload.Bytes()) + ansi.RestoreCursor
}

func (c rgb) rgba() color.RGBA {
	v := func(f float64) uint8 { return uint8(math.Round(255 * math.Max(0, math.Min(1, f)))) }
	return color.RGBA{R: v(c.r), G: v(c.g), B: v(c.b), A: 255}
}
