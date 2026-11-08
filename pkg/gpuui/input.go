package gpuui

import (
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/core/listview"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/widget"
	"time"
)

// Keep all rich input capabilities while adapting the unit mismatch in UI v0.1.55.
type richInput interface {
	gpucontext.EventSource
	gpucontext.PointerEventSource
	gpucontext.ScrollEventSource
	gpucontext.GestureEventSource
}
type normalizedInput struct{ richInput }

func adaptInput(source gpucontext.EventSource) gpucontext.EventSource {
	if rich, ok := source.(richInput); ok {
		return &normalizedInput{rich}
	}
	return source
}
func (s *normalizedInput) OnScrollEvent(fn func(gpucontext.ScrollEvent)) {
	s.richInput.OnScrollEvent(func(e gpucontext.ScrollEvent) { fn(normalizeScroll(e)) })
}

// ScrollView applies 40 pixels per input unit, even when the OS sends pixels.
// Convert once before that metadata is discarded by app.attachScrollBridge.
func normalizeScroll(e gpucontext.ScrollEvent) gpucontext.ScrollEvent {
	factor := 1.0
	switch e.DeltaMode {
	case gpucontext.ScrollDeltaLine:
		factor = 28
	case gpucontext.ScrollDeltaPage:
		factor = 560
	}
	e.DeltaX *= factor / 40
	e.DeltaY *= factor / 40
	e.DeltaMode = gpucontext.ScrollDeltaLine
	return e
}

type clickTracker struct {
	key  string
	at   time.Time
	x, y float32
}

func (c *clickTracker) click(key string, at time.Time, x, y float32) bool {
	elapsed := at.Sub(c.at)
	dx, dy := x-c.x, y-c.y
	activate := key == c.key && !c.at.IsZero() && elapsed >= 0 && elapsed <= 500*time.Millisecond && dx*dx+dy*dy <= 16
	*c = clickTracker{key: key, at: at, x: x, y: y}
	if activate {
		*c = clickTracker{}
	}
	return activate
}

// Defer navigation until ListView has finished selecting and focusing the old
// row, so rebuilding the tree cannot leave focus pointing at an unmounted list.
type fileList struct {
	*listview.Widget
	model      *Model
	clicks     clickTracker
	clicked    int
	activate   func(string)
	fileAction func(sftp.Item)
	changed    func()
}

func (l *fileList) Event(ctx widget.Context, e event.Event) bool {
	l.clicked = -1
	consumed := l.Widget.Event(ctx, e)
	if mouse, ok := e.(*event.MouseEvent); ok && mouse.MouseType == event.MousePress && mouse.Button == event.ButtonLeft && l.clicked >= 0 {
		ctx.RequestFocus(l)
	}
	if wheel, ok := e.(*event.WheelEvent); ok && (wheel.Delta.X != 0 || wheel.Delta.Y != 0) {
		l.clicks = clickTracker{}
	}
	if l.clicked < 0 || l.clicked >= len(l.model.Visible) {
		return consumed
	}
	item := l.model.Visible[l.clicked]
	activate := false
	switch ev := e.(type) {
	case *event.MouseEvent:
		if ev.MouseType == event.MousePress && ev.Button == event.ButtonLeft {
			x := ev.Position.X - l.Bounds().Min.X
			if x >= 6 && x < 34 {
				l.toggleChecked(l.clicked)
				return true
			}
			activate = l.clicks.click(item.Path, ev.Time(), ev.Position.X, ev.Position.Y)
		}
	case *event.KeyEvent:
		if ev.KeyType == event.KeyPress && ev.Key == event.KeySpace {
			l.toggleChecked(l.clicked)
			return true
		}
		activate = ev.KeyType == event.KeyPress && ev.Key == event.KeyEnter
	}
	if activate {
		if item.IsDir {
			l.activate(item.Path)
		} else if l.fileAction != nil {
			l.fileAction(item)
		}
	}
	return consumed
}

func (l *fileList) toggleChecked(index int) {
	l.model.Toggle(index)
	l.clicks = clickTracker{}
	l.InvalidateData()
	if l.changed != nil {
		l.changed()
	}
}
