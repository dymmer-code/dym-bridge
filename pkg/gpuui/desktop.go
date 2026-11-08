package gpuui

import (
	"github.com/dymmer-code/dym-bridge/pkg/config"
	native "github.com/dymmer-code/dym-bridge/pkg/desktop"
	"github.com/gogpu/gogpu"
	"os"
)

func (b *browser) post(fn func()) {
	select {
	case b.updates <- fn:
		b.gpu.RequestRedraw()
	case <-b.stopping:
	}
}
func (b *browser) initDesktop() {
	if b.desktopReady {
		return
	}
	b.desktopReady = true
	b.gpu.OnDragDrop(func(paths []string, x, y float64) { b.dropFiles(paths) })
	open := func() {
		if w := b.gpu.PrimaryWindow(); w != nil {
			w.Show()
		}
		b.gpu.RequestRedraw()
	}
	queue := func(fn func()) func() { return func() { b.post(fn) } }
	cleanup, tray, err := native.Install(native.Actions{Open: queue(open), Connect: queue(b.connectAction), Disconnect: queue(b.disconnect), Sites: queue(func() { open(); b.showSites() }), Quit: queue(b.quit)})
	if err != nil {
		b.status.Set("Desktop integration: " + err.Error())
	} else {
		b.desktopClose = cleanup
	}
	if tray {
		b.gpu.PrimaryWindow().SetOnClose(func() bool { b.gpu.PrimaryWindow().Hide(); return false })
	}
	b.gpu.SetCustomMenu("Connection", &gogpu.Menu{Title: "Connection", Items: []gogpu.MenuItem{
		{Title: "Sign in with Dymmer…", Action: queue(b.signIn)},
		{Title: "Connection…", Action: queue(b.showConnection)},
		{Title: "Reconnect", Action: queue(b.connectAction)},
		{Title: "Open Dym Bridge", Action: queue(open)},
		{Title: "Start over…", Action: queue(b.startOver)},
		{Title: "Quit Dym Bridge", Role: gogpu.RoleQuit, Action: queue(b.quit)},
	}})
}
func (b *browser) browseINI() {
	files, err := b.gpu.ShowOpenFileDialog(gogpu.FileDialogOptions{Title: "Open configuration"})
	if err != nil {
		b.status.Set(err.Error())
		return
	}
	if len(files) == 0 {
		return
	}
	b.configPath = files[0]
	b.showConnection()
}
func (b *browser) saveSampleINI() {
	filename, err := b.gpu.ShowSaveFileDialog(gogpu.FileDialogOptions{Title: "Save sample configuration"})
	if err == nil && filename != "" {
		err = os.WriteFile(filename, []byte(config.SampleConfigTemplate()), 0600)
	}
	if err != nil {
		b.status.Set(err.Error())
		b.rebuild()
		return
	}
	if filename != "" {
		b.configPath = filename
		b.showConnection()
	}
}

func (b *browser) quit() {
	if b.desktopClose != nil {
		b.desktopClose()
		b.desktopClose = nil
	}
	b.gpu.Quit()
}
