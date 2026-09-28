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
}

// NewApp initializes the dym-bridge desktop application.
func NewApp(cfg *config.Config) *App {
	a := app.NewWithID("com.dymmer.bridge")
	a.Settings().SetTheme(NewDymBridgeTheme())

	iconRes := GetAppIconResource()
	a.SetIcon(iconRes)

	w := a.NewWindow("dym-bridge — WordPress Manager")
	w.SetIcon(iconRes)
	w.Resize(fyne.NewSize(1024, 680))

	tm := tunnel.NewManager(cfg)

	dymApp := &App{
		fyneApp:   a,
		window:    w,
		config:    cfg,
		tunnelMgr: tm,
	}

	if cfg != nil {
		dymApp.setupConnectedUI()
	} else {
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
	header := widget.NewLabel("Configuración dym-bridge.ini no encontrada")
	header.TextStyle = fyne.TextStyle{Bold: true}

	var checkedPaths strings.Builder
	checkedPaths.WriteString("No se encontró el archivo de configuración en ninguna de las siguientes rutas:\n\n")
	for _, p := range config.CandidatePaths() {
		checkedPaths.WriteString(fmt.Sprintf(" • %s\n", p))
	}
	checkedPaths.WriteString("\nPuede crear una plantilla de ejemplo en la carpeta actual o seleccionar un archivo existente:")

	desc := widget.NewLabel(checkedPaths.String())
	desc.Wrapping = fyne.TextWrapWord

	btnCreateSample := widget.NewButtonWithIcon("Crear plantilla dym-bridge.ini", theme.DocumentCreateIcon(), func() {
		samplePath := config.DefaultConfigFileName
		if err := os.WriteFile(samplePath, []byte(config.SampleConfigTemplate()), 0644); err != nil {
			dialog.ShowError(fmt.Errorf("error al crear %s: %w", samplePath, err), a.window)
			return
		}
		dialog.ShowInformation("Plantilla Creada",
			fmt.Sprintf("Se ha generado el archivo %s.\nEdítelo con sus datos y vuelva a iniciar dym-bridge.", samplePath),
			a.window)
	})
	btnCreateSample.Importance = widget.HighImportance

	btnSelectFile := widget.NewButtonWithIcon("Seleccionar archivo .ini existente...", theme.FolderOpenIcon(), func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			filePath := reader.URI().Path()
			_ = reader.Close()

			cfg, err := config.Load(filePath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("archivo de configuración inválido: %w", err), a.window)
				return
			}

			a.config = cfg
			a.tunnelMgr.SetConfig(cfg)
			a.setupConnectedUI()
			a.Connect()
		}, a.window)
	})

	actions := container.NewHBox(btnCreateSample, btnSelectFile)
	content := container.NewVBox(
		header,
		desc,
		widget.NewSeparator(),
		actions,
	)

	a.window.SetContent(container.NewPadded(content))
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
		a.Connect()
	}
	a.fyneApp.Run()
}
