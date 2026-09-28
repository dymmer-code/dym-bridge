package ui

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dymmer-code/dym-bridge/pkg/local"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/dymmer-code/dym-bridge/pkg/tunnel"
)

// ExplorerView implements a dual-pane file manager (Local on Left, Remote on Right).
type ExplorerView struct {
	window     fyne.Window
	tunnelMgr  *tunnel.Manager
	sftpClient *sftp.Client
	mu         sync.Mutex

	// Top status & global actions
	statusDot    *widget.Icon
	statusLabel  *widget.Label
	openAdminBtn *widget.Button
	btnUpload    *widget.Button
	btnDownload  *widget.Button

	// --- Left Pane (Local) ---
	localPath            string
	localBreadcrumb      *BreadcrumbBar
	localList            *FileListView
	localInfo            *widget.Label
	localSelectionDetail *widget.Label
	localUpBtn           *widget.Button
	localRefreshBtn      *widget.Button
	localNewFolder       *widget.Button
	localDeleteBtn       *widget.Button

	// --- Right Pane (Remote) ---
	remotePath            string
	remoteRoot            string
	remoteBreadcrumb      *BreadcrumbBar
	remoteList            *FileListView
	remoteInfo            *widget.Label
	remoteSelectionDetail *widget.Label
	remoteUpBtn           *widget.Button
	remoteRefreshBtn      *widget.Button
	remoteNewFolder       *widget.Button
	remoteDeleteBtn       *widget.Button

	splitContainer *container.Split
	mainContainer  *fyne.Container
}

// NewExplorerView creates and initializes the dual-pane explorer.
func NewExplorerView(window fyne.Window, tm *tunnel.Manager, initialRemotePath string) *ExplorerView {
	if initialRemotePath == "" {
		initialRemotePath = "/var/www/html/wp-content"
	}
	initialLocalPath := local.DefaultLocalDir()

	ev := &ExplorerView{
		window:      window,
		tunnelMgr:   tm,
		localPath:   initialLocalPath,
		remotePath:  initialRemotePath,
		remoteRoot:  initialRemotePath,
	}

	ev.buildUI()
	ev.RefreshLocal()
	return ev
}

func (ev *ExplorerView) buildUI() {
	// 1. Top Global Bar
	ev.statusDot = widget.NewIcon(theme.MediaStopIcon())
	ev.statusLabel = widget.NewLabel("Conectando con el servidor...")
	ev.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	ev.btnUpload = widget.NewButtonWithIcon("Subir a WordPress ➔", theme.UploadIcon(), func() {
		ev.TransferSelectedLocalToRemote()
	})
	ev.btnUpload.Importance = widget.HighImportance
	ev.btnUpload.Disable()

	ev.btnDownload = widget.NewButtonWithIcon("⬅ Descargar a Local", theme.DownloadIcon(), func() {
		ev.TransferSelectedRemoteToLocal()
	})
	ev.btnDownload.Importance = widget.MediumImportance
	ev.btnDownload.Disable()

	ev.openAdminBtn = widget.NewButtonWithIcon("Abrir WordPress Admin", theme.NavigateNextIcon(), func() {
		ev.openWordPressAdmin()
	})
	ev.openAdminBtn.Importance = widget.HighImportance
	ev.openAdminBtn.Disable()

	transferControls := container.NewHBox(ev.btnUpload, ev.btnDownload)

	topBar := container.NewBorder(
		nil, nil,
		container.NewHBox(ev.statusDot, ev.statusLabel),
		container.NewHBox(transferControls, widget.NewSeparator(), ev.openAdminBtn),
	)

	// 2. Build Left Pane (Local)
	leftPane := ev.buildLocalPane()

	// 3. Build Right Pane (Remote)
	rightPane := ev.buildRemotePane()

	// 4. Split Container (50% local, 50% remote)
	ev.splitContainer = container.NewHSplit(leftPane, rightPane)
	ev.splitContainer.SetOffset(0.5)

	// 5. Master Layout
	ev.mainContainer = container.NewBorder(
		container.NewVBox(topBar, widget.NewSeparator()),
		nil,
		nil, nil,
		ev.splitContainer,
	)
}

func (ev *ExplorerView) buildLocalPane() fyne.CanvasObject {
	title := widget.NewLabel("💻 Mi Equipo (Local)")
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Quick jump buttons for local
	commonDirs := local.CommonDirectories()
	var jumpButtons []fyne.CanvasObject
	for label, dirPath := range commonDirs {
		p := dirPath
		btn := widget.NewButton(label, func() {
			ev.NavigateLocal(p)
		})
		btn.Importance = widget.LowImportance
		jumpButtons = append(jumpButtons, btn)
	}
	jumpRow := container.NewHScroll(container.NewHBox(jumpButtons...))

	// Breadcrumb
	ev.localBreadcrumb = NewBreadcrumbBar(ev.window, ev.localPath, func(targetPath string) {
		ev.NavigateLocal(targetPath)
	})

	// Toolbar
	ev.localUpBtn = widget.NewButtonWithIcon("Arriba", theme.MoveUpIcon(), func() {
		parent := filepath.Dir(ev.localPath)
		if parent != ev.localPath {
			ev.NavigateLocal(parent)
		}
	})
	ev.localRefreshBtn = widget.NewButtonWithIcon("Refrescar", theme.ViewRefreshIcon(), func() {
		ev.RefreshLocal()
	})
	ev.localNewFolder = widget.NewButtonWithIcon("Carpeta", theme.FolderNewIcon(), func() {
		ev.promptNewLocalFolder()
	})
	ev.localDeleteBtn = widget.NewButtonWithIcon("Borrar", theme.DeleteIcon(), func() {
		item := ev.localList.SelectedItem()
		if item != nil {
			ev.promptDeleteLocal(*item)
		}
	})
	ev.localDeleteBtn.Disable()

	localToolbar := container.NewHBox(
		ev.localUpBtn,
		ev.localRefreshBtn,
		ev.localNewFolder,
		ev.localDeleteBtn,
	)

	ev.localSelectionDetail = widget.NewLabel("")
	ev.localSelectionDetail.Truncation = fyne.TextTruncateEllipsis
	ev.localSelectionDetail.Importance = widget.LowImportance

	// File List
	ev.localList = NewFileListView(
		func(folderPath string) {
			ev.NavigateLocal(folderPath)
		},
		func(item sftp.Item) {
			// Double click local file -> upload to current remote directory!
			ev.uploadLocalItem(item)
		},
		func(item *sftp.Item) {
			if item != nil {
				ev.btnUpload.Enable()
				ev.localDeleteBtn.Enable()
				if item.IsDir {
					ev.localSelectionDetail.SetText(fmt.Sprintf("Seleccionado: 📁 %s", item.Name))
				} else {
					ev.localSelectionDetail.SetText(fmt.Sprintf("Seleccionado: 📄 %s (%s)", item.Name, sftp.FormatFileSize(item.Size)))
				}
			} else {
				ev.btnUpload.Disable()
				ev.localDeleteBtn.Disable()
				ev.localSelectionDetail.SetText("")
			}
		},
	)

	ev.localInfo = widget.NewLabel("0 elementos")
	ev.localInfo.Importance = widget.LowImportance

	header := container.NewVBox(
		container.NewBorder(nil, nil, title, jumpRow),
		ev.localBreadcrumb.Container(),
		localToolbar,
		widget.NewSeparator(),
	)

	footer := container.NewVBox(
		ev.localSelectionDetail,
		ev.localInfo,
	)

	return container.NewBorder(
		header,
		footer,
		nil, nil,
		ev.localList.Widget(),
	)
}

func (ev *ExplorerView) buildRemotePane() fyne.CanvasObject {
	title := widget.NewLabel("☁️ Servidor WordPress (Remoto)")
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Quick jump buttons for WordPress
	btnUploads := widget.NewButton("uploads", func() {
		ev.NavigateRemote(path.Join(ev.remoteRoot, "uploads"))
	})
	btnUploads.Importance = widget.LowImportance

	btnThemes := widget.NewButton("themes", func() {
		ev.NavigateRemote(path.Join(ev.remoteRoot, "themes"))
	})
	btnThemes.Importance = widget.LowImportance

	btnPlugins := widget.NewButton("plugins", func() {
		ev.NavigateRemote(path.Join(ev.remoteRoot, "plugins"))
	})
	btnPlugins.Importance = widget.LowImportance

	jumpRow := container.NewHBox(btnUploads, btnThemes, btnPlugins)

	// Breadcrumb
	ev.remoteBreadcrumb = NewBreadcrumbBar(ev.window, ev.remotePath, func(targetPath string) {
		ev.NavigateRemote(targetPath)
	})

	// Toolbar
	ev.remoteUpBtn = widget.NewButtonWithIcon("Arriba", theme.MoveUpIcon(), func() {
		parent := path.Dir(ev.remotePath)
		if parent != ev.remotePath {
			ev.NavigateRemote(parent)
		}
	})
	ev.remoteRefreshBtn = widget.NewButtonWithIcon("Refrescar", theme.ViewRefreshIcon(), func() {
		ev.RefreshRemote()
	})
	ev.remoteNewFolder = widget.NewButtonWithIcon("Carpeta", theme.FolderNewIcon(), func() {
		ev.promptNewRemoteFolder()
	})
	ev.remoteDeleteBtn = widget.NewButtonWithIcon("Borrar", theme.DeleteIcon(), func() {
		item := ev.remoteList.SelectedItem()
		if item != nil {
			ev.promptDeleteRemote(*item)
		}
	})
	ev.remoteDeleteBtn.Disable()

	remoteToolbar := container.NewHBox(
		ev.remoteUpBtn,
		ev.remoteRefreshBtn,
		ev.remoteNewFolder,
		ev.remoteDeleteBtn,
	)

	ev.remoteSelectionDetail = widget.NewLabel("")
	ev.remoteSelectionDetail.Truncation = fyne.TextTruncateEllipsis
	ev.remoteSelectionDetail.Importance = widget.LowImportance

	// File List
	ev.remoteList = NewFileListView(
		func(folderPath string) {
			ev.NavigateRemote(folderPath)
		},
		func(item sftp.Item) {
			// Double click remote file -> download to current local directory!
			ev.downloadRemoteItem(item)
		},
		func(item *sftp.Item) {
			if item != nil {
				ev.btnDownload.Enable()
				ev.remoteDeleteBtn.Enable()
				if item.IsDir {
					ev.remoteSelectionDetail.SetText(fmt.Sprintf("Seleccionado: 📁 %s", item.Name))
				} else {
					ev.remoteSelectionDetail.SetText(fmt.Sprintf("Seleccionado: 📄 %s (%s)", item.Name, sftp.FormatFileSize(item.Size)))
				}
			} else {
				ev.btnDownload.Disable()
				ev.remoteDeleteBtn.Disable()
				ev.remoteSelectionDetail.SetText("")
			}
		},
	)

	ev.remoteInfo = widget.NewLabel("0 elementos")
	ev.remoteInfo.Importance = widget.LowImportance

	header := container.NewVBox(
		container.NewBorder(nil, nil, title, jumpRow),
		ev.remoteBreadcrumb.Container(),
		remoteToolbar,
		widget.NewSeparator(),
	)

	footer := container.NewVBox(
		ev.remoteSelectionDetail,
		ev.remoteInfo,
	)

	return container.NewBorder(
		header,
		footer,
		nil, nil,
		ev.remoteList.Widget(),
	)
}

// Container returns the root Fyne canvas object.
func (ev *ExplorerView) Container() fyne.CanvasObject {
	return ev.mainContainer
}

// SetSFTPClient attaches the remote SFTP client and reloads the remote view.
func (ev *ExplorerView) SetSFTPClient(sc *sftp.Client) {
	ev.mu.Lock()
	ev.sftpClient = sc
	ev.mu.Unlock()

	if sc != nil {
		ev.RefreshRemote()
	}
}

// UpdateTunnelStatus updates UI elements according to the tunnel connection state.
func (ev *ExplorerView) UpdateTunnelStatus(st tunnel.State, msg string) {
	fyne.Do(func() {
		switch st {
		case tunnel.StateConnected:
			ev.statusLabel.SetText(fmt.Sprintf("Túnel activo ● (: %d)", ev.tunnelMgr.Config().WordPress.LocalPort))
			ev.openAdminBtn.Enable()
			ev.remoteUpBtn.Enable()
			ev.remoteRefreshBtn.Enable()
			ev.remoteNewFolder.Enable()
		case tunnel.StateConnecting:
			ev.statusLabel.SetText("Conectando con el servidor... ◌")
			ev.openAdminBtn.Disable()
		case tunnel.StateError:
			errTxt := "Error de conexión"
			if msg != "" {
				errTxt = fmt.Sprintf("Error: %s", msg)
			}
			ev.statusLabel.SetText(errTxt)
			ev.openAdminBtn.Disable()
		default:
			ev.statusLabel.SetText("Desconectado ○")
			ev.openAdminBtn.Disable()
		}
	})
}

// --- Local Navigation & Operations ---

func (ev *ExplorerView) NavigateLocal(targetPath string) {
	cleanPath := filepath.Clean(targetPath)
	ev.localPath = cleanPath
	ev.localBreadcrumb.SetPath(cleanPath)
	ev.RefreshLocal()
}

func (ev *ExplorerView) RefreshLocal() {
	curPath := ev.localPath
	go func() {
		items, err := local.ListDirectory(curPath)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(fmt.Errorf("no se pudo listar localmente: %w", err), ev.window)
				return
			}
			ev.localList.SetItems(items)

			dirs, files := countItems(items)
			ev.localInfo.SetText(fmt.Sprintf("%d elementos (%d carpetas, %d archivos)", len(items), dirs, files))
		})
	}()
}

func (ev *ExplorerView) promptNewLocalFolder() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("nueva_carpeta")

	dialog.ShowCustomConfirm("Nueva Carpeta Local", "Crear", "Cancelar", entry, func(confirmed bool) {
		if !confirmed || strings.TrimSpace(entry.Text) == "" {
			return
		}
		target := filepath.Join(ev.localPath, strings.TrimSpace(entry.Text))
		if err := local.Mkdir(target); err != nil {
			dialog.ShowError(err, ev.window)
		} else {
			ev.RefreshLocal()
		}
	}, ev.window)
}

func (ev *ExplorerView) promptDeleteLocal(item sftp.Item) {
	msg := fmt.Sprintf("¿Eliminar archivo local %q?", item.Name)
	if item.IsDir {
		msg = fmt.Sprintf("¿Eliminar carpeta local %q y su contenido?", item.Name)
	}
	dialog.ShowConfirm("Confirmar eliminación local", msg, func(confirmed bool) {
		if !confirmed {
			return
		}
		if err := local.Delete(item.Path, item.IsDir); err != nil {
			dialog.ShowError(err, ev.window)
		} else {
			ev.RefreshLocal()
		}
	}, ev.window)
}

// --- Remote Navigation & Operations ---

func (ev *ExplorerView) NavigateRemote(targetPath string) {
	cleanPath := path.Clean(targetPath)
	ev.remotePath = cleanPath
	ev.remoteBreadcrumb.SetPath(cleanPath)
	ev.RefreshRemote()
}

func (ev *ExplorerView) RefreshRemote() {
	ev.mu.Lock()
	sc := ev.sftpClient
	curPath := ev.remotePath
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	go func() {
		items, err := sc.List(curPath)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(fmt.Errorf("no se pudo listar remoto %s: %w", curPath, err), ev.window)
				return
			}
			ev.remoteList.SetItems(items)

			dirs, files := countItems(items)
			ev.remoteInfo.SetText(fmt.Sprintf("%d elementos (%d carpetas, %d archivos)", len(items), dirs, files))
		})
	}()
}

func (ev *ExplorerView) promptNewRemoteFolder() {
	ev.mu.Lock()
	sc := ev.sftpClient
	curDir := ev.remotePath
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	entry := widget.NewEntry()
	entry.SetPlaceHolder("nueva_carpeta")

	dialog.ShowCustomConfirm("Nueva Carpeta Remota", "Crear", "Cancelar", entry, func(confirmed bool) {
		if !confirmed || strings.TrimSpace(entry.Text) == "" {
			return
		}
		target := path.Join(curDir, strings.TrimSpace(entry.Text))
		go func() {
			if err := sc.Mkdir(target); err != nil {
				fyne.Do(func() { dialog.ShowError(err, ev.window) })
			} else {
				ev.RefreshRemote()
			}
		}()
	}, ev.window)
}

func (ev *ExplorerView) promptDeleteRemote(item sftp.Item) {
	ev.mu.Lock()
	sc := ev.sftpClient
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	msg := fmt.Sprintf("¿Eliminar archivo remoto %q?", item.Name)
	if item.IsDir {
		msg = fmt.Sprintf("¿Eliminar carpeta remota %q y su contenido?", item.Name)
	}

	dialog.ShowConfirm("Confirmar eliminación remota", msg, func(confirmed bool) {
		if !confirmed {
			return
		}
		go func() {
			if err := sc.Delete(item.Path, item.IsDir); err != nil {
				fyne.Do(func() { dialog.ShowError(err, ev.window) })
			} else {
				ev.RefreshRemote()
			}
		}()
	}, ev.window)
}

// --- Transfers (Local <-> Remote) ---

// TransferSelectedLocalToRemote uploads the selected item in the local pane to the remote directory.
func (ev *ExplorerView) TransferSelectedLocalToRemote() {
	item := ev.localList.SelectedItem()
	if item == nil {
		return
	}
	ev.uploadLocalItem(*item)
}

func (ev *ExplorerView) uploadLocalItem(item sftp.Item) {
	ev.mu.Lock()
	sc := ev.sftpClient
	targetRemoteDir := ev.remotePath
	ev.mu.Unlock()

	if sc == nil {
		dialog.ShowInformation("Sin conexión", "Conecte al servidor para subir archivos.", ev.window)
		return
	}

	if item.IsDir {
		dialog.ShowInformation("Aviso", "Seleccione un archivo individual para subir.", ev.window)
		return
	}

	remoteTarget := path.Join(targetRemoteDir, item.Name)

	go func() {
		prog := ShowTransferDialog("Subiendo archivo", item.Name, ev.window)
		err := sc.Upload(item.Path, remoteTarget, func(transferred, total int64) {
			prog.UpdateProgress(transferred, total)
		})
		prog.Close()

		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, ev.window) })
		} else {
			ev.RefreshRemote()
		}
	}()
}

// TransferSelectedRemoteToLocal downloads the selected item in the remote pane to the local directory.
func (ev *ExplorerView) TransferSelectedRemoteToLocal() {
	item := ev.remoteList.SelectedItem()
	if item == nil {
		return
	}
	ev.downloadRemoteItem(*item)
}

func (ev *ExplorerView) downloadRemoteItem(item sftp.Item) {
	ev.mu.Lock()
	sc := ev.sftpClient
	targetLocalDir := ev.localPath
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	if item.IsDir {
		dialog.ShowInformation("Aviso", "Seleccione un archivo individual para descargar.", ev.window)
		return
	}

	localTarget := filepath.Join(targetLocalDir, item.Name)

	go func() {
		prog := ShowTransferDialog("Descargando archivo", item.Name, ev.window)
		err := sc.Download(item.Path, localTarget, func(transferred, total int64) {
			prog.UpdateProgress(transferred, total)
		})
		prog.Close()

		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, ev.window) })
		} else {
			ev.RefreshLocal()
		}
	}()
}

// UploadDroppedFiles uploads files dragged from the OS to the remote directory.
func (ev *ExplorerView) UploadDroppedFiles(localPaths []string) {
	ev.mu.Lock()
	sc := ev.sftpClient
	targetRemoteDir := ev.remotePath
	ev.mu.Unlock()

	if sc == nil {
		dialog.ShowInformation("Sin conexión", "Conecte al servidor antes de soltar archivos.", ev.window)
		return
	}

	go func() {
		for _, localPath := range localPaths {
			fileName := filepath.Base(localPath)
			remoteTarget := path.Join(targetRemoteDir, fileName)

			prog := ShowTransferDialog("Subiendo archivo", fileName, ev.window)
			err := sc.Upload(localPath, remoteTarget, func(transferred, total int64) {
				prog.UpdateProgress(transferred, total)
			})
			prog.Close()

			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, ev.window) })
				break
			}
		}
		ev.RefreshRemote()
	}()
}

func (ev *ExplorerView) openWordPressAdmin() {
	adminURL := ev.tunnelMgr.AdminURL()
	parsedURL, err := url.Parse(adminURL)
	if err == nil {
		_ = fyne.CurrentApp().OpenURL(parsedURL)
	}
}

func countItems(items []sftp.Item) (dirs, files int) {
	for _, it := range items {
		if it.IsDir {
			dirs++
		} else {
			files++
		}
	}
	return dirs, files
}
