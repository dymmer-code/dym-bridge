package ui

import (
	"context"
	"errors"
	"os"
	"sync"
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

// showNoResources is the answer after a good sign-in when nobody has given this account access to a site yet:
// the sign-in worked, there is just nothing to open.
func (a *App) showNoResources() {
	icon := widget.NewIcon(theme.ConfirmIcon())
	title := widget.NewLabelWithStyle("Signed in", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	note := widget.NewLabelWithStyle("No site is shared with you yet.", fyne.TextAlignCenter, fyne.TextStyle{})
	help := widget.NewLabelWithStyle("Ask an administrator to give you access, then press Check again.",
		fyne.TextAlignCenter, fyne.TextStyle{})
	help.Wrapping = fyne.TextWrapWord
	again := widget.NewButtonWithIcon("Check again", theme.ViewRefreshIcon(), a.signIn)
	reset := widget.NewButtonWithIcon("Start over", theme.DeleteIcon(), a.startOver)

	a.window.Resize(fyne.NewSize(440, 360))
	a.window.SetContent(container.NewCenter(container.NewVBox(icon, title, note, help, layout.NewSpacer(),
		container.NewGridWrap(bigSize, again), container.NewGridWrap(bigSize, reset))))
}

// startOver forgets this computer's key and resources, after confirming: the way out when the master password is
// lost. Dymmer keeps the old device until it is removed from the keys page.
func (a *App) startOver() {
	dialog.ShowConfirm("Start over", "Forget this computer's key and sign in again?", func(ok bool) {
		if !ok {
			return
		}
		path, err := provision.DefaultPath()
		if err == nil {
			err = os.Remove(path)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			dialog.ShowError(err, a.window)
			return
		}
		a.tunnelMgr.Disconnect()
		a.config = nil
		a.profiles = nil
		a.showWelcome()
	}, a.window)
}

// showResources lets the user pick one of the resources Dymmer gave this device (one profile each).
func (a *App) showResources() {
	if a.profiles == nil {
		a.showWelcome()
		return
	}
	if len(a.profiles.Resources) == 0 {
		a.showNoResources()
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

	var d dialog.Dialog
	submit := func() {
		switch {
		case master.Text == "":
			dialog.ShowError(errors.New("the master password cannot be empty"), a.window)
		case first && master.Text != confirm.Text:
			dialog.ShowError(errors.New("the two passwords are not the same"), a.window)
		default:
			d.Hide()
			a.runSignIn(a.serverURL(), master.Text)
		}
	}
	master.OnSubmitted = func(string) { submit() }

	buttons := container.NewHBox(
		widget.NewButton("Cancel", func() { d.Hide() }),
		widget.NewButtonWithIcon("Continue", theme.ConfirmIcon(), submit),
	)
	if !first {
		forgot := widget.NewButton("Forgot it? Start over", func() {
			d.Hide()
			a.startOver()
		})
		forgot.Importance = widget.LowImportance
		buttons = container.NewHBox(forgot, layout.NewSpacer(), buttons)
	}
	body := container.NewVBox(widget.NewForm(items...), buttons)
	if first {
		warning := widget.NewLabelWithStyle("DO NOT LOSE this password. You will need it every time you connect.",
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		warning.Wrapping = fyne.TextWrapWord
		body = container.NewVBox(warning, widget.NewForm(items...), buttons)
	}
	d = dialog.NewCustomWithoutButtons(title, body, a.window)
	d.Resize(fyne.NewSize(460, 240))
	d.Show()
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

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)

	// The address the browser was sent to is kept so it can be opened again, or copied to the browser (or the
	// profile of the browser) the user really wants. It holds no secret: only the public part of the request.
	var (
		mu      sync.Mutex
		authURL string
	)
	open := func(u string) error {
		mu.Lock()
		authURL = u
		mu.Unlock()
		return provision.OpenBrowser(u)
	}
	current := func() string {
		mu.Lock()
		defer mu.Unlock()
		return authURL
	}

	var waiting dialog.Dialog
	openAgain := widget.NewButtonWithIcon("Open again", theme.ViewRefreshIcon(), func() {
		if u := current(); u != "" {
			_ = provision.OpenBrowser(u)
		}
	})
	copyLink := widget.NewButtonWithIcon("Copy link", theme.ContentCopyIcon(), func() {
		if u := current(); u != "" {
			a.fyneApp.Clipboard().SetContent(u)
		}
	})
	stop := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		cancel()
		waiting.Hide()
	})
	waiting = dialog.NewCustomWithoutButtons("Waiting for the browser",
		container.NewVBox(
			widget.NewLabel("Approve this computer in your browser, then come back."),
			container.NewHBox(openAgain, copyLink, layout.NewSpacer(), stop),
		), a.window)
	waiting.Show()

	go func() {
		defer func() { _ = rcv.Close() }()
		defer cancel()
		profiles, err := provision.Login(ctx, provision.LoginOptions{
			Server:         server,
			ProfilesPath:   path,
			MasterPassword: masterPassword,
			Open:           open,
			Receiver:       rcv,
		})
		if errors.Is(ctx.Err(), context.Canceled) {
			return // the user pressed Cancel: nothing to report
		}
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
