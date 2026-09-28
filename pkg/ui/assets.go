package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// CreateBridgeIcon generates a clean, wireframe bridge/gateway icon as a PNG.
func CreateBridgeIcon(dim int) []byte {
	if dim <= 0 {
		dim = 64
	}
	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.Transparent}, image.Point{}, draw.Src)

	scale := float64(dim) / 64.0
	mainColor := color.RGBA{R: 13, G: 110, B: 253, A: 255}
	stroke := 3.5 * scale

	// Draw horizontal roadway / bridge deck
	drawThickLine(img, 6.0*scale, 44.0*scale, 58.0*scale, 44.0*scale, stroke, mainColor)

	// Draw left and right bridge towers
	towerStroke := 3.0 * scale
	// Left tower
	drawThickLine(img, 18.0*scale, 14.0*scale, 18.0*scale, 44.0*scale, towerStroke, mainColor)
	// Right tower
	drawThickLine(img, 46.0*scale, 14.0*scale, 46.0*scale, 44.0*scale, towerStroke, mainColor)

	// Suspension cables (parabolic arcs approximated with line segments)
	cableStroke := 2.2 * scale
	pts := []struct{ x, y float64 }{
		{6.0, 36.0},
		{18.0, 15.0},
		{32.0, 34.0},
		{46.0, 15.0},
		{58.0, 36.0},
	}
	for i := 0; i < len(pts)-1; i++ {
		drawThickLine(img, pts[i].x*scale, pts[i].y*scale, pts[i+1].x*scale, pts[i+1].y*scale, cableStroke, mainColor)
	}

	// Vertical suspender ropes
	vertStroke := 1.8 * scale
	// Middle suspenders
	drawThickLine(img, 25.0*scale, 28.0*scale, 25.0*scale, 44.0*scale, vertStroke, mainColor)
	drawThickLine(img, 32.0*scale, 34.0*scale, 32.0*scale, 44.0*scale, vertStroke, mainColor)
	drawThickLine(img, 39.0*scale, 28.0*scale, 39.0*scale, 44.0*scale, vertStroke, mainColor)

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// CreateTrayTemplateIcon generates a template icon (monochrome black for macOS/Windows status bar).
func CreateTrayTemplateIcon(dim int) []byte {
	if dim <= 0 {
		dim = 32
	}
	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.Transparent}, image.Point{}, draw.Src)

	scale := float64(dim) / 32.0
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	stroke := 2.0 * scale

	// Roadway
	drawThickLine(img, 3.0*scale, 22.0*scale, 29.0*scale, 22.0*scale, stroke, black)
	// Towers
	drawThickLine(img, 9.0*scale, 7.0*scale, 9.0*scale, 22.0*scale, stroke, black)
	drawThickLine(img, 23.0*scale, 7.0*scale, 23.0*scale, 22.0*scale, stroke, black)
	// Cable
	drawThickLine(img, 3.0*scale, 18.0*scale, 9.0*scale, 8.0*scale, 1.6*scale, black)
	drawThickLine(img, 9.0*scale, 8.0*scale, 16.0*scale, 17.0*scale, 1.6*scale, black)
	drawThickLine(img, 16.0*scale, 17.0*scale, 23.0*scale, 8.0*scale, 1.6*scale, black)
	drawThickLine(img, 23.0*scale, 8.0*scale, 29.0*scale, 18.0*scale, 1.6*scale, black)

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func drawThickLine(img *image.RGBA, x0, y0, x1, y1 float64, width float64, c color.RGBA) {
	minX := int(math.Floor(math.Min(x0, x1) - width - 1))
	maxX := int(math.Ceil(math.Max(x0, x1) + width + 1))
	minY := int(math.Floor(math.Min(y0, y1) - width - 1))
	maxY := int(math.Ceil(math.Max(y0, y1) + width + 1))

	b := img.Bounds()
	if minX < b.Min.X {
		minX = b.Min.X
	}
	if minY < b.Min.Y {
		minY = b.Min.Y
	}
	if maxX > b.Max.X {
		maxX = b.Max.X
	}
	if maxY > b.Max.Y {
		maxY = b.Max.Y
	}

	dx := x1 - x0
	dy := y1 - y0
	lenSq := dx*dx + dy*dy
	halfW := width / 2.0

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5

			var t float64
			if lenSq > 0 {
				t = ((px-x0)*dx + (py-y0)*dy) / lenSq
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
			}

			projX := x0 + t*dx
			projY := y0 + t*dy
			dist := math.Sqrt((px-projX)*(px-projX) + (py-projY)*(py-projY))

			if dist <= halfW {
				img.SetRGBA(x, y, c)
			} else if dist < halfW+1.0 {
				alphaRatio := 1.0 - (dist - halfW)
				if alphaRatio > 0 {
					targetAlpha := uint8(float64(c.A) * alphaRatio)
					orig := img.RGBAAt(x, y)
					if targetAlpha > orig.A {
						img.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: targetAlpha})
					}
				}
			}
		}
	}
}

// GetAppIconResource returns the application icon resource.
func GetAppIconResource() fyne.Resource {
	return fyne.NewStaticResource("app_icon.png", CreateBridgeIcon(128))
}

// GetTrayIconResource returns a themed resource for the macOS/Windows system tray.
func GetTrayIconResource() fyne.Resource {
	rawPNG := CreateTrayTemplateIcon(32)
	baseRes := fyne.NewStaticResource("tray_icon.png", rawPNG)
	return theme.NewThemedResource(baseRes)
}
