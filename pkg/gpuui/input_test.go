package gpuui

import (
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/core/listview"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
	"math"
	"testing"
	"time"
)

func TestScrollUnits(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mode        gpucontext.ScrollDeltaMode
		delta, want float64
	}{{"pixel", gpucontext.ScrollDeltaPixel, 12, 12}, {"fraction", gpucontext.ScrollDeltaPixel, .5, .5}, {"wheel", gpucontext.ScrollDeltaLine, 1, 28}, {"reverse", gpucontext.ScrollDeltaLine, -3, -84}, {"page", gpucontext.ScrollDeltaPage, 1, 560}} {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeScroll(gpucontext.ScrollEvent{DeltaY: tc.delta, DeltaMode: tc.mode})
			if math.Abs(got.DeltaY*40-tc.want) > 1e-9 {
				t.Fatalf("rendered delta %g want %g", got.DeltaY*40, tc.want)
			}
		})
	}
}
func TestDoubleClick(t *testing.T) {
	now := time.Unix(100, 0)
	var c clickTracker
	if c.click("folder-a", now, 0, 0) {
		t.Fatal("single click activated")
	}
	if !c.click("folder-a", now.Add(200*time.Millisecond), 2, 1) {
		t.Fatal("double click missed")
	}
	if c.click("folder-a", now.Add(300*time.Millisecond), 2, 1) {
		t.Fatal("triple click activated twice")
	}
	c = clickTracker{}
	c.click("a", now, 0, 0)
	if c.click("b", now.Add(100*time.Millisecond), 0, 0) {
		t.Fatal("different rows activated")
	}
	c = clickTracker{}
	c.click("a", now, 0, 0)
	if c.click("a", now.Add(time.Second), 0, 0) {
		t.Fatal("slow clicks activated")
	}
	c = clickTracker{}
	c.click("a", now, 0, 0)
	if c.click("a", now.Add(100*time.Millisecond), 20, 0) {
		t.Fatal("distant clicks activated")
	}
}

func TestFileListActivatesFolderAfterDoubleClick(t *testing.T) {
	model := NewDemoModel()
	opened := ""
	transferred := ""
	list := &fileList{model: model, activate: func(dir string) { opened = dir }, fileAction: func(item sftp.Item) { transferred = item.Path }}
	list.Widget = listview.New(listview.ItemCount(len(model.Visible)), listview.FixedItemHeight(28), listview.SelectionModeOpt(listview.SelectionSingle), listview.OnItemClick(func(i int) { list.clicked = i }))
	ctx := widget.NewContext()
	list.Layout(ctx, geometry.Constraints{MinWidth: 400, MaxWidth: 400, MinHeight: 300, MaxHeight: 300})
	list.SetBounds(geometry.NewRect(0, 0, 400, 300))
	now := time.Unix(100, 0)
	click := func(t time.Time, y float32) {
		list.Event(ctx, event.NewMouseEventWithTime(event.MousePress, event.ButtonLeft, event.ButtonStateLeft, geometry.Pt(40, y), geometry.Pt(40, y), 0, t))
	}
	click(now, 10)
	if opened != "" {
		t.Fatal("single click opened folder")
	}
	click(now.Add(100*time.Millisecond), 10)
	if opened != model.Visible[0].Path {
		t.Fatalf("opened %q", opened)
	}
	opened = ""
	click(now.Add(time.Second), 94)
	click(now.Add(time.Second+100*time.Millisecond), 94)
	if opened != "" {
		t.Fatal("file should not open as a folder")
	}
	if transferred != model.Visible[3].Path {
		t.Fatalf("double click did not transfer file: %q", transferred)
	}
}

func TestCheckboxSelectionIndependentOfNavigation(t *testing.T) {
	m := NewDemoModel()
	opened := ""
	l := &fileList{model: m, activate: func(dir string) { opened = dir }}
	l.Widget = listview.New(listview.ItemCount(len(m.Visible)), listview.FixedItemHeight(28), listview.SelectionModeOpt(listview.SelectionSingle), listview.OnItemClick(func(i int) { l.clicked = i; m.Focus(i) }), listview.OnSelectionChange(m.Focus))
	ctx := widget.NewContext()
	l.Layout(ctx, geometry.Constraints{MinWidth: 400, MaxWidth: 400, MinHeight: 300, MaxHeight: 300})
	l.SetBounds(geometry.NewRect(0, 0, 400, 300))
	now := time.Unix(100, 0)
	click := func(x, y float32) {
		now = now.Add(100 * time.Millisecond)
		l.Event(ctx, event.NewMouseEventWithTime(event.MousePress, event.ButtonLeft, event.ButtonStateLeft, geometry.Pt(x, y), geometry.Pt(x, y), 0, now))
	}
	click(18, 10)
	click(18, 38)
	if m.SelectionCount() != 2 {
		t.Fatal("checkbox clicks should accumulate")
	}
	click(80, 66)
	if m.SelectionCount() != 2 {
		t.Fatal("row focus cleared checks")
	}
	click(18, 10)
	if m.SelectionCount() != 1 || opened != "" {
		t.Fatal("checkbox should uncheck, not open")
	}
	l.Event(ctx, &event.KeyEvent{KeyType: event.KeyPress, Key: event.KeySpace})
	if m.SelectionCount() != 2 {
		t.Fatal("space should toggle focused checkbox")
	}
	click(80, 66)
	click(80, 66)
	if opened != m.Visible[2].Path {
		t.Fatal("double click should still open folder")
	}
	m.ToggleAll()
	if m.SelectionCount() != len(m.Visible) {
		t.Fatal("select all should check remaining rows")
	}
	m.ToggleAll()
	if m.SelectionCount() != 0 {
		t.Fatal("select all should clear checks")
	}
}
