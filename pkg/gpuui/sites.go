package gpuui

import (
	"fmt"

	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/dymmer-code/dym-bridge/pkg/sites"
	"github.com/gogpu/ui/core/menu"
	"github.com/gogpu/ui/core/scrollview"
	"github.com/gogpu/ui/core/textfield"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/widget"
)

func (b *browser) sitesKey() string {
	if b.cfg == nil {
		return ""
	}
	return fmt.Sprintf("%s@%s:%d", b.cfg.SSH.User, b.cfg.SSH.Host, b.cfg.SSH.Port)
}
func (b *browser) loadSites() {
	b.siteLinks = nil
	filename, err := sites.DefaultPath()
	if err == nil {
		b.siteLinks, _, err = sites.Load(filename, b.sitesKey())
	}
	b.siteLoadError = err
}
func (b *browser) sitesMenu() widget.Widget {
	var items []menu.MenuItem
	for _, link := range b.siteLinks {
		if !b.connected || b.busy {
			items = append(items, menu.ItemDisabled(link.Name, ""))
		} else {
			items = append(items, menu.Item(link.Name, "", func() { b.openSite(link) }))
		}
	}
	if len(items) == 0 {
		items = append(items, menu.ItemDisabled("No sites configured", ""))
	}
	items = append(items, menu.Sep())
	if b.busy {
		items = append(items, menu.ItemDisabled("Configure sites…", ""))
	} else {
		items = append(items, menu.Item("Configure sites…", "", b.configureSites))
	}
	cs := b.theme.Colors
	painter := sitesMenuPainter{colors: menu.MenuColorScheme{BarBackground: cs.SurfaceSecondary, BarText: cs.OnSurface, BarHover: cs.AccentLight, BarActiveText: cs.Accent, MenuBackground: cs.Surface, MenuBorder: cs.StrokeDefault, ItemText: cs.OnSurface, ItemHover: cs.AccentLight, ItemDisabledText: cs.OnSurfaceSecond, ShortcutText: cs.OnSurfaceSecond, SeparatorColor: cs.StrokeDefault, SubMenuArrow: cs.OnSurface}}
	return menu.NewBar([]menu.TopMenu{menu.BarMenu("Sites", items...)}, menu.PainterOpt(painter))
}

type sitesMenuPainter struct{ colors menu.MenuColorScheme }

func (p sitesMenuPainter) PaintMenuBar(c widget.Canvas, s *menu.MenuBarPaintState) {
	s.ColorScheme = p.colors
	menu.DefaultPainter{}.PaintMenuBar(c, s)
}
func (p sitesMenuPainter) PaintMenu(c widget.Canvas, s *menu.MenuPaintState) {
	s.ColorScheme = p.colors
	menu.DefaultPainter{}.PaintMenu(c, s)
}
func (b *browser) openSite(link sites.Link) {
	if b.busy || !b.connected {
		return
	}
	var localURL string
	b.runOperation("Opening "+link.Name+"…", func() error {
		var err error
		localURL, err = b.session.siteURL(link.URL)
		return err
	}, func(err error) {
		if err == nil {
			err = provision.OpenBrowser(localURL)
		}
		if err != nil {
			b.status.Set("Could not open site: " + err.Error())
			return
		}
		b.status.Set("Opened " + link.Name + " through SSH")
	})
}
func (b *browser) configureSites() {
	if b.busy {
		return
	}
	if b.cfg == nil {
		b.showForm("Sites", b.text("Choose a connection before configuring its sites.", 12), b.action("Connection…", true, b.showConnection))
		return
	}
	b.loadSites()
	if b.siteLoadError != nil {
		b.showForm("Sites", b.text("Could not read saved sites. The file has not been changed.", 12), b.text(b.siteLoadError.Error(), 12))
		return
	}
	b.editSites(append([]sites.Link(nil), b.siteLinks...))
}
func (b *browser) editSites(draft []sites.Link) {
	type fields struct{ name, url *textfield.Widget }
	rows := make([]fields, len(draft))
	message := state.NewSignal("HTTP(S) URLs are reached from the SSH server, through the tunnel.")
	collect := func() []sites.Link {
		links := make([]sites.Link, len(rows))
		for i, row := range rows {
			links[i] = sites.Link{Name: row.name.Text(), URL: row.url.Text()}
		}
		return links
	}
	var content []widget.Widget
	for i, link := range draft {
		name, url := b.field("Name", false), b.field("http://127.0.0.1:3000/", false)
		name.SetText(link.Name)
		url.SetText(link.URL)
		rows[i] = fields{name, url}
		content = append(content, p.VBox(p.HBox(p.Expanded(name), b.action("Remove", false, func() { all := collect(); b.editSites(append(all[:i], all[i+1:]...)) })).Gap(8), url).Gap(4))
	}
	if len(content) == 0 {
		content = append(content, b.text("No sites yet. Add a name and URL to get started.", 12))
	}
	height := float32(min(310, max(28, len(draft)*116)))
	b.showForm("Configure sites", b.text(b.sitesKey(), 12), p.Box(scrollview.New(p.VBox(content...).Gap(16))).Height(height),
		p.TextFn(func() string { return message.Get() }).FontSize(12).MaxLines(2),
		p.HBox(b.action("Add site", false, func() { b.editSites(append(collect(), sites.Link{})) }), p.Expanded(p.Box()), b.action("Save", true, func() {
			filename, err := sites.DefaultPath()
			if err == nil {
				err = sites.Save(filename, b.sitesKey(), collect())
			}
			if err != nil {
				message.Set(err.Error())
				return
			}
			b.loadSites()
			b.status.Set("Sites saved")
			b.rebuild()
		})).Gap(8))
}
