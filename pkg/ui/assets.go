package ui

import (
	"bytes"
	_ "embed"
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

const DymBridgeAppIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <rect width="512" height="512" fill="#000000"/>
  <g transform="rotate(-45 256 256)">
    <rect x="116" y="116" width="280" height="280" rx="46" fill="#5F82AD"/>
    <rect x="208" y="104" width="22" height="304" rx="11" fill="#000000"/>
    <rect x="282" y="104" width="22" height="304" rx="11" fill="#000000"/>
    <line x1="256" y1="130" x2="256" y2="175" stroke="#000000" stroke-width="6" stroke-linecap="round"/>
    <line x1="256" y1="205" x2="256" y2="250" stroke="#000000" stroke-width="6" stroke-linecap="round"/>
    <line x1="256" y1="280" x2="256" y2="325" stroke="#000000" stroke-width="6" stroke-linecap="round"/>
    <line x1="256" y1="355" x2="256" y2="390" stroke="#000000" stroke-width="6" stroke-linecap="round"/>
    <rect x="200" y="180" width="38" height="14" rx="4" fill="#5F82AD"/>
    <rect x="200" y="320" width="38" height="14" rx="4" fill="#5F82AD"/>
    <rect x="274" y="180" width="38" height="14" rx="4" fill="#5F82AD"/>
    <rect x="274" y="320" width="38" height="14" rx="4" fill="#5F82AD"/>
  </g>
</svg>`

const DymBridgeTrayIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="32" height="32">
  <g transform="rotate(-45 16 16)">
    <rect x="7" y="7" width="18" height="18" rx="3" fill="#000"/>
    <rect x="13" y="6" width="2" height="20" rx="1" fill="#FFF"/>
    <rect x="17" y="6" width="2" height="20" rx="1" fill="#FFF"/>
    <rect x="12.5" y="11" width="3" height="1" fill="#000"/>
    <rect x="12.5" y="20" width="3" height="1" fill="#000"/>
    <rect x="16.5" y="11" width="3" height="1" fill="#000"/>
    <rect x="16.5" y="20" width="3" height="1" fill="#000"/>
  </g>
</svg>`

//go:embed app_icon.png
var AppIconPNGData []byte

//go:embed tray_icon.png
var TrayIconPNGData []byte

// GetAppIconResource returns the application icon resource.
func GetAppIconResource() fyne.Resource {
	return fyne.NewStaticResource("app_icon.png", AppIconPNGData)
}

// GetTrayIconResource returns a resource for the macOS/Windows system tray.
func GetTrayIconResource() fyne.Resource {
	return fyne.NewStaticResource("tray_icon.png", TrayIconPNGData)
}

// PCIcon returns a themed vector SVG icon of a personal computer / desktop monitor.
func PCIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("pc_monitor.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#000" d="M20 3H4c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h6v2H8v2h8v-2h-2v-2h6c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm0 12H4V5h16v10z"/></svg>`)))
}

// ServerCloudIcon returns a themed vector SVG icon of a cloud server.
func ServerCloudIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("cloud_server.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#000" d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM19 18H6c-2.21 0-4-1.79-4-4 0-2.05 1.53-3.76 3.56-3.97l1.07-.11.5-.95C8.08 7.14 9.94 6 12 6c2.62 0 4.88 1.86 5.39 4.43l.3 1.5 1.53.11c1.56.1 2.78 1.41 2.78 2.96 0 1.65-1.35 3-3 3z"/></svg>`)))
}

// ServerIcon returns a themed vector SVG icon of a physical server / rack.
func ServerIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("server_rack.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#000" d="M20 13H4c-1.1 0-2 .9-2 2v4c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2v-4c0-1.1-.9-2-2-2zm-1 6H5v-4h14v4zm-12-1a1 1 0 1 0 0-2 1 1 0 0 0 0 2zm13-16H4c-1.1 0-2 .9-2 2v4c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-1 6H5V5h14v4zm-12-1a1 1 0 1 0 0-2 1 1 0 0 0 0 2z"/></svg>`)))
}
