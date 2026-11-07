package ui

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dymmer-code/dym-bridge/pkg/config"
	"github.com/dymmer-code/dym-bridge/pkg/provision"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/dymmer-code/dym-bridge/pkg/tunnel"
)

// App manages the lifecycle, window and background services of dym-bridge.
type App struct {
	fyneApp    fyne.App
	window     fyne.Window
	config     *config.Config
	tunnelMgr  *tunnel.Manager
	trayMgr    *TrayManager
	explorer   *ExplorerView
	sftpClient *sftp.Client
	profiles   *provision.Profiles
}

// NewApp initializes the dym-bridge desktop application.
func NewApp(cfg *config.Config, profiles *provision.Profiles) *App {
	a := app.NewWithID("com.dymmer.bridge")
	a.Settings().SetTheme(NewDymBridgeTheme())

	iconRes := GetAppIconResource()
	a.SetIcon(iconRes)

	w := a.NewWindow("Dym Bridge")
	w.SetIcon(iconRes)
	w.Resize(fyne.NewSize(680, 480))

	tm := tunnel.NewManager(cfg)

	dymApp := &App{
		fyneApp:   a,
		window:    w,
		config:    cfg,
		tunnelMgr: tm,
		profiles:  profiles,
	}

	switch {
	case cfg != nil:
		dymApp.setupConnectedUI()
	case profiles != nil && len(profiles.Resources) > 0:
		dymApp.showResources()
	default:
		dymApp.setupMissingConfigUI()
	}

	// Stay resident in tray when window close button (X) is clicked
	w.SetCloseIntercept(func() {
		w.Hide()
	})

	return dymApp
}

func (a *App) setupConnectedUI() {
	a.explorer = NewExplorerView(a.window, a.tunnelMgr, a.config.SFTP.RemotePath)

	// Setup drag & drop upload support
	a.window.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		var localPaths []string
		for _, u := range uris {
			p := u.Path()
			if p != "" {
				localPaths = append(localPaths, p)
			}
		}
		if len(localPaths) > 0 {
			a.explorer.UploadDroppedFiles(localPaths)
		}
	})

	// Configure system tray
	a.trayMgr = NewTrayManager(
		a.fyneApp,
		a.window,
		a.tunnelMgr,
		func() {
			a.openAdminInBrowser()
		},
		func() {
			a.Connect()
		},
	)

	// Register tunnel state listener
	a.tunnelMgr.AddListener(func(st tunnel.State, msg string) {
		a.explorer.UpdateTunnelStatus(st, msg)
		if a.trayMgr != nil {
			a.trayMgr.Refresh()
		}

		if st == tunnel.StateConnected {
			sshCli := a.tunnelMgr.SSHClient()
			if sshCli != nil {
				sc, err := sftp.NewClient(sshCli)
				if err == nil {
					a.sftpClient = sc
					a.explorer.SetSFTPClient(sc)
				}
			}
		} else if st == tunnel.StateDisconnected || st == tunnel.StateError {
			if a.sftpClient != nil {
				_ = a.sftpClient.Close()
				a.sftpClient = nil
			}
			a.explorer.SetSFTPClient(nil)
		}
	})

	a.window.SetContent(a.explorer.Container())
}

func (a *App) setupMissingConfigUI() {
	header := widget.NewLabel("Configuration dym-bridge.ini Not Found")
	header.TextStyle = fyne.TextStyle{Bold: true}

	var checkedPaths strings.Builder
	checkedPaths.WriteString("Configuration file was not found in any of the following paths:\n\n")
	for _, p := range config.CandidatePaths() {
		checkedPaths.WriteString(fmt.Sprintf(" • %s\n", p))
	}
	checkedPaths.WriteString("\nYou can generate a sample template in the current directory or select an existing file:")

	desc := widget.NewLabel(checkedPaths.String())
	desc.Wrapping = fyne.TextWrapWord

	btnCreateSample := widget.NewButtonWithIcon("Create sample dym-bridge.ini", theme.DocumentCreateIcon(), func() {
		samplePath := config.DefaultConfigFileName
		if err := os.WriteFile(samplePath, []byte(config.SampleConfigTemplate()), 0644); err != nil {
			dialog.ShowError(fmt.Errorf("error creating %s: %w", samplePath, err), a.window)
			return
		}
		dialog.ShowInformation("Template Created",
			fmt.Sprintf("The sample configuration file %s has been created.\nEdit it with your server details and restart dym-bridge.", samplePath),
			a.window)
	})
	btnCreateSample.Importance = widget.HighImportance

	btnSelectFile := widget.NewButtonWithIcon("Select existing .ini file...", theme.FolderOpenIcon(), func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			filePath := reader.URI().Path()
			_ = reader.Close()

			cfg, err := config.Load(filePath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("invalid configuration file: %w", err), a.window)
				return
			}

			a.config = cfg
			a.tunnelMgr.SetConfig(cfg)
			a.setupConnectedUI()
			if a.config.HasEncryptedCredentials() {
				a.promptMasterPassword()
			} else {
				a.Connect()
			}
		}, a.window)
	})

	btnSignIn := widget.NewButtonWithIcon("Sign in with Dymmer", theme.LoginIcon(), a.signIn)
	btnSignIn.Importance = widget.HighImportance

	actions := container.NewHBox(btnSignIn, btnCreateSample, btnSelectFile)
	content := container.NewVBox(
		header,
		desc,
		widget.NewSeparator(),
		actions,
	)

	a.window.SetContent(container.NewPadded(content))
}

func (a *App) promptMasterPassword() {
	pwdEntry := widget.NewPasswordEntry()
	pwdEntry.SetPlaceHolder("Clave maestra...")

	errLabel := widget.NewLabel("")
	errLabel.Importance = widget.DangerImportance
	errLabel.Wrapping = fyne.TextWrapWord

	descLabel := widget.NewLabel("El archivo de configuración contiene credenciales cifradas (contraseña o clave SSH).\nPor favor, introduzca la clave maestra para desbloquearlas:")
	descLabel.Wrapping = fyne.TextWrapWord

	var d dialog.Dialog

	btnUnlock := widget.NewButtonWithIcon("Desbloquear", theme.ConfirmIcon(), func() {
		pwd := pwdEntry.Text
		if pwd == "" {
			errLabel.SetText("La clave maestra no puede estar vacía.")
			return
		}

		if err := a.config.Unlock(pwd); err != nil {
			errLabel.SetText("❌ Clave maestra incorrecta o datos cifrados corruptos.")
			pwdEntry.SetText("")
			return
		}

		d.Hide()
		a.Connect()
	})
	btnUnlock.Importance = widget.HighImportance

	btnExit := widget.NewButtonWithIcon("Salir", theme.CancelIcon(), func() {
		d.Hide()
		a.fyneApp.Quit()
	})

	pwdEntry.OnSubmitted = func(_ string) {
		btnUnlock.Tapped(&fyne.PointEvent{})
	}

	content := container.NewVBox(
		descLabel,
		pwdEntry,
		errLabel,
		widget.NewSeparator(),
		container.NewHBox(btnExit, btnUnlock),
	)

	d = dialog.NewCustomWithoutButtons("Desbloquear Credenciales", content, a.window)
	d.Resize(fyne.NewSize(460, 200))
	d.Show()
	a.window.Canvas().Focus(pwdEntry)
}

// Connect starts the background SSH tunnel and connects the SFTP client.
func (a *App) Connect() {
	if a.config == nil {
		return
	}
	go func() {
		_ = a.tunnelMgr.Connect()
	}()
}

func (a *App) openAdminInBrowser() {
	// A provisioned resource has no web tunnel (Dymmer gives no Host header): nothing to open.
	if a.config == nil || a.config.WordPress.HostHeader == "" {
		return
	}
	adminURL := a.tunnelMgr.AdminURL()
	parsedURL, err := url.Parse(adminURL)
	if err == nil {
		_ = a.fyneApp.OpenURL(parsedURL)
	}
}

// Run shows the window, initiates connection and runs the Fyne event loop.
func (a *App) Run() {
	a.window.Show()
	if a.config != nil {
		if a.config.HasEncryptedCredentials() {
			a.promptMasterPassword()
		} else {
			a.Connect()
		}
	}
	a.fyneApp.Run()
}
