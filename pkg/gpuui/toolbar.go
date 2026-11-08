package gpuui

import (
	"github.com/gogpu/ui/core/toolbar"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/icon"
	"github.com/gogpu/ui/theme/fluent"
	"github.com/gogpu/ui/widget"
)

var upIcon = icon.FromSVG("up", 24, "M12 4 L5 11 L7 13 L11 9 L11 20 L13 20 L13 9 L17 13 L19 11 Z")
var downIcon = icon.FromSVG("down", 24, "M12 20 L5 13 L7 11 L11 15 L11 4 L13 4 L13 15 L17 11 L19 13 Z")

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
