// Command genicons renders the PWA icons (an envelope on a gradient tile).
// Run from the repo root: go run ./tools/genicons
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type rgb struct{ r, g, b float64 }

func lerp(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

var (
	top    = rgb{99, 102, 241} // indigo
	bottom = rgb{139, 92, 246} // violet
	white  = rgb{255, 255, 255}
)

func roundedRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	return math.Hypot(x-cx, y-cy) <= r
}

// sample returns colour and coverage at unit coordinates (0..1).
// scale shrinks the artwork towards the centre (for maskable safe zones).
func sample(x, y, scale float64, rounded bool) (rgb, float64) {
	bg := lerp(top, bottom, (x+y)/2)
	if rounded && !roundedRect(x, y, 0, 0, 1, 1, 0.22) {
		return bg, 0
	}
	// Map into artwork space.
	ax, ay := (x-0.5)/scale+0.5, (y-0.5)/scale+0.5
	if roundedRect(ax, ay, 0.22, 0.30, 0.78, 0.70, 0.045) {
		// Flap: a V from the top corners down to the middle.
		d := math.Min(lineDist(ax, ay, 0.22, 0.30, 0.5, 0.54), lineDist(ax, ay, 0.78, 0.30, 0.5, 0.54))
		if d < 0.016/1 {
			return lerp(top, bottom, 0.5), 1
		}
		return white, 1
	}
	return bg, 1
}

func lineDist(px, py, x0, y0, x1, y1 float64) float64 {
	dx, dy := x1-x0, y1-y0
	t := math.Max(0, math.Min(1, ((px-x0)*dx+(py-y0)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(px-(x0+t*dx), py-(y0+t*dy))
}

func render(size int, scale float64, rounded bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					c, cov := sample(x, y, scale, rounded)
					r, g, b, a = r+c.r*cov, g+c.g*cov, b+c.b*cov, a+cov
				}
			}
			if a > 0 {
				img.SetNRGBA(px, py, color.NRGBA{uint8(r / a), uint8(g / a), uint8(b / a), uint8(255 * a / (ss * ss))})
			}
		}
	}
	return img
}

func save(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func main() {
	dir := filepath.Join("web", "static", "icons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	save(filepath.Join(dir, "icon-192.png"), render(192, 1, true))
	save(filepath.Join(dir, "icon-512.png"), render(512, 1, true))
	save(filepath.Join(dir, "maskable-512.png"), render(512, 0.8, false))
	save(filepath.Join(dir, "apple-touch-icon.png"), render(180, 0.9, false))
}
