package gpuui

import (
	"github.com/gogpu/ui/geometry"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"strings"
)

// cellText constrains text to its column. The current toolkit's TextWidget
// exposes Ellipsis but its Draw method does not apply it yet.
type cellText struct {
	*p.TextWidget
	size   float32
	color  widget.Color
	bold   bool
	path   bool
	center bool
}

func (t *cellText) Bold() *cellText { t.bold = true; t.TextWidget.Bold(); return t }
func (t *cellText) Draw(_ widget.Context, c widget.Canvas) {
	bounds := t.Bounds()
	if !t.IsVisible() || bounds.IsEmpty() {
		return
	}
	text := t.Content()
	width := bounds.Width()
	if t.path {
		text = compactPath(text, width, func(s string) float32 { return c.MeasureText(s, t.size, t.bold) })
	} else if c.MeasureText(text, t.size, t.bold) > width {
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
	if t.center {
		bounds.Min.Y += (bounds.Height() - t.size*1.4) / 2
		bounds.Max.Y = bounds.Min.Y + t.size*1.4
	}
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

// wrappedText draws actual lines: TextWidget currently measures wrapping but
// sends the entire paragraph to the single-line Canvas.DrawText method.
type wrappedText struct {
	*p.TextWidget
	size  float32
	color widget.Color
}

func (b *browser) helpText(text string) *wrappedText {
	return &wrappedText{TextWidget: p.Text(text), size: 12, color: b.theme.Colors.OnSurfaceSecond}
}
func (t *wrappedText) Layout(_ widget.Context, constraints geometry.Constraints) geometry.Size {
	width := constraints.MaxWidth
	lines := wrapWords(t.Content(), width, func(s string) float32 { return float32(len([]rune(s))) * t.size * .75 })
	size := constraints.Constrain(geometry.Sz(width, float32(len(lines))*t.size*1.5))
	t.SetBounds(geometry.FromPointSize(t.Position(), size))
	return size
}
func (t *wrappedText) Draw(_ widget.Context, c widget.Canvas) {
	bounds := t.Bounds()
	if !t.IsVisible() || bounds.IsEmpty() {
		return
	}
	lines := wrapWords(t.Content(), bounds.Width(), func(s string) float32 { return c.MeasureText(s, t.size, false) })
	c.PushClip(bounds)
	defer c.PopClip()
	for i, line := range lines {
		r := geometry.NewRect(bounds.Min.X, bounds.Min.Y+float32(i)*t.size*1.5, bounds.Width(), t.size*1.5)
		c.DrawText(line, r, t.size, t.color, false, widget.TextAlignLeft)
	}
}
func wrapWords(text string, width float32, measure func(string) float32) []string {
	if text == "" || width <= 0 {
		return nil
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if measure(candidate) <= width {
			line = candidate
			continue
		}
		if line != "" {
			lines = append(lines, line)
			line = ""
		}
		// Split unusually long words without losing Unicode code points.
		for _, r := range word {
			candidate = line + string(r)
			if line != "" && measure(candidate) > width {
				lines = append(lines, line)
				line = ""
			}
			line += string(r)
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
