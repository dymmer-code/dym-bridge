package gpuui

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	_ "github.com/gogpu/gg/gpu"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/core/listview"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/desktop"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/theme/fluent"
	"github.com/gogpu/ui/widget"
)

type browser struct {
	ui            *app.App
	gpu           *gogpu.App
	theme         *fluent.Theme
	local, remote *Model
	dark          bool
	status        state.Signal[string]
	selection     state.Signal[string]
}

// Run starts the read-only prototype. No SSH connection or file mutation is made.
func Run(dir string) error {
	b := &browser{local: NewModel(dir), remote: NewDemoModel(), status: state.NewSignal("Vista previa · No se realizan conexiones ni escrituras"), selection: state.NewSignal("Selecciona un archivo para ver sus detalles")}
	if err := b.local.Navigate(dir); err != nil {
		return err
	}
	b.gpu = gogpu.NewApp(gogpu.DefaultConfig().WithTitle("Dym Bridge · Vista previa GoGPU").WithSize(1180, 760))
	b.setTheme()
	b.ui = app.New(app.WithWindowProvider(b.gpu), app.WithPlatformProvider(b.gpu), app.WithEventSource(b.gpu.EventSource()), app.WithTheme(b.theme.AsTheme()))
	b.rebuild()
	return desktop.Run(b.gpu, b.ui)
}
func (b *browser) setTheme() {
	opts := fluent.WithAccentColor(widget.Hex(0x527CAF))
	if b.dark {
		b.theme = fluent.NewDarkTheme(opts)
	} else {
		b.theme = fluent.NewTheme(opts)
	}
}
func (b *browser) text(s string, size float32) *p.TextWidget {
	return p.Text(s).FontSize(size).Color(b.theme.Colors.OnSurface).MaxLines(1).Ellipsis()
}
func (b *browser) action(label string, primary bool, fn func()) *button.Widget {
	variant := button.Outlined
	if primary {
		variant = button.Filled
	}
	return button.New(button.TextOpt(label), button.VariantOpt(variant), button.OnClick(fn), button.PainterOpt(fluent.ButtonPainter{Theme: b.theme}))
}
func (b *browser) navigate(m *Model, dir string) {
	if err := m.Navigate(dir); err != nil {
		b.status.Set(err.Error())
		return
	}
	b.selection.Set("Selecciona un archivo para ver sus detalles")
	b.status.Set("Carpeta abierta · " + dir)
	b.rebuild()
}
func (b *browser) rebuild() {
	cs := b.theme.Colors
	side := []widget.Widget{b.text("DYM BRIDGE", 19).Bold(), b.text("Tu espacio de trabajo", 12), p.Box().Height(24), b.text("EXPLORAR", 11).Bold(), b.action("Archivos", true, func() { b.navigate(b.remote, "/wp-content") }), p.Box().Height(16), b.text("ACCESOS LOCALES", 11).Bold()}
	dirs := local.CommonDirectories()
	for _, name := range []string{"Home", "Downloads", "Desktop", "Documents"} {
		if dir, ok := dirs[name]; ok {
			side = append(side, b.action(name, false, func() { b.navigate(b.local, dir) }))
		}
	}
	side = append(side, p.Expanded(p.Box()))
	side = append(side, b.text("LABORATORIO", 11).Bold())
	side = append(side, b.action("10.000 archivos", false, func() {
		b.remote.Stress()
		b.status.Set("Prueba sintética · 10.000 archivos virtualizados")
		b.rebuild()
	}))
	side = append(side, b.action("Cambiar tema", false, func() { b.dark = !b.dark; b.setTheme(); b.ui.SetTheme(b.theme.AsTheme()); b.rebuild() }))
	sidebar := p.VBox(side...).Width(194).Padding(20).Gap(12).Background(cs.SurfaceSecondary)
	header := p.HBox(p.Expanded(p.VBox(b.text("Archivos", 28).Bold(), b.text("Del equipo al servidor, con todo a la vista", 14)).Gap(5)), p.Box(b.text("SERVIDOR DE DEMOSTRACIÓN", 11).Bold()).PaddingXY(14, 10).Rounded(8).Background(cs.AccentLight)).Gap(20)
	panels := p.HBox(p.Expanded(b.pane(b.local, "Este equipo", "Navegación local · solo lectura")), p.Expanded(b.pane(b.remote, "Sitio de ejemplo", "Datos ficticios · sin conexión SSH"))).Gap(18)
	details := p.VBox(b.text("DETALLES", 11).Bold(), p.TextFn(func() string { return b.selection.Get() }).FontSize(13).Color(cs.OnSurface).MaxLines(1).Ellipsis()).Padding(16).Gap(8).Rounded(10).Background(cs.SurfaceSecondary)
	transfers := p.HBox(p.Expanded(p.VBox(b.text("Transferencias", 16).Bold(), b.text("Esta prueba evalúa la interfaz. Subidas y descargas aún no están conectadas.", 12)).Gap(5)), button.New(button.TextOpt("Sin transferencias"), button.Disabled(true), button.PainterOpt(fluent.ButtonPainter{Theme: b.theme}))).Padding(16).Rounded(10).Background(cs.SurfaceSecondary)
	footer := p.TextFn(func() string { return b.status.Get() }).FontSize(12).Color(cs.OnSurfaceSecond).MaxLines(1).Ellipsis()
	content := p.VBox(header, p.Expanded(panels), details, transfers, footer).Padding(24).Gap(18).Background(cs.Surface)
	b.ui.SetRoot(p.HBox(sidebar, p.Expanded(content)))
	b.gpu.RequestRedraw()
}
func (b *browser) pane(m *Model, title, subtitle string) widget.Widget {
	cs := b.theme.Colors
	var list *listview.Widget
	count := state.NewSignal(len(m.Visible))
	summary := state.NewSignal(fmt.Sprintf("%d elementos", len(m.Visible)))
	list = listview.New(listview.ItemCountSignal(count), listview.FixedItemHeight(44), listview.SelectionModeOpt(listview.SelectionSingle), listview.A11yLabel(title), listview.Divider(true), listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
		if ctx.Index >= len(m.Visible) {
			return p.Box()
		}
		item := m.Visible[ctx.Index]
		kind := "ARCHIVO"
		size := sftp.FormatFileSize(item.Size)
		if item.IsDir {
			kind = "CARPETA"
			size = "—"
		}
		name := b.text(item.Name, 14)
		if item.IsDir {
			name.Bold()
		}
		row := p.HBox(p.Expanded(name), p.Box(b.text(size, 12)).Width(64), p.Box(b.text(kind, 10)).Width(64)).PaddingXY(12, 10).Gap(8)
		if ctx.Selected {
			row.Background(cs.AccentLight)
		}
		return row
	}), listview.OnSelectionChange(func(i int) {
		m.Select(i)
		if item := m.Selected(); item != nil {
			b.selection.Set(item.Name + " · " + sftp.FormatFileSize(item.Size) + " · " + item.Path)
		}
	}))
	query := textfield.New(textfield.Placeholder("Buscar en esta carpeta…"), textfield.InitialValue(m.Query), textfield.A11yLabel("Buscar en "+title), textfield.PainterOpt(fluent.TextFieldPainter{Theme: b.theme}), textfield.OnChange(func(q string) {
		m.Filter(q)
		count.Set(len(m.Visible))
		summary.Set(fmt.Sprintf("%d de %d elementos", len(m.Visible), len(m.Items)))
		list.InvalidateData()
		b.selection.Set("Selecciona un archivo para ver sus detalles")
	}))
	up := b.action("Subir", false, func() {
		parent := filepath.Dir(m.Path)
		if m.Demo {
			parent = path.Dir(m.Path)
		}
		b.navigate(m, parent)
	})
	open := b.action("Abrir carpeta", false, func() {
		if item := m.Selected(); item != nil && item.IsDir {
			b.navigate(m, item.Path)
		} else {
			b.status.Set("Selecciona una carpeta y pulsa Abrir carpeta")
		}
	})
	return p.VBox(p.VBox(b.text(title, 18).Bold(), b.text(subtitle, 12)).Gap(4), p.HBox(up, b.action("Actualizar", false, func() { b.navigate(m, m.Path) }), open).Gap(8), b.text(m.Path, 12), query, p.HBox(p.Expanded(b.text("NOMBRE", 10).Bold()), p.Box(b.text("TAMAÑO", 10)).Width(64), p.Box(b.text("TIPO", 10)).Width(64)).PaddingXY(12, 0).Gap(8), p.Expanded(list), p.TextFn(func() string { return summary.Get() }).FontSize(12).Color(cs.OnSurfaceSecond)).Padding(16).Gap(12).Rounded(12).BorderStyle(1, cs.StrokeDefault).Background(cs.SurfaceSecondary)
}
