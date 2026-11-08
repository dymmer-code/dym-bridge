//go:build windows

package desktop

import (
	_ "embed"
	"fyne.io/systray"
	"runtime"
)

//go:embed tray.ico
var trayIcon []byte

func Install(a Actions) (func(), bool, error) {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTooltip("Dym Bridge — secure files and sites")
			entries := []struct {
				label string
				fn    func()
			}{{"Open Dym Bridge", a.Open}, {"Sites…", a.Sites}, {"Connect / Reconnect", a.Connect}, {"Disconnect", a.Disconnect}, {"Quit Dym Bridge", a.Quit}}
			for _, entry := range entries {
				item := systray.AddMenuItem(entry.label, entry.label)
				go func() {
					for range item.ClickedCh {
						entry.fn()
					}
				}()
			}
		}, func() {})
	}()
	return systray.Quit, true, nil
}
