package gpuui

import (
	"context"
	"fmt"
	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/dymmer-code/dym-bridge/pkg/sites"
	"path"
	"path/filepath"
	"sync"

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

type browser struct {
	profiles        *provision.Profiles
	profilesFile    string
	callbackFile    string
	openURL         func(string) error
	server          string
	loginCancel     context.CancelFunc
	stopping        chan struct{}
	desktopReady    bool
	desktopClose    func()
	connectionEpoch uint64

	ui              *app.App
	gpu             *gogpu.App
	theme           *fluent.Theme
	local, remote   *Model
	dark            bool
	transfersOpen   bool
	status          state.Signal[string]
	selection       state.Signal[string]
	session         session
	cfg             *config.Config
	configPath      string
	siteLinks       []sites.Link
	manualSites     []sites.Link
	siteLoadError   error
	connected, busy bool
	updates         chan func()
	progress        chan transferProgress
	workers         sync.WaitGroup
	transferText    state.Signal[string]
}

// Run starts the native GoGPU explorer. Connections are initiated explicitly.
func Run(dir string) error { return RunWithConfig(dir, "") }
func RunWithConfig(dir, configPath string) error {
	b := &browser{local: NewModel(dir), remote: NewModel("/"), status: state.NewSignal("Disconnected"), selection: state.NewSignal(""), configPath: configPath,
		stopping: make(chan struct{}), updates: make(chan func(), 16), progress: make(chan transferProgress, 1),
		transferText: state.NewSignal("No transfers")}
	if err := b.local.Navigate(dir); err != nil {
		return err
	}
	if configPath == "" {
		b.configPath, _ = config.FindConfigFile("")
	}
	b.gpu = gogpu.NewApp(gogpu.DefaultConfig().WithTitle("Dym Bridge").WithSize(1180, 760))
	b.setTheme()
	b.ui = app.New(app.WithWindowProvider(b.gpu), app.WithPlatformProvider(b.gpu), app.WithEventSource(adaptInput(b.gpu.EventSource())), app.WithTheme(b.theme.AsTheme()))
	b.gpu.OnUpdate(func(float64) { b.initDesktop(); b.drainUpdates() })

	b.rebuild()
	b.showConnection()
	err := desktop.Run(b.gpu, b.ui)
	close(b.stopping)
	if b.loginCancel != nil {
		b.loginCancel()
	}
	b.session.abort()
	b.workers.Wait()
	b.session.close()
	return err
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
func (b *browser) action(label string, primary bool, fn func(), extra ...button.Option) *button.Widget {
	variant := button.TextOnly
	if primary {
		variant = button.Filled
	}
	return button.New(append([]button.Option{button.TextOpt(label), button.SizeOpt(button.Small), button.VariantOpt(variant), button.OnClick(fn), button.PainterOpt(fluent.ButtonPainter{Theme: b.theme})}, extra...)...)
}
func (b *browser) navigate(m *Model, dir string) {
	if b.busy {
		return
	}
	if m == b.remote {
		b.navigateRemote(dir)
		return
	}
	if err := m.Navigate(dir); err != nil {
		b.status.Set(err.Error())
		return
	}
	b.selection.Set("Select a file to view its details")
	b.status.Set("Folder opened · " + dir)
	b.rebuild()
}
func (b *browser) rule() widget.Widget {
	return p.Box().Height(1).Background(b.theme.Colors.StrokeDefault)
}
func (b *browser) rebuild() {
	cs := b.theme.Colors
	header := p.HBox(
		p.Box(b.sitesMenu()).Width(90), p.Expanded(p.Box()),
		b.action("Theme", false, func() { b.dark = !b.dark; b.setTheme(); b.ui.SetTheme(b.theme.AsTheme()); b.rebuild() }),
	).PaddingXY(8, 4).Gap(8).CrossAlign(p.CrossAxisCenter).Background(cs.SurfaceSecondary)
	remoteTitle, remoteStatus := "Remote server", "Disconnected"
	if b.connected {
		remoteTitle = b.cfg.SSH.Host
		remoteStatus = "Connected"
	}
	panels := p.HBox(p.Expanded(b.pane(b.local, "Local computer", "")), p.Box().Width(1).Background(cs.StrokeDefault), p.Expanded(b.pane(b.remote, remoteTitle, remoteStatus)))
	children := []widget.Widget{header, b.rule(), p.Expanded(panels)}
	if b.transfersOpen {
		progress := p.TextFn(func() string { return b.transferText.Get() }).FontSize(12).Color(cs.OnSurface).MaxLines(1).Ellipsis()
		children = append(children, b.rule(), p.HBox(p.Expanded(progress), b.action("Hide", false, func() { b.transfersOpen = false; b.rebuild() })).PaddingXY(12, 4).CrossAlign(p.CrossAxisCenter).Background(cs.SurfaceSecondary))
	}
	footer := &cellText{TextWidget: p.TextFn(func() string { return b.status.Get() }).FontSize(12), size: 12, color: cs.OnSurfaceSecond, center: true}
	footerItems := []toolbar.Item{}
	if !b.transfersOpen && b.transferText.Get() != "No transfers" {
		footerItems = append(footerItems, toolbar.IconButton("Show transfers", transferHistoryIcon, func() { b.transfersOpen = true; b.rebuild() }))
	}
	if b.connected {
		footerItems = append(footerItems, toolbar.IconButton("Disconnect", disconnectIcon, b.disconnect))
	} else {
		label := "Connect"
		if b.cfg != nil {
			label = "Reconnect"
		}
		footerItems = append(footerItems, toolbar.IconButton(label, connectIcon, b.connectAction))
	}
	footerItems = append(footerItems, toolbar.IconButton("Connection settings", icon.Settings, b.showConnection))
	for i := range footerItems {
		footerItems[i].Enabled = !b.busy
	}
	footerToolbar := newPaneToolbar(b.theme, footerItems...)
	footerToolbar.overlays = b.ui.Window().Overlays()
	children = append(children, b.rule(), p.HBox(p.Expanded(footer), p.Box(footerToolbar).Width(float32(len(footerItems)*31+16))).Height(36).PaddingXY(12, 2).CrossAlign(p.CrossAxisCenter).Background(cs.SurfaceSecondary))

	b.ui.SetRoot(p.VBox(children...).Background(cs.Surface))
	b.gpu.RequestRedraw()
}
func (b *browser) pane(m *Model, title, subtitle string) widget.Widget {
	cs := b.theme.Colors
	list := &fileList{model: m, clicked: -1, activate: func(dir string) { b.navigate(m, dir) }, fileAction: func(item sftp.Item) { b.transferSelection(m == b.local, []sftp.Item{item}) }}
	count := state.NewSignal(len(m.Visible))
	summary := state.NewSignal(fmt.Sprintf("%d items · %d selected", len(m.Visible), m.SelectionCount()))
	refreshToolbar := func() {}
	updateSummary := func() {
		summary.Set(fmt.Sprintf("%d items · %d selected", len(m.Visible), m.SelectionCount()))
		refreshToolbar()
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
		glyph := b.fileGlyph(item.Name, item.IsDir)
		check := checkbox.New(checkbox.Checked(m.IsSelected(item.Path)), checkbox.A11yHint("Select "+item.Name), checkbox.PainterOpt(fluent.CheckboxPainter{Theme: b.theme}))
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
		if m == b.remote {
			dir = path.Dir(m.Path)
		}
		b.navigate(m, dir)
	}
	// Two rows only: identity/path, then familiar compact file commands.
	paneIcon := computerIcon
	if m == b.remote {
		paneIcon = serverIcon
	}
	heading := p.HBox(icon.NewIcon(paneIcon, icon.Size(18), icon.Color(cs.OnSurfaceSecond), icon.Label(title)), p.Expanded(b.text(m.Path, 12).Path()), b.action("…", false, func() { b.goToFolder(m) })).PaddingXY(10, 5).Gap(12).CrossAlign(p.CrossAxisCenter).Background(cs.SurfaceSecondary)
	transferLabel := "Upload"
	transferIcon := uploadIcon
	if m == b.remote {
		transferLabel = "Download"
		transferIcon = downloadIcon
	}
	items := []toolbar.Item{
		toolbar.IconButton("Parent folder", upIcon, parent),
		toolbar.IconButton("Refresh", icon.Refresh, func() { b.navigate(m, m.Path) }),
		toolbar.Separator(),
		toolbar.IconButton("Select/deselect all", icon.Check, func() {
			m.ToggleAll()
			list.clicks = clickTracker{}
			list.InvalidateData()
			updateSummary()
			b.status.Set(fmt.Sprintf("%d items selected in %s", m.SelectionCount(), title))
		}),
		toolbar.IconButton("New folder", newFolderIcon, func() { b.newFolder(m) }), toolbar.IconButton("Delete", icon.Delete, func() { b.deleteSelected(m) }),
		toolbar.Separator(), toolbar.IconButton(transferLabel, transferIcon, func() { b.transfer(m == b.local) }),
		toolbar.Spacer(), toolbar.Custom(b.text(subtitle, 11)),
	}
	actions := newPaneToolbar(b.theme, items...)
	refreshToolbar = func() {
		available := !b.busy && (m == b.local || b.connected)
		items[0].Enabled = available
		items[1].Enabled = available
		items[3].Enabled = available && len(m.Visible) > 0
		items[4].Enabled = available
		items[5].Enabled = available && m.SelectionCount() > 0
		items[7].Enabled = !b.busy && b.connected && m.SelectionCount() > 0
		actions.SetNeedsRedraw(true)
	}
	refreshToolbar()
	actions.overlays = b.ui.Window().Overlays()
	columns := p.HBox(p.Box().Width(20), p.Box().Width(20), p.Expanded(b.text("Name", 11)), p.Box(b.text("Size", 11)).Width(66), p.Box(b.text("Modified", 11)).Width(100)).PaddingXY(10, 5).Gap(8).Background(cs.SurfaceSecondary)
	if m == b.remote && !b.connected {
		summary.Set("Disconnected")
	}
	countbar := p.Box(p.TextFn(func() string { return summary.Get() }).FontSize(11).Color(cs.OnSurfaceSecond)).PaddingXY(10, 5).Background(cs.SurfaceSecondary)
	return p.VBox(heading, actions, b.rule(), columns, b.rule(), p.Expanded(list), b.rule(), countbar).Background(cs.Surface)
}
