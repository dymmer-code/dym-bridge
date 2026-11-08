package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildMenu sets the application menu: the system menu bar on macOS and a menu bar at the top of the window on
// Windows and Linux (the toolkit does both from the same definition).
func (a *App) buildMenu() {
	file := fyne.NewMenu("Dym Bridge",
		fyne.NewMenuItem("Sign in with Dymmer…", a.signIn),
		fyne.NewMenuItem("Reconnect", a.Connect),
		fyne.NewMenuItem("Choose a site…", a.chooseSite),
		fyne.NewMenuItem("Start over…", a.startOver),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Settings…", a.showSettings),
	)
	a.window.SetMainMenu(fyne.NewMainMenu(file))
}

// showSettings holds what the manual way needs: the server and the .ini files. Nothing here is needed to sign in.
func (a *App) showSettings() {
	server := widget.NewEntry()
	server.SetText(a.serverURL())
	server.OnChanged = func(v string) { a.server = strings.TrimSpace(v) }

	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem("Server", server)),
		widget.NewSeparator(),
		widget.NewButtonWithIcon("Use a .ini file…", theme.FolderOpenIcon(), a.useIniFile),
		widget.NewButtonWithIcon("Create a sample .ini", theme.DocumentCreateIcon(), a.createSampleIni),
	)
	d := dialog.NewCustom("Settings", "Close", content, a.window)
	d.Resize(fyne.NewSize(440, 260))
	d.Show()
}

// chooseSite goes back to the list of sites Dymmer gave this computer, closing the current connection.
func (a *App) chooseSite() {
	if a.profiles == nil {
		return
	}
	a.tunnelMgr.Disconnect()
	a.showResources()
}
