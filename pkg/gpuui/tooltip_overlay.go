package gpuui

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/overlay"
	"github.com/gogpu/ui/widget"
)

// Tooltips are passive: unlike menus, dismissing one must not consume input.
type passiveHelpOverlay struct{ *overlay.Container }

func (o *passiveHelpOverlay) Event(ctx widget.Context, e event.Event) bool {
	switch ev := e.(type) {
	case *event.MouseEvent:
		if ev.MouseType == event.MousePress {
			o.Dismiss()
		}
	case *event.KeyEvent:
		if ev.KeyType == event.KeyPress {
			o.Dismiss()
		}
	case *event.WheelEvent:
		o.Dismiss()
	}
	return false
}

type helpOverlayManager struct {
	stack *overlay.Stack
	ctx   widget.Context
}

func (m *helpOverlayManager) PushOverlay(w widget.Widget, dismiss func()) {
	// Match windowOverlayManager: retained rendering only composites overlay
	// content registered as a repaint boundary. Event transparency is separate.
	if b, ok := w.(interface{ SetRepaintBoundary(bool) }); ok {
		b.SetRepaintBoundary(true)
	}
	o := &passiveHelpOverlay{overlay.NewContainer(w, m.ctx.WindowSize(), overlay.WithOnDismiss(dismiss))}
	m.stack.Push(o)
	widget.MountTree(o, m.ctx)
}
func (m *helpOverlayManager) PopOverlay() {
	if o := m.stack.Pop(); o != nil {
		widget.UnmountTree(o)
	}
}
func (m *helpOverlayManager) RemoveOverlay(w widget.Widget) {
	for _, o := range m.stack.List() {
		if p, ok := o.(*passiveHelpOverlay); ok && p.Content() == w {
			m.stack.Remove(o)
			widget.UnmountTree(o)
			return
		}
	}
}
