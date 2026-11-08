package gpuui

import (
	"github.com/gogpu/ui/core/popover"
	"github.com/gogpu/ui/core/toolbar"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/overlay"
	"github.com/gogpu/ui/theme/fluent"
	"github.com/gogpu/ui/widget"
)

// Toolbar labels are only accessibility metadata upstream. Capture the actual
// painted button bounds so help also works for disabled items, without duplicating
// the toolbar's layout or enabling actions that cannot run.
type paneToolbar struct {
	*toolbar.Widget
	theme    *fluent.Theme
	targets  []toolbar.PaintButtonState
	tip      *toolbarHelp
	manager  widget.OverlayManager
	overlays *overlay.Stack
}
type helpPainter struct {
	paneToolbarPainter
	owner *paneToolbar
}

func (p helpPainter) PaintButtonItem(c widget.Canvas, s toolbar.PaintButtonState) {
	p.owner.targets = append(p.owner.targets, s)
	p.paneToolbarPainter.PaintButtonItem(c, s)
}
func newPaneToolbar(theme *fluent.Theme, items ...toolbar.Item) *paneToolbar {
	w := &paneToolbar{theme: theme}
	w.Widget = toolbar.New(toolbar.Height(32), toolbar.ButtonSize(28), toolbar.Gap(3), toolbar.PainterOpt(helpPainter{paneToolbarPainter{theme: theme}, w}), toolbar.Items(items...))
	return w
}
func (w *paneToolbar) Draw(ctx widget.Context, c widget.Canvas) {
	w.targets = w.targets[:0]
	w.Widget.Draw(ctx, c)
}
func (w *paneToolbar) hideHelp() {
	if w.tip != nil && w.manager != nil {
		w.manager.RemoveOverlay(w.tip)
	}
	w.tip = nil
	w.manager = nil
}
func (w *paneToolbar) Mount(widget.Context) {}
func (w *paneToolbar) Unmount()             { w.hideHelp() }
func (w *paneToolbar) Event(ctx widget.Context, e event.Event) bool {
	if m, ok := e.(*event.MouseEvent); ok {
		switch m.MouseType {
		case event.MouseMove, event.MouseEnter, event.MouseDrag:
			w.showHelp(ctx, m.Position)
		case event.MouseLeave, event.MousePress:
			w.hideHelp()
		}
	} else {
		w.hideHelp()
	}
	return w.Widget.Event(ctx, e)
}
func (w *paneToolbar) showHelp(ctx widget.Context, point geometry.Point) {
	for _, target := range w.targets {
		if !target.Bounds.Contains(point.Sub(w.Bounds().Min)) {
			continue
		}
		text := target.Label
		if target.Disabled {
			text += " · Unavailable"
		}
		if w.tip != nil && w.tip.text == text {
			return
		}
		w.hideHelp()
		manager := ctx.OverlayManager()
		if w.overlays != nil {
			manager = &helpOverlayManager{stack: w.overlays, ctx: ctx}
		}
		if manager == nil {
			return
		}
		size := geometry.Sz(float32(len([]rune(text)))*7+20, 28)
		anchor := geometry.FromPointSize(w.ScreenBounds().Min.Add(target.Bounds.Min), target.Bounds.Size())
		pos := popover.CalculatePosition(popover.BottomStart, anchor, size, ctx.WindowSize(), 5)
		tip := &toolbarHelp{text: text, theme: w.theme}
		tip.SetVisible(true)
		tip.SetEnabled(true)
		tip.SetBounds(geometry.FromPointSize(pos, size))
		w.tip, w.manager = tip, manager
		manager.PushOverlay(tip, func() { w.hideHelp() })
		return
	}
	w.hideHelp()
}

type toolbarHelp struct {
	widget.WidgetBase
	text  string
	theme *fluent.Theme
}

func (t *toolbarHelp) Layout(_ widget.Context, c geometry.Constraints) geometry.Size {
	return c.Constrain(t.Bounds().Size())
}
func (t *toolbarHelp) Draw(_ widget.Context, c widget.Canvas) {
	cs := t.theme.Colors
	b := t.Bounds()
	c.DrawRoundRect(b, cs.Surface, 4)
	c.StrokeRoundRect(b, cs.StrokeDefault, 4, 1)
	c.DrawText(t.text, geometry.NewRect(b.Min.X+10, b.Min.Y+6, b.Width()-20, 16), 12, cs.OnSurface, false, widget.TextAlignLeft)
}
func (t *toolbarHelp) Event(widget.Context, event.Event) bool { return false }
func (t *toolbarHelp) Children() []widget.Widget              { return nil }
