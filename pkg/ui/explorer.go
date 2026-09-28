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
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
	"github.com/dymmer-code/dym-bridge/pkg/tunnel"
)

// ExplorerView encapsulates the entire file manager and WordPress status UI.
type ExplorerView struct {
	window       fyne.Window
	tunnelMgr    *tunnel.Manager
	sftpClient   *sftp.Client
	currentPath  string
	rootPath     string
	mu           sync.Mutex

	// UI Elements
	statusLabel  *widget.Label
	statusDot    *widget.Icon
	openAdminBtn *widget.Button

	breadcrumb   *BreadcrumbBar
	fileList     *FileListView
	statusInfo   *widget.Label

	// Action buttons
	btnUpload    *widget.Button
	btnNewFolder *widget.Button
	btnRefresh   *widget.Button
	btnUp        *widget.Button
	btnDownload  *widget.Button
	btnRename    *widget.Button
	btnDelete    *widget.Button

	mainContainer *fyne.Container
}

// NewExplorerView builds and configures the explorer view.
func NewExplorerView(window fyne.Window, tm *tunnel.Manager, initialPath string) *ExplorerView {
	if initialPath == "" {
		initialPath = "/var/www/html/wp-content"
	}

	ev := &ExplorerView{
		window:      window,
		tunnelMgr:   tm,
		currentPath: initialPath,
		rootPath:    initialPath,
	}

	ev.buildUI()
	return ev
}

func (ev *ExplorerView) buildUI() {
	// 1. Top Header: Connection status and "Abrir WordPress Admin"
	ev.statusDot = widget.NewIcon(theme.MediaStopIcon())
	ev.statusLabel = widget.NewLabel("Conectando al servidor...")
	ev.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	ev.openAdminBtn = widget.NewButtonWithIcon("Abrir WordPress Admin", theme.NavigateNextIcon(), func() {
		ev.openWordPressAdmin()
	})
	ev.openAdminBtn.Importance = widget.HighImportance
	ev.openAdminBtn.Disable()

	topHeader := container.NewBorder(
		nil, nil,
		container.NewHBox(ev.statusDot, ev.statusLabel),
		ev.openAdminBtn,
	)

	// 2. Breadcrumb
	ev.breadcrumb = NewBreadcrumbBar(ev.currentPath, func(targetPath string) {
		ev.NavigateTo(targetPath)
	})

	// 3. Toolbar
	ev.btnUpload = widget.NewButtonWithIcon("Subir archivo...", theme.UploadIcon(), func() {
		ev.promptUploadFile()
	})
	ev.btnNewFolder = widget.NewButtonWithIcon("Nueva carpeta", theme.FolderNewIcon(), func() {
		ev.promptNewFolder()
	})
	ev.btnRefresh = widget.NewButtonWithIcon("Refrescar", theme.ViewRefreshIcon(), func() {
		ev.Refresh()
	})
	ev.btnUp = widget.NewButtonWithIcon("Subir nivel", theme.MoveUpIcon(), func() {
		parent := path.Dir(ev.currentPath)
		if parent != ev.currentPath {
			ev.NavigateTo(parent)
		}
	})

	ev.btnDownload = widget.NewButtonWithIcon("Descargar", theme.DownloadIcon(), func() {
		item := ev.fileList.SelectedItem()
		if item != nil && !item.IsDir {
			ev.promptDownloadFile(*item)
		}
	})
	ev.btnDownload.Disable()

	ev.btnRename = widget.NewButtonWithIcon("Renombrar", theme.DocumentCreateIcon(), func() {
		item := ev.fileList.SelectedItem()
		if item != nil {
			ev.promptRename(*item)
		}
	})
	ev.btnRename.Disable()

	ev.btnDelete = widget.NewButtonWithIcon("Eliminar", theme.DeleteIcon(), func() {
		item := ev.fileList.SelectedItem()
		if item != nil {
			ev.promptDelete(*item)
		}
	})
	ev.btnDelete.Disable()

	toolbar := container.NewHBox(
		ev.btnUpload,
		ev.btnNewFolder,
		ev.btnUp,
		ev.btnRefresh,
		widget.NewSeparator(),
		ev.btnDownload,
		ev.btnRename,
		ev.btnDelete,
	)

	// 4. File List View
	ev.fileList = NewFileListView(
		func(folderPath string) {
			ev.NavigateTo(folderPath)
		},
		func(item sftp.Item) {
			ev.promptDownloadFile(item)
		},
		func(item *sftp.Item) {
			if item != nil {
				ev.btnDownload.Enable()
				ev.btnRename.Enable()
				ev.btnDelete.Enable()
			} else {
				ev.btnDownload.Disable()
				ev.btnRename.Disable()
				ev.btnDelete.Disable()
			}
		},
	)

	// 5. Bottom Status Bar & WordPress Quick Jump
	ev.statusInfo = widget.NewLabel("0 elementos")
	ev.statusInfo.Importance = widget.LowImportance

	btnJumpUploads := widget.NewButton("wp-content/uploads", func() {
		ev.NavigateTo(path.Join(ev.rootPath, "uploads"))
	})
	btnJumpUploads.Importance = widget.LowImportance

	btnJumpThemes := widget.NewButton("themes", func() {
		ev.NavigateTo(path.Join(ev.rootPath, "themes"))
	})
	btnJumpThemes.Importance = widget.LowImportance

	btnJumpPlugins := widget.NewButton("plugins", func() {
		ev.NavigateTo(path.Join(ev.rootPath, "plugins"))
	})
	btnJumpPlugins.Importance = widget.LowImportance

	bottomBar := container.NewBorder(
		nil, nil,
		ev.statusInfo,
		container.NewHBox(
			widget.NewLabel("Accesos rápidos:"),
			btnJumpUploads,
			btnJumpThemes,
			btnJumpPlugins,
		),
	)

	// Master Layout
	topSection := container.NewVBox(
		topHeader,
		widget.NewSeparator(),
		toolbar,
		ev.breadcrumb.Container(),
		widget.NewSeparator(),
	)

	ev.mainContainer = container.NewBorder(
		topSection,
		bottomBar,
		nil, nil,
		ev.fileList.Widget(),
	)
}

// Container returns the Fyne canvas object for layout.
func (ev *ExplorerView) Container() fyne.CanvasObject {
	return ev.mainContainer
}

// SetSFTPClient attaches an active SFTP client and reloads directory contents.
func (ev *ExplorerView) SetSFTPClient(sc *sftp.Client) {
	ev.mu.Lock()
	ev.sftpClient = sc
	ev.mu.Unlock()

	if sc != nil {
		ev.Refresh()
	}
}

// UpdateTunnelStatus updates UI elements according to the tunnel connection state.
func (ev *ExplorerView) UpdateTunnelStatus(st tunnel.State, msg string) {
	fyne.Do(func() {
		switch st {
		case tunnel.StateConnected:
			ev.statusLabel.SetText("Túnel activo y seguro ●")
			ev.openAdminBtn.Enable()
			ev.btnUpload.Enable()
			ev.btnNewFolder.Enable()
			ev.btnRefresh.Enable()
			ev.btnUp.Enable()
		case tunnel.StateConnecting:
			ev.statusLabel.SetText("Conectando con el servidor...")
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

// NavigateTo changes the current directory and loads items.
func (ev *ExplorerView) NavigateTo(targetPath string) {
	cleanPath := path.Clean(targetPath)
	ev.currentPath = cleanPath
	ev.breadcrumb.SetPath(cleanPath)
	ev.Refresh()
}

// Refresh reloads items in the current directory.
func (ev *ExplorerView) Refresh() {
	ev.mu.Lock()
	sc := ev.sftpClient
	curPath := ev.currentPath
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	go func() {
		items, err := sc.List(curPath)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(fmt.Errorf("no se pudo listar %s: %w", curPath, err), ev.window)
				return
			}
			ev.fileList.SetItems(items)

			dirs := 0
			files := 0
			for _, it := range items {
				if it.IsDir {
					dirs++
				} else {
					files++
				}
			}
			ev.statusInfo.SetText(fmt.Sprintf("%d elementos (%d carpetas, %d archivos)", len(items), dirs, files))
		})
	}()
}

// UploadLocalFiles uploads one or more local files to the current directory (used by drag & drop).
func (ev *ExplorerView) UploadLocalFiles(localPaths []string) {
	ev.mu.Lock()
	sc := ev.sftpClient
	curDir := ev.currentPath
	ev.mu.Unlock()

	if sc == nil {
		dialog.ShowInformation("Sin conexión", "Conecte al servidor antes de subir archivos.", ev.window)
		return
	}

	go func() {
		for _, localPath := range localPaths {
			fileName := filepath.Base(localPath)
			remoteTarget := path.Join(curDir, fileName)

			progDialog := ShowTransferDialog("Subiendo archivo", fileName, ev.window)
			err := sc.Upload(localPath, remoteTarget, func(transferred, total int64) {
				progDialog.UpdateProgress(transferred, total)
			})
			progDialog.Close()

			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("error al subir %s: %w", fileName, err), ev.window)
				})
				break
			}
		}
		ev.Refresh()
	}()
}

func (ev *ExplorerView) promptUploadFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		localPath := reader.URI().Path()
		_ = reader.Close()
		ev.UploadLocalFiles([]string{localPath})
	}, ev.window)
}

func (ev *ExplorerView) promptDownloadFile(item sftp.Item) {
	ev.mu.Lock()
	sc := ev.sftpClient
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		destPath := writer.URI().Path()
		_ = writer.Close()

		go func() {
			progDialog := ShowTransferDialog("Descargando archivo", item.Name, ev.window)
			err := sc.Download(item.Path, destPath, func(transferred, total int64) {
				progDialog.UpdateProgress(transferred, total)
			})
			progDialog.Close()

			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("error al descargar %s: %w", item.Name, err), ev.window)
				})
			}
		}()
	}, ev.window)
}

func (ev *ExplorerView) promptNewFolder() {
	ev.mu.Lock()
	sc := ev.sftpClient
	curDir := ev.currentPath
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	entry := widget.NewEntry()
	entry.SetPlaceHolder("nombre_carpeta")

	dialog.ShowCustomConfirm("Nueva Carpeta", "Crear", "Cancelar", entry, func(confirmed bool) {
		if !confirmed || strings.TrimSpace(entry.Text) == "" {
			return
		}
		folderName := strings.TrimSpace(entry.Text)
		target := path.Join(curDir, folderName)

		go func() {
			if err := sc.Mkdir(target); err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("no se pudo crear la carpeta: %w", err), ev.window)
				})
			} else {
				ev.Refresh()
			}
		}()
	}, ev.window)
}

func (ev *ExplorerView) promptRename(item sftp.Item) {
	ev.mu.Lock()
	sc := ev.sftpClient
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	entry := widget.NewEntry()
	entry.SetText(item.Name)

	dialog.ShowCustomConfirm("Renombrar", "Guardar", "Cancelar", entry, func(confirmed bool) {
		if !confirmed || strings.TrimSpace(entry.Text) == "" || entry.Text == item.Name {
			return
		}
		newName := strings.TrimSpace(entry.Text)
		newPath := path.Join(path.Dir(item.Path), newName)

		go func() {
			if err := sc.Rename(item.Path, newPath); err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("error al renombrar: %w", err), ev.window)
				})
			} else {
				ev.Refresh()
			}
		}()
	}, ev.window)
}

func (ev *ExplorerView) promptDelete(item sftp.Item) {
	ev.mu.Lock()
	sc := ev.sftpClient
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	msg := fmt.Sprintf("¿Está seguro de que desea eliminar el archivo %q?", item.Name)
	if item.IsDir {
		msg = fmt.Sprintf("¿Está seguro de que desea eliminar la carpeta %q y todo su contenido?", item.Name)
	}

	dialog.ShowConfirm("Confirmar eliminación", msg, func(confirmed bool) {
		if !confirmed {
			return
		}
		go func() {
			if err := sc.Delete(item.Path, item.IsDir); err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("error al eliminar: %w", err), ev.window)
				})
			} else {
				ev.Refresh()
			}
		}()
	}, ev.window)
}

func (ev *ExplorerView) openWordPressAdmin() {
	adminURL := ev.tunnelMgr.AdminURL()
	parsedURL, err := url.Parse(adminURL)
	if err == nil {
		_ = fyne.CurrentApp().OpenURL(parsedURL)
	}
}
