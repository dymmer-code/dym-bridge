package gpuui

import (
	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/core/menu"
	"github.com/gogpu/ui/core/toolbar"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/icon"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/theme/fluent"
	"github.com/gogpu/ui/widget"
)

var backIcon = icon.IconData{Name: "back", ViewBox: 24, StrokeWidth: 1.8, Ops: []icon.PathOp{icon.Move(8, 4), icon.Line(3, 9), icon.Line(8, 14), icon.Move(3, 9), icon.Line(14, 9), icon.Cubic(23, 9, 23, 20, 14, 20), icon.Line(10, 20)}}
var moonIcon = icon.FromSVG("moon", 24, "M20 15.5 A9 9 0 0 1 8.5 4 A8 8 0 1 0 20 15.5 Z")
var sunIcon = icon.IconData{Name: "sun", ViewBox: 24, StrokeWidth: 1.8, Ops: []icon.PathOp{icon.Move(17, 12), icon.Cubic(17, 18.7, 7, 18.7, 7, 12), icon.Cubic(7, 5.3, 17, 5.3, 17, 12), icon.ClosePath(), icon.Move(12, 1), icon.Line(12, 4), icon.Move(12, 20), icon.Line(12, 23), icon.Move(1, 12), icon.Line(4, 12), icon.Move(20, 12), icon.Line(23, 12), icon.Move(4, 4), icon.Line(6, 6), icon.Move(18, 18), icon.Line(20, 20), icon.Move(4, 20), icon.Line(6, 18), icon.Move(18, 6), icon.Line(20, 4)}}

func (b *browser) themeButton() widget.Widget {
	label, glyph := "Switch to dark theme", moonIcon
	if b.dark {
		label, glyph = "Switch to light theme", sunIcon
	}
	bar := newPaneToolbar(b.theme, toolbar.IconButton(label, glyph, func() { b.dark = !b.dark; b.setTheme(); b.ui.SetTheme(b.theme.AsTheme()); b.rebuild() }))
	bar.overlays = b.ui.Window().Overlays()
	return p.Box(bar).Width(47)
}

type iconActionPainter struct {
	fluent.ButtonPainter
	glyph icon.IconData
}

func (p iconActionPainter) ButtonPadding(size button.Size) (float32, float32) {
	x, y := p.ButtonPainter.ButtonPadding(size)
	return x + 14, y
}
func (p iconActionPainter) PaintButton(c widget.Canvas, s button.PaintState) {
	label := s.Text
	s.Text = ""
	p.ButtonPainter.PaintButton(c, s)
	color := p.Theme.Colors.OnSurface
	if s.Disabled {
		color = p.Theme.Colors.OnSurfaceSecond
	}
	y := s.Bounds.Min.Y + (s.Bounds.Height()-18)/2
	icon.Draw(c, p.glyph, geometry.NewRect(s.Bounds.Min.X+10, y, 18, 18), color)
	r := s.Bounds
	r.Min.X += 36
	r.Max.X -= 10
	c.PushClip(r)
	c.DrawText(label, r, 12, color, false, widget.TextAlignLeft)
	c.PopClip()
}
func (b *browser) iconAction(label string, glyph icon.IconData, fn func()) *button.Widget {
	return b.action(label, false, fn, button.PainterOpt(iconActionPainter{fluent.ButtonPainter{Theme: b.theme}, glyph}))
}

// The library's default menu painter ignores ColorScheme; use our Fluent palette.
func paintSitesBar(c widget.Canvas, s *menu.MenuBarPaintState) {
	cs := s.ColorScheme
	c.DrawRect(s.Bounds, cs.BarBackground)
	for i, m := range s.Menus {
		if i >= len(s.MenuRects) {
			break
		}
		r := s.MenuRects[i]
		fg := cs.BarText
		if i == s.OpenIndex || i == s.HoveredIndex {
			c.DrawRoundRect(r, cs.BarHover, 4)
		}
		if i == s.OpenIndex {
			fg = cs.BarActiveText
		}
		c.DrawText(m.Label, r, 13, fg, false, widget.TextAlignCenter)
	}
}
func paintSitesPopup(c widget.Canvas, s *menu.MenuPaintState) {
	cs := s.ColorScheme
	c.DrawRoundRect(s.Bounds, cs.MenuBackground, 5)
	c.StrokeRoundRect(s.Bounds, cs.MenuBorder, 5, 1)
	c.PushClip(s.Bounds)
	defer c.PopClip()
	y := s.Bounds.Min.Y + 4
	for i, item := range s.Items {
		if item.IsSeparator() {
			c.DrawLine(geometry.Pt(s.Bounds.Min.X+8, y+s.SeparatorHeight/2), geometry.Pt(s.Bounds.Max.X-8, y+s.SeparatorHeight/2), cs.SeparatorColor, 1)
			y += s.SeparatorHeight
			continue
		}
		r := geometry.NewRect(s.Bounds.Min.X, y, s.Bounds.Width(), s.ItemHeight)
		if i == s.HighlightedIndex && !item.Disabled {
			c.DrawRect(r, cs.ItemHover)
		}
		fg := cs.ItemText
		if item.Disabled {
			fg = cs.ItemDisabledText
		}
		r.Min.X += 12
		r.Max.X -= 12
		c.DrawText(item.Label, r, 13, fg, false, widget.TextAlignLeft)
		y += s.ItemHeight
	}
}
