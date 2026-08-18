// Command genicon renders the viterm application icon as a set of PNGs
// suitable for packing into an .icns file with iconutil. The icon is drawn
// from geometry: a rounded dark tile carrying a prompt chevron and a split
// pane divider with a cursor, in the default theme's palette.
//
// Usage: go run ./scripts/genicon <output-dir>
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const master = 4096 // rendered once at high resolution, then downsampled

// Palette, matching the default theme.
var (
	bgTop    = rgb(0x2a, 0x2f, 0x4a)
	bgBottom = rgb(0x16, 0x16, 0x22)
	accent   = rgb(0x7a, 0xa2, 0xf7)
	dim      = rgb(0x56, 0x5f, 0x89)
	green    = rgb(0x9e, 0xce, 0x6a)
)

func rgb(r, g, b uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: 255} }

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: genicon <output-dir>")
		os.Exit(2)
	}
	out := os.Args[1]
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	img := render(master)

	type spec struct {
		name string
		size int
	}
	specs := []spec{
		{"icon_16x16.png", 16}, {"icon_16x16@2x.png", 32},
		{"icon_32x32.png", 32}, {"icon_32x32@2x.png", 64},
		{"icon_128x128.png", 128}, {"icon_128x128@2x.png", 256},
		{"icon_256x256.png", 256}, {"icon_256x256@2x.png", 512},
		{"icon_512x512.png", 512}, {"icon_512x512@2x.png", 1024},
	}
	for _, s := range specs {
		scaled := downsample(img, s.size)
		if err := writePNG(filepath.Join(out, s.name), scaled); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Println("wrote iconset to", out)
}

// render draws the icon at the given size.
func render(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	f := float64(size)

	// Tile geometry follows the macOS icon grid: the tile occupies the
	// middle ~80% of the canvas with generously rounded corners.
	tile := rect{cx: f / 2, cy: f / 2, hw: f * 0.40, hh: f * 0.40, r: f * 0.18}

	// Prompt chevron in the left pane.
	chW := f * 0.030
	chevA := capsule{x1: f * 0.240, y1: f * 0.375, x2: f * 0.360, y2: f * 0.480, r: chW}
	chevB := capsule{x1: f * 0.360, y1: f * 0.480, x2: f * 0.240, y2: f * 0.585, r: chW}

	// Cursor beside the chevron.
	cursor := rect{cx: f * 0.465, cy: f * 0.575, hw: f * 0.045, hh: f * 0.011, r: f * 0.010}

	// Pane divider and two content strokes in the right pane.
	divider := capsule{x1: f * 0.615, y1: f * 0.330, x2: f * 0.615, y2: f * 0.670, r: f * 0.011}
	lineA := capsule{x1: f * 0.680, y1: f * 0.400, x2: f * 0.770, y2: f * 0.400, r: f * 0.011}
	lineB := capsule{x1: f * 0.680, y1: f * 0.490, x2: f * 0.740, y2: f * 0.490, r: f * 0.011}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5

			a := tile.coverage(px, py)
			if a <= 0 {
				continue
			}
			// Vertical gradient across the tile.
			t := (py - (tile.cy - tile.hh)) / (2 * tile.hh)
			c := lerpColor(bgTop, bgBottom, clamp01(t))

			c = over(c, accent, math.Max(chevA.coverage(px, py), chevB.coverage(px, py)))
			c = over(c, green, cursor.coverage(px, py))
			c = over(c, dim, divider.coverage(px, py))
			c = over(c, dim, math.Max(lineA.coverage(px, py), lineB.coverage(px, py)))

			c.A = uint8(a * 255)
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// rect is a rounded rectangle centered at (cx, cy).
type rect struct{ cx, cy, hw, hh, r float64 }

func (s rect) distance(px, py float64) float64 {
	dx := math.Abs(px-s.cx) - (s.hw - s.r)
	dy := math.Abs(py-s.cy) - (s.hh - s.r)
	ax, ay := math.Max(dx, 0), math.Max(dy, 0)
	return math.Hypot(ax, ay) + math.Min(math.Max(dx, dy), 0) - s.r
}

func (s rect) coverage(px, py float64) float64 { return smooth(s.distance(px, py)) }

// capsule is a thick line segment with rounded ends.
type capsule struct{ x1, y1, x2, y2, r float64 }

func (s capsule) coverage(px, py float64) float64 {
	vx, vy := s.x2-s.x1, s.y2-s.y1
	wx, wy := px-s.x1, py-s.y1
	t := clamp01((wx*vx + wy*vy) / (vx*vx + vy*vy))
	dx, dy := wx-t*vx, wy-t*vy
	return smooth(math.Hypot(dx, dy) - s.r)
}

// smooth converts a signed distance into edge coverage over roughly two
// pixels of the master rendering, which downsamples into clean antialiasing.
func smooth(d float64) float64 { return clamp01(0.5 - d/2) }

func clamp01(v float64) float64 { return math.Min(1, math.Max(0, v)) }

func lerpColor(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: 255,
	}
}

// over blends src onto dst with the given coverage.
func over(dst, src color.NRGBA, a float64) color.NRGBA {
	if a <= 0 {
		return dst
	}
	return color.NRGBA{
		R: uint8(float64(dst.R) + (float64(src.R)-float64(dst.R))*a),
		G: uint8(float64(dst.G) + (float64(src.G)-float64(dst.G))*a),
		B: uint8(float64(dst.B) + (float64(src.B)-float64(dst.B))*a),
		A: 255,
	}
}

// downsample area-averages the master image to the target size. The master
// size is a multiple of every target, so a plain block average is exact.
func downsample(src *image.NRGBA, target int) *image.NRGBA {
	block := src.Bounds().Dx() / target
	out := image.NewNRGBA(image.Rect(0, 0, target, target))
	n := float64(block * block)
	for y := 0; y < target; y++ {
		for x := 0; x < target; x++ {
			var r, g, b, a float64
			for by := 0; by < block; by++ {
				for bx := 0; bx < block; bx++ {
					c := src.NRGBAAt(x*block+bx, y*block+by)
					w := float64(c.A) / 255
					r += float64(c.R) * w
					g += float64(c.G) * w
					b += float64(c.B) * w
					a += float64(c.A)
				}
			}
			avgA := a / n
			var c color.NRGBA
			if avgA > 0 {
				c = color.NRGBA{
					R: uint8(r / (a / 255)),
					G: uint8(g / (a / 255)),
					B: uint8(b / (a / 255)),
					A: uint8(avgA),
				}
			}
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
