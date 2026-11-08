package gpuui

import (
	"github.com/gogpu/ui/geometry"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
)

// cellText constrains text to its column. The current toolkit's TextWidget
// exposes Ellipsis but its Draw method does not apply it yet.
type cellText struct {
	*p.TextWidget
	size  float32
	color widget.Color
	bold  bool
}

func (t *cellText) Bold() *cellText { t.bold = true; t.TextWidget.Bold(); return t }
func (t *cellText) Draw(_ widget.Context, c widget.Canvas) {
	bounds := t.Bounds()
	if !t.IsVisible() || bounds.IsEmpty() {
		return
	}
	text := t.Content()
	width := bounds.Width()
	if c.MeasureText(text, t.size, t.bold) > width {
		runes := []rune(text)
		lo, hi := 0, len(runes)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if c.MeasureText(string(runes[:mid])+"…", t.size, t.bold) <= width {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		text = string(runes[:lo]) + "…"
	}
	c.PushClip(bounds)
	c.DrawText(text, bounds, t.size, t.color, t.bold, widget.TextAlignLeft)
	c.PopClip()
}

func (t *cellText) Layout(ctx widget.Context, constraints geometry.Constraints) geometry.Size {
	size := t.TextWidget.Layout(ctx, constraints)
	size.Width = float32(len([]rune(t.Content()))) * t.size * 0.72
	size = constraints.Constrain(size)
	t.SetBounds(geometry.FromPointSize(t.Position(), size))
	return size
}
