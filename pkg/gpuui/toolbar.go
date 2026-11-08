package gpuui

import (
	"github.com/gogpu/ui/core/toolbar"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/icon"
	"github.com/gogpu/ui/theme/fluent"
	"github.com/gogpu/ui/widget"
)

var newFolderIcon = icon.FromSVG("new-folder", 24, "M2 5 L9 5 L11 8 L22 8 L22 21 L2 21 Z M15 11 L15 18 M11.5 14.5 L18.5 14.5")

var upIcon = icon.FromSVG("up", 24, "M12 4 L5 11 L7 13 L11 9 L11 20 L13 20 L13 9 L17 13 L19 11 Z")

// Transfer arrows share an open box, distinct from parent-folder navigation.
const transferBox = "M3 14 L5 14 L5 20 L19 20 L19 14 L21 14 L21 22 L3 22 Z"

var uploadIcon = icon.FromSVG("upload-box", 24, "M12 2 L6 8 L7.5 9.5 L11 6 L11 16 L13 16 L13 6 L16.5 9.5 L18 8 Z "+transferBox)
var downloadIcon = icon.FromSVG("download-box", 24, "M11 2 L13 2 L13 12 L16.5 8.5 L18 10 L12 16 L6 10 L7.5 8.5 L11 12 Z "+transferBox)

// Use the same Fluent palette as the rest of the window; the toolkit's default
// toolbar painter has hard-coded light colors and no Fluent implementation yet.
type paneToolbarPainter struct{ theme *fluent.Theme }

func (p paneToolbarPainter) PaintToolbar(c widget.Canvas, s toolbar.PaintToolbarState) {
	c.DrawRect(s.Bounds, p.theme.Colors.SurfaceSecondary)
}
func (p paneToolbarPainter) PaintButtonItem(c widget.Canvas, s toolbar.PaintButtonState) {
	cs := p.theme.Colors
	color := cs.OnSurface
	if s.Disabled {
		color = cs.OnSurfaceSecond.WithAlpha(.55)
	} else if s.Pressed || s.Hovered {
		c.DrawRoundRect(s.Bounds, cs.AccentLight, 3)
	}
	icon.Draw(c, s.Icon, s.IconBounds, color)
	if s.ShowLabel {
		c.DrawText(s.Label, s.TextBounds, 12, color, false, widget.TextAlignLeft)
	}
	if s.Focused && !s.Disabled {
		c.StrokeRoundRect(s.Bounds, cs.Accent, 3, 1)
	}
}
func (p paneToolbarPainter) PaintSeparator(c widget.Canvas, b geometry.Rect) {
	x := b.Min.X + b.Width()/2
	c.DrawLine(geometry.Pt(x, b.Min.Y+7), geometry.Pt(x, b.Max.Y-7), p.theme.Colors.StrokeDefault, 1)
}

// Pane identities are visual; English labels remain available to accessibility.
var computerIcon = icon.IconData{Name: "computer", ViewBox: 24, StrokeWidth: 1.6, Ops: []icon.PathOp{
	icon.Move(3, 4), icon.Line(21, 4), icon.Line(21, 17), icon.Line(3, 17), icon.ClosePath(),
	icon.Move(12, 17), icon.Line(12, 21), icon.Move(7, 21), icon.Line(17, 21),
}}
var serverIcon = icon.IconData{Name: "server", ViewBox: 24, StrokeWidth: 1.6, Ops: []icon.PathOp{
	icon.Move(4, 3), icon.Line(20, 3), icon.Line(20, 21), icon.Line(4, 21), icon.ClosePath(),
	icon.Move(4, 9), icon.Line(20, 9), icon.Move(4, 15), icon.Line(20, 15),
	icon.Move(7, 6), icon.Line(9, 6), icon.Move(7, 12), icon.Line(9, 12), icon.Move(7, 18), icon.Line(9, 18),
}}

var connectIcon = icon.IconData{Name: "connect", ViewBox: 24, StrokeWidth: 1.8, Ops: []icon.PathOp{icon.Move(8, 2), icon.Line(8, 7), icon.Move(16, 2), icon.Line(16, 7), icon.Move(5, 7), icon.Line(19, 7), icon.Line(19, 10), icon.Line(15, 15), icon.Line(9, 15), icon.Line(5, 10), icon.ClosePath(), icon.Move(12, 15), icon.Line(12, 22)}}
var disconnectIcon = icon.IconData{Name: "disconnect", ViewBox: 24, StrokeWidth: 1.8, Ops: []icon.PathOp{icon.Move(8, 2), icon.Line(8, 7), icon.Move(16, 2), icon.Line(16, 7), icon.Move(5, 7), icon.Line(19, 7), icon.Line(19, 10), icon.Line(15, 15), icon.Line(9, 15), icon.Line(5, 10), icon.ClosePath(), icon.Move(12, 17), icon.Line(12, 22), icon.Move(3, 3), icon.Line(21, 21)}}
var transferHistoryIcon = icon.IconData{Name: "transfers", ViewBox: 24, StrokeWidth: 1.8, Ops: []icon.PathOp{icon.Move(7, 20), icon.Line(7, 4), icon.Move(3, 8), icon.Line(7, 4), icon.Line(11, 8), icon.Move(17, 4), icon.Line(17, 20), icon.Move(13, 16), icon.Line(17, 20), icon.Line(21, 16)}}
