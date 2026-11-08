package ui

import (
	"context"
	"errors"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dymmer-code/dym-bridge/pkg/callback"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
)

// DefaultServer is the Dymmer the application signs in with unless the user says otherwise.
const DefaultServer = "https://dymmer.com"

// bigSize is the size of the main buttons: large enough to be the obvious thing to press.
var bigSize = fyne.NewSize(320, 64)

// showWelcome is the first screen: one big button that takes the user to Dymmer to approve this computer.
func (a *App) showWelcome() {
	logo := canvas.NewImageFromResource(GetAppIconResource())
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(96, 96))
	title := widget.NewLabelWithStyle("Dym Bridge", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	btn := widget.NewButtonWithIcon("Sign in with Dymmer", theme.LoginIcon(), a.signIn)
	btn.Importance = widget.HighImportance

	a.window.Resize(fyne.NewSize(440, 360))
	a.window.SetContent(container.NewCenter(container.NewVBox(logo, title, layout.NewSpacer(), container.NewGridWrap(bigSize, btn))))
}

// showResources lets the user pick one of the resources Dymmer gave this device (one profile each).
func (a *App) showResources() {
	if a.profiles == nil || len(a.profiles.Resources) == 0 {
		a.showWelcome()
		return
	}
	items := []fyne.CanvasObject{}
	for i, r := range a.profiles.Resources {
		index := i
		btn := widget.NewButtonWithIcon(r.Label(), theme.StorageIcon(), func() { a.useResource(index) })
		btn.Importance = widget.HighImportance
		items = append(items, container.NewGridWrap(bigSize, btn))
	}
	a.window.Resize(fyne.NewSize(440, 360))
	a.window.SetContent(container.NewCenter(container.NewVBox(items...)))
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

// signIn asks only for the master password (a new one the first time) and runs the whole sign-in in the background.
// The server is the one in Settings, dymmer.com unless changed.
func (a *App) signIn() {
	path, err := provision.DefaultPath()
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	_, loadErr := provision.Load(path)
	first := errors.Is(loadErr, provision.ErrNoProfiles)

	master := widget.NewPasswordEntry()
	items := []*widget.FormItem{widget.NewFormItem("Master password", master)}
	title := "Master password"
	confirm := widget.NewPasswordEntry()
	if first {
		title = "Choose a master password"
		items = append(items, widget.NewFormItem("Repeat it", confirm))
	}

	form := dialog.NewForm(title, "Continue", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		switch {
		case master.Text == "":
			dialog.ShowError(errors.New("the master password cannot be empty"), a.window)
		case first && master.Text != confirm.Text:
			dialog.ShowError(errors.New("the two passwords are not the same"), a.window)
		default:
			a.runSignIn(a.serverURL(), master.Text)
		}
	}, a.window)
	form.Resize(fyne.NewSize(420, 200))
	form.Show()
	a.window.Canvas().Focus(master)
}

func (a *App) serverURL() string {
	if a.server != "" {
		return a.server
	}
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
