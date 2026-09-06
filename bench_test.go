package main

import (
	"image"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() { lipgloss.SetColorProfile(termenv.TrueColor) }

func benchRender(b *testing.B, w int, g glyphSet) {
	setSize(w)
	defer setSize(36)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Render(float64(i)*0.01, g)
	}
}

func BenchmarkSextants36(b *testing.B) { benchRender(b, 36, sextants) }
func BenchmarkHalf36(b *testing.B)     { benchRender(b, 36, halfBlocks) }
func BenchmarkSextants52(b *testing.B) { benchRender(b, 52, sextants) }
func BenchmarkSextants72(b *testing.B) { benchRender(b, 72, sextants) }

// Shading only: the same sample workload with no glyph or style building.
func BenchmarkShadeOnly(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f := newFrame(float64(i) * 0.01)
		for cy := 0; cy < cellsH*3; cy++ {
			for cx := 0; cx < cellsW*2; cx++ {
				x := (float64(cx)*0.5 - 18) / 18
				y := (float64(cy)*(2.0/3) - 18) / 18
				shade(x, y, 0.5/18, (2.0/3)/18, f)
			}
		}
	}
}

// Style building only: one cell's worth of lipgloss work, 648 times.
func BenchmarkStyleOnly(b *testing.B) {
	b.ReportAllocs()
	fg, bg := rgb{0.8, 0.6, 0.3}, rgb{0.2, 0.1, 0.05}
	for i := 0; i < b.N; i++ {
		for c := 0; c < cellsW*cellsH; c++ {
			_ = lipgloss.NewStyle().Foreground(fg.color()).Background(bg.color()).Render("▀")
		}
	}
}

func BenchmarkSixel36(b *testing.B) {
	b.ReportAllocs()
	cell := image.Pt(10, 20)
	for i := 0; i < b.N; i++ {
		_ = sixelBlock(float64(i)*0.01, cell)
	}
}
