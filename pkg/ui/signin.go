package ui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dymmer-code/dym-bridge/pkg/callback"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
)

// DefaultServer is the Dymmer the application signs in with unless the user says otherwise.
const DefaultServer = "https://dymmer.com"

// showResources lets the user pick one of the resources Dymmer gave this device (one profile each).
func (a *App) showResources() {
	if a.profiles == nil || len(a.profiles.Resources) == 0 {
		return
	}
	header := widget.NewLabel("Choose where to connect")
	header.TextStyle = fyne.TextStyle{Bold: true}

	items := []fyne.CanvasObject{header}
	for i, r := range a.profiles.Resources {
		index := i
		btn := widget.NewButtonWithIcon(r.Label(), theme.StorageIcon(), func() { a.useResource(index) })
		items = append(items, btn)
	}
	items = append(items, widget.NewSeparator(),
		widget.NewButtonWithIcon("Sign in with Dymmer again (refresh)", theme.ViewRefreshIcon(), a.signIn))
	a.window.SetContent(container.NewPadded(container.NewVBox(items...)))
}

func (a *App) useResource(index int) {
	cfg, err := a.profiles.Config(index)
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	a.config = cfg
	a.tunnelMgr.SetConfig(cfg)
	a.setupConnectedUI()
	a.promptMasterPassword()
}

// signIn asks for the server and the master password and runs the whole sign-in in the background.
func (a *App) signIn() {
	server := widget.NewEntry()
	server.SetText(a.serverURL())
	master := widget.NewPasswordEntry()
	master.SetPlaceHolder("Master password (protects the key of this computer)")
	info := widget.NewLabel("Your browser will open to approve this computer. The master password stays on this computer.")
	info.Wrapping = fyne.TextWrapWord

	form := dialog.NewForm("Sign in with Dymmer", "Continue", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Server", server),
		widget.NewFormItem("Master password", master),
	}, func(ok bool) {
		if !ok {
			return
		}
		if master.Text == "" {
			dialog.ShowError(fmt.Errorf("the master password cannot be empty"), a.window)
			return
		}
		a.runSignIn(server.Text, master.Text)
	}, a.window)
	form.Resize(fyne.NewSize(480, 260))
	form.Show()
}

func (a *App) serverURL() string {
	if a.profiles != nil && a.profiles.Server != "" {
		return a.profiles.Server
	}
	return DefaultServer
}

func (a *App) runSignIn(server, masterPassword string) {
	path, err := provision.DefaultPath()
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	cbPath, err := callback.DefaultPath()
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	rcv, err := callback.Listen(cbPath)
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	waiting := dialog.NewCustomWithoutButtons("Waiting for the browser",
		widget.NewLabel("Approve this computer in your browser, then come back."), a.window)
	waiting.Show()

	go func() {
		defer func() { _ = rcv.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()
		profiles, err := provision.Login(ctx, provision.LoginOptions{
			Server:         server,
			ProfilesPath:   path,
			MasterPassword: masterPassword,
			Open:           provision.OpenBrowser,
			Receiver:       rcv,
		})
		fyne.Do(func() {
			waiting.Hide()
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			a.profiles = profiles
			a.showResources()
		})
	}()
}
