package gpuui

import (
	"github.com/gogpu/ui/core/toolbar"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/theme/fluent"
	"github.com/gogpu/ui/widget"
	"testing"
)

type tipStack struct{ current widget.Widget }

func (s *tipStack) PushOverlay(w widget.Widget, _ func()) { s.current = w }
func (s *tipStack) PopOverlay()                           { s.current = nil }
func (s *tipStack) RemoveOverlay(w widget.Widget) {
	if s.current == w {
		s.current = nil
	}
}

func TestToolbarHelpIncludesDisabledAndCloses(t *testing.T) {
	ctx := widget.NewContext()
	stack := &tipStack{}
	ctx.SetOverlayManager(stack)
	ctx.SetWindowSize(geometry.Sz(800, 600))
	w := newPaneToolbar(fluent.NewTheme(), toolbar.IconButton("Actualizar", upIcon, nil))
	w.SetBounds(geometry.NewRect(100, 50, 300, 32))
	w.targets = []toolbar.PaintButtonState{{Label: "Eliminar", Disabled: true, Bounds: geometry.NewRect(0, 0, 28, 28)}}
	move := func(x float32) {
		w.Event(ctx, event.NewMouseEvent(event.MouseMove, event.ButtonNone, 0, geometry.Pt(x, 60), geometry.Pt(x, 60), 0))
	}
	move(110)
	if stack.current == nil || w.tip.text != "Eliminar · No disponible en esta vista previa" {
		t.Fatal("missing disabled tooltip")
	}
	first := stack.current
	move(111)
	if stack.current != first {
		t.Fatal("tooltip recreated within same button")
	}
	move(200)
	if stack.current != nil {
		t.Fatal("tooltip remained outside button")
	}
	move(110)
	w.Unmount()
	if stack.current != nil {
		t.Fatal("tooltip survived unmount")
	}
}
