package gpuui

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	_ "github.com/gogpu/gg/gpu"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/core/checkbox"
	"github.com/gogpu/ui/core/listview"
	"github.com/gogpu/ui/core/toolbar"
	"github.com/gogpu/ui/desktop"
	"github.com/gogpu/ui/icon"
	p "github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/state"
	"github.com/gogpu/ui/theme/fluent"
	"github.com/gogpu/ui/widget"
)

var documentIcon = icon.FromSVG("document", 24, "M6 3 L15 3 L19 7 L19 21 L6 21 Z M15 3 L15 7 L19 7 M9 11 L16 11 M9 14 L16 14 M9 17 L14 17")

type browser struct {
	ui            *app.App
	gpu           *gogpu.App
	theme         *fluent.Theme
	local, remote *Model
	dark          bool
	transfersOpen bool
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
	b.ui = app.New(app.WithWindowProvider(b.gpu), app.WithPlatformProvider(b.gpu), app.WithEventSource(adaptInput(b.gpu.EventSource())), app.WithTheme(b.theme.AsTheme()))
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
func (b *browser) text(s string, size float32) *cellText {
	return &cellText{TextWidget: p.Text(s).FontSize(size).Color(b.theme.Colors.OnSurface).MaxLines(1), size: size, color: b.theme.Colors.OnSurface}
}
func (b *browser) action(label string, primary bool, fn func()) *button.Widget {
	variant := button.TextOnly
	if primary {
		variant = button.Filled
	}
	return button.New(button.TextOpt(label), button.SizeOpt(button.Small), button.VariantOpt(variant), button.OnClick(fn), button.PainterOpt(fluent.ButtonPainter{Theme: b.theme}))
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
func (b *browser) rule() widget.Widget {
	return p.Box().Height(1).Background(b.theme.Colors.StrokeDefault)
}
func (b *browser) rebuild() {
	cs := b.theme.Colors
	header := p.HBox(
		b.text("Dym Bridge", 15).Bold(),
		p.Expanded(b.text("Sitio de ejemplo · demostración", 12)),
		b.action("10.000 archivos", false, func() { b.remote.Stress(); b.status.Set("Prueba sintética · 10.000 archivos"); b.rebuild() }),
		b.action("Tema", false, func() { b.dark = !b.dark; b.setTheme(); b.ui.SetTheme(b.theme.AsTheme()); b.rebuild() }),
	).PaddingXY(12, 4).Gap(12).CrossAlign(p.CrossAxisCenter).Background(cs.SurfaceSecondary)
	panels := p.HBox(p.Expanded(b.pane(b.local, "Este equipo", "Solo lectura")), p.Box().Width(1).Background(cs.StrokeDefault), p.Expanded(b.pane(b.remote, "Sitio de ejemplo", "Servidor de demostración")))
	label := "+ Transferencias"
	if b.transfersOpen {
		label = "- Transferencias"
	}
	transferBar := p.HBox(b.action(label, false, func() { b.transfersOpen = !b.transfersOpen; b.rebuild() }), p.Expanded(b.text("0 en cola", 12)), b.text("Vista previa · sin conexiones ni escrituras", 11)).PaddingXY(8, 0).Gap(12).CrossAlign(p.CrossAxisCenter).Background(cs.SurfaceSecondary)
	children := []widget.Widget{header, b.rule(), p.Expanded(panels), b.rule(), transferBar}
	if b.transfersOpen {
		children = append(children, b.rule(), p.VBox(b.text("No hay transferencias", 13), b.text("Subidas y descargas aún no están conectadas en este prototipo.", 12)).PaddingXY(16, 12).Gap(4).Background(cs.Surface))
	}
	footer := p.TextFn(func() string { return b.status.Get() }).FontSize(11).Color(cs.OnSurfaceSecond).MaxLines(1).Ellipsis()
	children = append(children, b.rule(), p.Box(footer).PaddingXY(12, 5).Background(cs.SurfaceSecondary))
	b.ui.SetRoot(p.VBox(children...).Background(cs.Surface))
	b.gpu.RequestRedraw()
}
func (b *browser) pane(m *Model, title, subtitle string) widget.Widget {
	cs := b.theme.Colors
	list := &fileList{model: m, clicked: -1, activate: func(dir string) { b.navigate(m, dir) }}
	count := state.NewSignal(len(m.Visible))
	summary := state.NewSignal(fmt.Sprintf("%d elementos · %d seleccionados", len(m.Visible), m.SelectionCount()))
	updateSummary := func() {
		summary.Set(fmt.Sprintf("%d elementos · %d seleccionados", len(m.Visible), m.SelectionCount()))
	}
	list.changed = updateSummary
	list.Widget = listview.New(listview.OnItemClick(func(i int) { list.clicked = i; m.Focus(i); list.InvalidateData(); updateSummary() }), listview.ItemCountSignal(count), listview.FixedItemHeight(28), listview.SelectionModeOpt(listview.SelectionSingle), listview.A11yLabel(title), listview.Divider(false), listview.BuildItem(func(ctx listview.ItemContext) widget.Widget {
		if ctx.Index >= len(m.Visible) {
			return p.Box()
		}
		item := m.Visible[ctx.Index]
		size := sftp.FormatFileSize(item.Size)
		if item.IsDir {
			size = "—"
		}
		name := b.text(item.Name, 13)
		image := documentIcon
		tint := cs.OnSurfaceSecond
		label := "Archivo"
		if item.IsDir {
			image = icon.FolderClosed
			tint = cs.Accent
			label = "Carpeta"
		}
		glyph := icon.NewIcon(image, icon.Size(16), icon.Color(tint), icon.Label(label))
		check := checkbox.New(checkbox.Checked(m.IsSelected(item.Path)), checkbox.A11yHint("Seleccionar "+item.Name), checkbox.PainterOpt(fluent.CheckboxPainter{Theme: b.theme}))
		row := p.HBox(p.Box(check).Width(20).Height(18), glyph, p.Expanded(name), p.Box(b.text(size, 11)).Width(66), p.Box(b.text(item.ModTime.Format("02/01/06 15:04"), 11)).Width(100)).PaddingXY(10, 5).Gap(8)

		if ctx.Index%2 == 1 {
			row.Background(cs.SurfaceSecondary)
		}
		if m.IsSelected(item.Path) {
			row.Background(cs.AccentLight)
		}

		return row
	}), listview.OnSelectionChange(func(i int) {
		m.Focus(i)
		list.InvalidateData()
		updateSummary()
		if item := m.Selected(); item != nil {
			b.status.Set(item.Name + " · " + sftp.FormatFileSize(item.Size) + " · " + item.Path)
		}
	}))
	parent := func() {
		dir := filepath.Dir(m.Path)
		if m.Demo {
			dir = path.Dir(m.Path)
		}
		b.navigate(m, dir)
	}
	// Two rows only: identity/path, then familiar compact file commands.
	heading := p.HBox(b.text(title, 12).Bold(), p.Expanded(b.text(m.Path, 12))).PaddingXY(10, 5).Gap(12).CrossAlign(p.CrossAxisCenter).Background(cs.SurfaceSecondary)
	transferLabel := "Subir"
	transferIcon := uploadIcon
	if m.Demo {
		transferLabel = "Descargar"
		transferIcon = downloadIcon
	}
	disabled := func(label string, image icon.IconData) toolbar.Item {
		item := toolbar.IconButton(label, image, nil)
		item.Enabled = false
		return item
	}
	actions := newPaneToolbar(b.theme,
		toolbar.IconButton("Carpeta superior", upIcon, parent),
		toolbar.IconButton("Actualizar", icon.Refresh, func() { b.navigate(m, m.Path) }),
		toolbar.Separator(),
		toolbar.IconButton("Seleccionar/deseleccionar todo", icon.Check, func() {
			m.ToggleAll()
			list.clicks = clickTracker{}
			list.InvalidateData()
			updateSummary()
			b.status.Set(fmt.Sprintf("%d elementos seleccionados en %s", m.SelectionCount(), title))
		}),
		disabled("Nueva carpeta", newFolderIcon), disabled("Eliminar", icon.Delete),
		toolbar.Separator(), disabled(transferLabel, transferIcon),
		toolbar.Spacer(), toolbar.Custom(b.text(subtitle, 11)),
	)
	columns := p.HBox(p.Box().Width(20), p.Box().Width(16), p.Expanded(b.text("Nombre", 11)), p.Box(b.text("Tamaño", 11)).Width(66), p.Box(b.text("Modificado", 11)).Width(100)).PaddingXY(10, 5).Gap(8).Background(cs.SurfaceSecondary)
	countbar := p.Box(p.TextFn(func() string { return summary.Get() }).FontSize(11).Color(cs.OnSurfaceSecond)).PaddingXY(10, 5).Background(cs.SurfaceSecondary)
	return p.VBox(heading, actions, b.rule(), columns, b.rule(), p.Expanded(list), b.rule(), countbar).Background(cs.Surface)
}
