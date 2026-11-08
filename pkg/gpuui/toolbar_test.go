package gpuui

import (
	"github.com/gogpu/ui/core/toolbar"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/overlay"
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
	w := newPaneToolbar(fluent.NewTheme(), toolbar.IconButton("Refresh", upIcon, nil))
	w.SetBounds(geometry.NewRect(100, 50, 300, 32))
	w.targets = []toolbar.PaintButtonState{{Label: "Delete", Disabled: true, Bounds: geometry.NewRect(0, 0, 28, 28)}}
	move := func(x float32) {
		w.Event(ctx, event.NewMouseEvent(event.MouseMove, event.ButtonNone, 0, geometry.Pt(x, 60), geometry.Pt(x, 60), 0))
	}
	move(110)
	if stack.current == nil || w.tip.text != "Delete · Not available in this preview" {
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

// Exercise the real overlay event path, not just a mock that stores tooltips.
func TestTooltipDoesNotConsumeToolbarClicks(t *testing.T) {
	ctx := widget.NewContext()
	ctx.SetWindowSize(geometry.Sz(800, 600))
	stack := overlay.NewStack(nil)
	ctx.SetOverlayManager(&helpOverlayManager{stack: stack, ctx: ctx})
	model := NewDemoModel()
	w := newPaneToolbar(fluent.NewTheme(), toolbar.IconButton("Select/deselect all", upIcon, model.ToggleAll))
	w.Layout(ctx, geometry.Constraints{MinWidth: 300, MaxWidth: 300, MinHeight: 32, MaxHeight: 32})
	w.targets = []toolbar.PaintButtonState{{Label: "Select/deselect all", Bounds: geometry.NewRect(0, 0, 28, 28)}}
	send := func(kind event.MouseEventType) {
		e := event.NewMouseEvent(kind, event.ButtonLeft, event.ButtonStateLeft, geometry.Pt(14, 16), geometry.Pt(14, 16), 0)
		if !stack.HandleEvent(ctx, e) {
			w.Event(ctx, e)
		}
	}
	for i := 0; i < 4; i++ {
		send(event.MouseMove)
		if stack.IsEmpty() {
			t.Fatal("expected tooltip before click")
		}
		send(event.MousePress)
		send(event.MouseRelease)
		want := 0
		if i%2 == 0 {
			want = len(model.Visible)
		}
		if model.SelectionCount() != want {
			t.Fatalf("click %d: selected %d, want %d", i+1, model.SelectionCount(), want)
		}
	}
}
