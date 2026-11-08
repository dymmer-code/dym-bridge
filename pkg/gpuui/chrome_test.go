package gpuui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/core/menu"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

type chromeCanvas struct {
	widget.Canvas
	text        []string
	backgrounds []widget.Color
}

func (c *chromeCanvas) DrawRect(_ geometry.Rect, color widget.Color) {
	c.backgrounds = append(c.backgrounds, color)
}
func (c *chromeCanvas) DrawRoundRect(r geometry.Rect, color widget.Color, _ float32) {
	c.DrawRect(r, color)
}
func (c *chromeCanvas) StrokeRoundRect(geometry.Rect, widget.Color, float32, float32)  {}
func (c *chromeCanvas) DrawLine(geometry.Point, geometry.Point, widget.Color, float32) {}
func (c *chromeCanvas) PushClip(geometry.Rect)                                         {}
func (c *chromeCanvas) PopClip()                                                       {}
func (c *chromeCanvas) DrawText(text string, _ geometry.Rect, _ float32, _ widget.Color, _ bool, _ widget.TextAlign) {
	if text != "" {
		c.text = append(c.text, text)
	}
}
func formControls(b *browser) ([]string, int) {
	c := &chromeCanvas{}
	fields := 0
	var visit func(widget.Widget)
	visit = func(w widget.Widget) {
		if _, ok := w.(*textfield.Widget); ok {
			fields++
		}
		if btn, ok := w.(*button.Widget); ok {
			btn.SetBounds(geometry.NewRect(0, 0, 300, 32))
			btn.Draw(widget.NewContext(), c)
		}
		for _, child := range w.Children() {
			visit(child)
		}
	}
	visit(b.ui.Window().Root())
	return c.text, fields
}
func TestConnectionOffersPickerOnlyWithoutResources(t *testing.T) {
	b := testBrowser(t)
	b.showConnection()
	labels, fields := formControls(b)
	if fields != 0 || !slices.Contains(labels, "Choose INI file…") || !slices.Contains(labels, "Create INI file…") || !slices.Contains(labels, "Sign in with Dymmer") {
		t.Fatalf("%v fields=%d", labels, fields)
	}
	b.configPath = filepath.Join(t.TempDir(), "connection.ini")
	if err := os.WriteFile(b.configPath, []byte(config.SampleConfigTemplate()), 0600); err != nil {
		t.Fatal(err)
	}
	b.showConnection()
	labels, fields = formControls(b)
	if fields != 0 || len(labels) != 2 || labels[0] != "Back" {
		t.Fatalf("saved INI should expose just Back and resource: %v", labels)
	}
	b.configPath = filepath.Join(t.TempDir(), "missing.ini")
	b.showConnection()
	labels, _ = formControls(b)
	if !slices.Contains(labels, "Choose INI file…") {
		t.Fatal(labels)
	}
}
func TestSavedProfilesShowOnlyResourceActions(t *testing.T) {
	b := testBrowser(t)
	profiles := &provision.Profiles{Resources: []provision.Resource{{Name: "My site"}}}
	if err := profiles.Save(b.profilesFile); err != nil {
		t.Fatal(err)
	}
	b.showConnection()
	labels, fields := formControls(b)
	if fields != 0 || len(labels) != 2 || labels[1] != profiles.Resources[0].Label() {
		t.Fatal(labels, fields)
	}
	b.signIn()
	_, fields = formControls(b)
	if fields != 2 {
		t.Fatalf("optional password and confirmation only, got %d fields", fields)
	}
}
func TestSitesPaintUsesThemeForBarAndPopup(t *testing.T) {
	b := testBrowser(t)
	var backgrounds []widget.Color
	for _, dark := range []bool{false, true} {
		b.dark = dark
		b.setTheme()
		bar := b.sitesMenu().(*menu.Bar)
		c := &chromeCanvas{}
		bar.SetBounds(geometry.NewRect(0, 0, 90, 32))
		bar.Draw(widget.NewContext(), c)
		if len(c.backgrounds) == 0 || c.backgrounds[0] != b.theme.Colors.SurfaceSecondary {
			t.Fatal("bar ignored theme")
		}
		backgrounds = append(backgrounds, c.backgrounds[0])
		c = &chromeCanvas{}
		paintSitesPopup(c, &menu.MenuPaintState{Bounds: geometry.NewRect(0, 0, 200, 44), ColorScheme: menu.MenuColorScheme{MenuBackground: b.theme.Colors.Surface}, Items: []menu.MenuItem{menu.ItemDisabled("No interfaces provided", "")}, ItemHeight: 36, HighlightedIndex: -1})
		if c.backgrounds[0] != b.theme.Colors.Surface {
			t.Fatal("popup ignored theme")
		}
	}
	if backgrounds[0] == backgrounds[1] {
		t.Fatal("light/dark backgrounds match")
	}
}
func TestCloseDefersWindowMutation(t *testing.T) {
	b := testBrowser(t)
	// No native window exists: the callback must only record intent, never touch it.
	if b.requestHide() {
		t.Fatal("close should hide to tray")
	}
	if !b.hideRequested {
		t.Fatal("hide intent lost")
	}
	b.flushWindowActions()
	if b.hideRequested {
		t.Fatal("hide intent not consumed")
	}
}

func TestHelpWrapsWithinFieldWidth(t *testing.T) {
	text := "The master password protects the private SSH key used to connect to your server. Leave it blank to store the key unencrypted on this computer."
	measure := func(s string) float32 { return float32(len([]rune(s))) * 6 }
	for _, width := range []float32{552, 300, 120} {
		lines := wrapWords(text, width, measure)
		if len(lines) < 2 {
			t.Fatal("paragraph did not wrap")
		}
		for _, line := range lines {
			if measure(line) > width {
				t.Fatalf("line exceeds field width: %q", line)
			}
		}
		if strings.Join(lines, " ") != text {
			t.Fatal("wrapping lost text")
		}
	}
	for _, line := range wrapWords("界界界界界界", 12, measure) {
		if measure(line) > 12 {
			t.Fatal("long Unicode word did not wrap")
		}
	}
	b := testBrowser(t)
	help := b.helpText(text)
	size := help.Layout(widget.NewContext(), geometry.Constraints{MaxWidth: 300, MaxHeight: 1000})
	c := &chromeCanvas{}
	help.Draw(widget.NewContext(), c)
	if len(c.text) < 2 || size.Width != 300 || size.Height < float32(len(c.text))*18 {
		t.Fatalf("bad wrapped layout: %+v %v", size, c.text)
	}
}
func (c *chromeCanvas) MeasureText(s string, _ float32, _ bool) float32 {
	return float32(len([]rune(s))) * 6
}
