//go:build fyne

package ui

import (
	"fmt"
	"net/url"
	"os"
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

	// Top actions & global status
	openAdminBtn    *widget.Button
	btnUpload       *widget.Button
	btnDownload     *widget.Button
	globalStatusBar *widget.Label
	reconnectBtn    *widget.Button
	// OnReconnect tries the connection again; it is offered when the connection failed or ended.
	OnReconnect func()

	// --- Left Pane (Local) ---
	localPath         string
	localBreadcrumb   *BreadcrumbBar
	localList         *FileListView
	localInfo         *widget.Label
	localUpBtn        *widget.Button
	localRefreshBtn   *widget.Button
	localSelectAllBtn *widget.Button
	localNewFolder    *widget.Button
	localDeleteBtn    *widget.Button
	localItemsCount   int

	// --- Right Pane (Remote) ---
	remotePath         string
	remoteRoot         string
	remoteBreadcrumb   *BreadcrumbBar
	remoteList         *FileListView
	remoteInfo         *widget.Label
	remoteUpBtn        *widget.Button
	remoteRefreshBtn   *widget.Button
	remoteSelectAllBtn *widget.Button
	remoteNewFolder    *widget.Button
	remoteDeleteBtn    *widget.Button
	remoteTitleLabel   *widget.Label
	remoteItemsCount   int

	splitContainer *container.Split
	mainContainer  *fyne.Container
}

// NewExplorerView creates and initializes the dual-pane explorer.
func NewExplorerView(window fyne.Window, tm *tunnel.Manager, initialRemotePath string) *ExplorerView {
	if initialRemotePath == "" {
		initialRemotePath = "/var/www/wordpress"
	}
	initialLocalPath := local.DefaultLocalDir()

	ev := &ExplorerView{
		window:     window,
		tunnelMgr:  tm,
		localPath:  initialLocalPath,
		remotePath: initialRemotePath,
		remoteRoot: initialRemotePath,
	}

	ev.buildUI()
	ev.RefreshLocal()
	return ev
}

func (ev *ExplorerView) buildUI() {
	// 1. Top Global Bar
	ev.btnUpload = widget.NewButtonWithIcon("Upload", theme.UploadIcon(), func() {
		ev.TransferSelectedLocalToRemote()
	})
	ev.btnUpload.Importance = widget.HighImportance
	ev.btnUpload.Disable()

	ev.btnDownload = widget.NewButtonWithIcon("Download", theme.DownloadIcon(), func() {
		ev.TransferSelectedRemoteToLocal()
	})
	ev.btnDownload.Importance = widget.MediumImportance
	ev.btnDownload.Disable()

	ev.openAdminBtn = widget.NewButtonWithIcon("Web Admin", theme.NavigateNextIcon(), func() {
		ev.openWordPressAdmin()
	})
	ev.openAdminBtn.Importance = widget.HighImportance
	ev.openAdminBtn.Disable()

	btnEncrypt := widget.NewButtonWithIcon("Cifrar...", theme.LoginIcon(), func() {
		ShowEncryptDialog(ev.window)
	})
	btnEncrypt.Importance = widget.LowImportance

	transferControls := container.NewHBox(ev.btnUpload, ev.btnDownload)

	topBar := container.NewBorder(
		nil, nil,
		btnEncrypt,
		container.NewHBox(transferControls, widget.NewSeparator(), ev.openAdminBtn),
	)

	// 2. Build Left Pane (Local)
	leftPane := ev.buildLocalPane()

	// 3. Build Right Pane (Remote)
	rightPane := ev.buildRemotePane()

	// 4. Split Container (50% local, 50% remote)
	ev.splitContainer = container.NewHSplit(leftPane, rightPane)
	ev.splitContainer.SetOffset(0.5)

	// 5. Global Bottom Status Bar (for error and info messages with ellipsis)
	ev.globalStatusBar = widget.NewLabel("")
	ev.globalStatusBar.Truncation = fyne.TextTruncateEllipsis
	ev.globalStatusBar.Importance = widget.LowImportance

	ev.reconnectBtn = widget.NewButtonWithIcon("Reconnect", theme.ViewRefreshIcon(), func() {
		if ev.OnReconnect != nil {
			ev.OnReconnect()
		}
	})
	ev.reconnectBtn.Hide()

	// 6. Master Layout
	ev.mainContainer = container.NewBorder(
		container.NewVBox(topBar, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), container.NewBorder(nil, nil, nil, ev.reconnectBtn, ev.globalStatusBar)),
		nil, nil,
		ev.splitContainer,
	)
}

func (ev *ExplorerView) buildLocalPane() fyne.CanvasObject {
	titleIcon := widget.NewIcon(PCIcon())
	titleLabel := widget.NewLabel("Local")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	titleBox := container.NewHBox(titleIcon, titleLabel)

	// Breadcrumb
	ev.localBreadcrumb = NewBreadcrumbBar(ev.window, ev.localPath, func(targetPath string) {
		ev.NavigateLocal(targetPath)
	})

	// Toolbar (compact icons)
	ev.localUpBtn = widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
		parent := filepath.Dir(ev.localPath)
		if parent != ev.localPath {
			ev.NavigateLocal(parent)
		}
	})
	ev.localRefreshBtn = widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		ev.RefreshLocal()
	})
	ev.localSelectAllBtn = widget.NewButtonWithIcon("", theme.CheckButtonCheckedIcon(), func() {
		ev.localList.ToggleSelectAll()
	})
	ev.localSelectAllBtn.Importance = widget.LowImportance

	ev.localNewFolder = widget.NewButtonWithIcon("", theme.FolderNewIcon(), func() {
		ev.promptNewLocalFolder()
	})
	ev.localDeleteBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		ev.promptDeleteLocal()
	})
	ev.localDeleteBtn.Disable()

	localToolbar := container.NewHBox(
		ev.localUpBtn,
		ev.localRefreshBtn,
		ev.localSelectAllBtn,
		ev.localNewFolder,
		ev.localDeleteBtn,
	)

	// File List
	ev.localList = NewFileListView(
		func(folderPath string) {
			ev.NavigateLocal(folderPath)
		},
		func(item sftp.Item) {
			ev.uploadItems([]sftp.Item{item})
		},
		func(selected []sftp.Item) {
			count := len(selected)
			if count > 0 {
				if ev.tunnelMgr != nil && ev.tunnelMgr.State() == tunnel.StateConnected {
					ev.btnUpload.Enable()
				} else {
					ev.btnUpload.Disable()
				}
				ev.btnUpload.SetText(fmt.Sprintf("Upload (%d)", count))
				ev.localDeleteBtn.Enable()
				ev.localInfo.SetText(fmt.Sprintf("%d items (%d selected)", ev.localItemsCount, count))
			} else {
				ev.btnUpload.Disable()
				ev.btnUpload.SetText("Upload")
				ev.localDeleteBtn.Disable()
				ev.localInfo.SetText(fmt.Sprintf("%d items", ev.localItemsCount))
			}
		},
	)

	ev.localInfo = widget.NewLabel("0 items")
	ev.localInfo.Truncation = fyne.TextTruncateEllipsis
	ev.localInfo.Importance = widget.LowImportance

	subHeader := container.NewBorder(nil, nil, localToolbar, nil, ev.localBreadcrumb.Container())
	header := container.NewVBox(
		titleBox,
		subHeader,
		widget.NewSeparator(),
	)

	return container.NewBorder(
		header,
		ev.localInfo,
		nil, nil,
		ev.localList.Widget(),
	)
}

func (ev *ExplorerView) buildRemotePane() fyne.CanvasObject {
	hostName := "Remote"
	if ev.tunnelMgr != nil && ev.tunnelMgr.Config() != nil && strings.TrimSpace(ev.tunnelMgr.Config().SSH.Host) != "" {
		hostName = strings.TrimSpace(ev.tunnelMgr.Config().SSH.Host)
	}

	titleIcon := widget.NewIcon(ServerIcon())
	ev.remoteTitleLabel = widget.NewLabel(hostName)
	ev.remoteTitleLabel.TextStyle = fyne.TextStyle{Bold: true}
	titleBox := container.NewHBox(titleIcon, ev.remoteTitleLabel)

	// Breadcrumb
	ev.remoteBreadcrumb = NewBreadcrumbBar(ev.window, ev.remotePath, func(targetPath string) {
		ev.NavigateRemote(targetPath)
	})

	// Toolbar (compact icons)
	ev.remoteUpBtn = widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
		parent := path.Dir(ev.remotePath)
		if parent != ev.remotePath {
			ev.NavigateRemote(parent)
		}
	})
	ev.remoteUpBtn.Disable()

	ev.remoteRefreshBtn = widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		ev.RefreshRemote()
	})
	ev.remoteRefreshBtn.Disable()

	ev.remoteSelectAllBtn = widget.NewButtonWithIcon("", theme.CheckButtonCheckedIcon(), func() {
		ev.remoteList.ToggleSelectAll()
	})
	ev.remoteSelectAllBtn.Importance = widget.LowImportance
	ev.remoteSelectAllBtn.Disable()

	ev.remoteNewFolder = widget.NewButtonWithIcon("", theme.FolderNewIcon(), func() {
		ev.promptNewRemoteFolder()
	})
	ev.remoteNewFolder.Disable()

	ev.remoteDeleteBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		ev.promptDeleteRemote()
	})
	ev.remoteDeleteBtn.Disable()

	remoteToolbar := container.NewHBox(
		ev.remoteUpBtn,
		ev.remoteRefreshBtn,
		ev.remoteSelectAllBtn,
		ev.remoteNewFolder,
		ev.remoteDeleteBtn,
	)

	// File List
	ev.remoteList = NewFileListView(
		func(folderPath string) {
			ev.NavigateRemote(folderPath)
		},
		func(item sftp.Item) {
			ev.downloadItems([]sftp.Item{item})
		},
		func(selected []sftp.Item) {
			count := len(selected)
			if count > 0 {
				if ev.tunnelMgr != nil && ev.tunnelMgr.State() == tunnel.StateConnected {
					ev.btnDownload.Enable()
					ev.remoteDeleteBtn.Enable()
				} else {
					ev.btnDownload.Disable()
					ev.remoteDeleteBtn.Disable()
				}
				ev.btnDownload.SetText(fmt.Sprintf("Download (%d)", count))
				ev.remoteInfo.SetText(fmt.Sprintf("%d items (%d selected)", ev.remoteItemsCount, count))
			} else {
				ev.btnDownload.Disable()
				ev.btnDownload.SetText("Download")
				ev.remoteDeleteBtn.Disable()
				if ev.tunnelMgr != nil && ev.tunnelMgr.State() == tunnel.StateConnected {
					ev.remoteInfo.SetText(fmt.Sprintf("%d items", ev.remoteItemsCount))
				} else {
					ev.remoteInfo.SetText("Disconnected")
				}
			}
		},
	)

	ev.remoteInfo = widget.NewLabel("Disconnected")
	ev.remoteInfo.Truncation = fyne.TextTruncateEllipsis
	ev.remoteInfo.Importance = widget.LowImportance

	subHeader := container.NewBorder(nil, nil, remoteToolbar, nil, ev.remoteBreadcrumb.Container())
	header := container.NewVBox(
		titleBox,
		subHeader,
		widget.NewSeparator(),
	)

	return container.NewBorder(
		header,
		ev.remoteInfo,
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
	} else {
		fyne.Do(func() {
			ev.remoteList.SetItems(nil)
		})
	}
}

// UpdateTunnelStatus updates UI elements according to the tunnel connection state.
func (ev *ExplorerView) UpdateTunnelStatus(st tunnel.State, msg string) {
	fyne.Do(func() {
		if ev.tunnelMgr != nil && ev.tunnelMgr.Config() != nil && ev.remoteTitleLabel != nil {
			if host := strings.TrimSpace(ev.tunnelMgr.Config().SSH.Host); host != "" {
				ev.remoteTitleLabel.SetText(host)
			}
		}

		switch st {
		case tunnel.StateConnected:
			ev.reconnectBtn.Hide()
			ev.globalStatusBar.Importance = widget.LowImportance
			ev.globalStatusBar.SetText("Tunnel active and secure.")
			ev.openAdminBtn.Enable()

			ev.remoteUpBtn.Enable()
			ev.remoteRefreshBtn.Enable()
			ev.remoteSelectAllBtn.Enable()
			ev.remoteNewFolder.Enable()

			if ev.localList.SelectedCount() > 0 {
				ev.btnUpload.Enable()
				ev.btnUpload.SetText(fmt.Sprintf("Upload (%d)", ev.localList.SelectedCount()))
			}
			if ev.remoteList.SelectedCount() > 0 {
				ev.btnDownload.Enable()
				ev.btnDownload.SetText(fmt.Sprintf("Download (%d)", ev.remoteList.SelectedCount()))
				ev.remoteDeleteBtn.Enable()
			}

		case tunnel.StateConnecting:
			ev.reconnectBtn.Hide()
			ev.globalStatusBar.Importance = widget.LowImportance
			ev.globalStatusBar.SetText("Establishing SSH connection...")
			ev.disableRemoteUI("Connecting...")

		case tunnel.StateError:
			ev.reconnectBtn.Show()
			ev.globalStatusBar.Importance = widget.DangerImportance
			errMsg := "Connection error"
			if msg != "" {
				errMsg = fmt.Sprintf("Error: %s", msg)
			}
			ev.globalStatusBar.SetText(errMsg)
			ev.disableRemoteUI("Connection failed")

		default:
			ev.reconnectBtn.Show()
			ev.globalStatusBar.Importance = widget.LowImportance
			ev.globalStatusBar.SetText("")
			ev.disableRemoteUI("Disconnected")
		}
	})
}

func (ev *ExplorerView) disableRemoteUI(statusText string) {
	ev.openAdminBtn.Disable()
	ev.btnUpload.Disable()
	ev.btnUpload.SetText("Upload")
	ev.btnDownload.Disable()
	ev.btnDownload.SetText("Download")

	ev.remoteUpBtn.Disable()
	ev.remoteRefreshBtn.Disable()
	ev.remoteSelectAllBtn.Disable()
	ev.remoteNewFolder.Disable()
	ev.remoteDeleteBtn.Disable()

	ev.remoteItemsCount = 0
	ev.remoteList.SetItems(nil)
	ev.remoteInfo.SetText(statusText)
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
				dialog.ShowError(fmt.Errorf("failed to list local directory: %w", err), ev.window)
				return
			}
			ev.localItemsCount = len(items)
			ev.localList.SetItems(items)

			dirs, files := countItems(items)
			ev.localInfo.SetText(fmt.Sprintf("%d items (%d folders, %d files)", len(items), dirs, files))
		})
	}()
}

func (ev *ExplorerView) promptNewLocalFolder() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("new_folder")

	dialog.ShowCustomConfirm("New Local Folder", "Create", "Cancel", entry, func(confirmed bool) {
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

func (ev *ExplorerView) promptDeleteLocal() {
	items := ev.localList.SelectedItems()
	if len(items) == 0 {
		return
	}

	var msg string
	if len(items) == 1 {
		it := items[0]
		if it.IsDir {
			msg = fmt.Sprintf("Delete local folder %q and all its contents?", it.Name)
		} else {
			msg = fmt.Sprintf("Delete local file %q?", it.Name)
		}
	} else {
		msg = fmt.Sprintf("Delete %d selected local items and their contents?", len(items))
	}

	dialog.ShowConfirm("Confirm Local Deletion", msg, func(confirmed bool) {
		if !confirmed {
			return
		}
		for _, it := range items {
			if err := local.Delete(it.Path, it.IsDir); err != nil {
				dialog.ShowError(err, ev.window)
				break
			}
		}
		ev.RefreshLocal()
	}, ev.window)
}

// --- Remote Navigation & Operations ---

func (ev *ExplorerView) NavigateRemote(targetPath string) {
	ev.mu.Lock()
	sc := ev.sftpClient
	ev.mu.Unlock()
	if sc == nil {
		return
	}

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
				dialog.ShowError(fmt.Errorf("failed to list remote directory %s: %w", curPath, err), ev.window)
				return
			}
			ev.remoteItemsCount = len(items)
			ev.remoteList.SetItems(items)

			dirs, files := countItems(items)
			ev.remoteInfo.SetText(fmt.Sprintf("%d items (%d folders, %d files)", len(items), dirs, files))
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
	entry.SetPlaceHolder("new_folder")

	dialog.ShowCustomConfirm("New Remote Folder", "Create", "Cancel", entry, func(confirmed bool) {
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

func (ev *ExplorerView) promptDeleteRemote() {
	ev.mu.Lock()
	sc := ev.sftpClient
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	items := ev.remoteList.SelectedItems()
	if len(items) == 0 {
		return
	}

	var msg string
	if len(items) == 1 {
		it := items[0]
		if it.IsDir {
			msg = fmt.Sprintf("Delete remote folder %q and all its contents?", it.Name)
		} else {
			msg = fmt.Sprintf("Delete remote file %q?", it.Name)
		}
	} else {
		msg = fmt.Sprintf("Delete %d selected remote items and their contents?", len(items))
	}

	dialog.ShowConfirm("Confirm Remote Deletion", msg, func(confirmed bool) {
		if !confirmed {
			return
		}
		go func() {
			for _, it := range items {
				if err := sc.Delete(it.Path, it.IsDir); err != nil {
					fyne.Do(func() { dialog.ShowError(err, ev.window) })
					break
				}
			}
			ev.RefreshRemote()
		}()
	}, ev.window)
}

// --- Transfers (Local <-> Remote) ---

// TransferSelectedLocalToRemote uploads all selected local items to the remote directory.
func (ev *ExplorerView) TransferSelectedLocalToRemote() {
	selected := ev.localList.SelectedItems()
	if len(selected) == 0 {
		return
	}
	ev.uploadItems(selected)
}

func (ev *ExplorerView) uploadItems(items []sftp.Item) {
	ev.mu.Lock()
	sc := ev.sftpClient
	targetRemoteDir := ev.remotePath
	ev.mu.Unlock()

	if sc == nil {
		dialog.ShowInformation("Not Connected", "Connect to server to upload files.", ev.window)
		return
	}

	totalItems := len(items)
	title := "Uploading"
	if totalItems == 1 {
		title = fmt.Sprintf("Uploading %s", items[0].Name)
	}

	prog := ShowTransferDialog(title, "", ev.window)

	go func() {
		defer prog.Close()

		for idx, item := range items {
			currentNum := idx + 1
			remoteTarget := path.Join(targetRemoteDir, item.Name)

			if item.IsDir {
				prog.SetTitle(fmt.Sprintf("[%d/%d] Uploading folder %s...", currentNum, totalItems, item.Name))
				prog.ResetProgress()
				err := sc.UploadDir(item.Path, remoteTarget, func(fileName string, size int64) {
					prog.SetTitle(fmt.Sprintf("[%d/%d] %s: %s", currentNum, totalItems, item.Name, fileName))
					prog.ResetProgress()
				}, func(transferred, total int64) {
					prog.UpdateProgress(transferred, total)
				})
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, ev.window) })
					break
				}
			} else {
				prog.SetTitle(fmt.Sprintf("[%d/%d] Uploading file %s...", currentNum, totalItems, item.Name))
				prog.ResetProgress()
				err := sc.Upload(item.Path, remoteTarget, func(transferred, total int64) {
					prog.UpdateProgress(transferred, total)
				})
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, ev.window) })
					break
				}
			}
		}

		ev.RefreshRemote()
		fyne.Do(func() {
			ev.localList.ClearSelection()
		})
	}()
}

// TransferSelectedRemoteToLocal downloads all selected remote items to the local directory.
func (ev *ExplorerView) TransferSelectedRemoteToLocal() {
	selected := ev.remoteList.SelectedItems()
	if len(selected) == 0 {
		return
	}
	ev.downloadItems(selected)
}

func (ev *ExplorerView) downloadItems(items []sftp.Item) {
	ev.mu.Lock()
	sc := ev.sftpClient
	targetLocalDir := ev.localPath
	ev.mu.Unlock()

	if sc == nil {
		return
	}

	totalItems := len(items)
	title := "Downloading"
	if totalItems == 1 {
		title = fmt.Sprintf("Downloading %s", items[0].Name)
	}

	prog := ShowTransferDialog(title, "", ev.window)

	go func() {
		defer prog.Close()

		for idx, item := range items {
			currentNum := idx + 1
			localTarget := filepath.Join(targetLocalDir, item.Name)

			if item.IsDir {
				prog.SetTitle(fmt.Sprintf("[%d/%d] Downloading folder %s...", currentNum, totalItems, item.Name))
				prog.ResetProgress()
				err := sc.DownloadDir(item.Path, localTarget, func(fileName string, size int64) {
					prog.SetTitle(fmt.Sprintf("[%d/%d] %s: %s", currentNum, totalItems, item.Name, fileName))
					prog.ResetProgress()
				}, func(transferred, total int64) {
					prog.UpdateProgress(transferred, total)
				})
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, ev.window) })
					break
				}
			} else {
				prog.SetTitle(fmt.Sprintf("[%d/%d] Downloading file %s...", currentNum, totalItems, item.Name))
				prog.ResetProgress()
				err := sc.Download(item.Path, localTarget, func(transferred, total int64) {
					prog.UpdateProgress(transferred, total)
				})
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, ev.window) })
					break
				}
			}
		}

		ev.RefreshLocal()
		fyne.Do(func() {
			ev.remoteList.ClearSelection()
		})
	}()
}

// UploadDroppedFiles uploads files or folders dragged from the OS to the remote directory.
func (ev *ExplorerView) UploadDroppedFiles(localPaths []string) {
	ev.mu.Lock()
	sc := ev.sftpClient
	targetRemoteDir := ev.remotePath
	ev.mu.Unlock()

	if sc == nil {
		dialog.ShowInformation("Not Connected", "Connect to server before dropping files.", ev.window)
		return
	}

	totalItems := len(localPaths)
	prog := ShowTransferDialog("Uploading", "", ev.window)

	go func() {
		defer prog.Close()

		for idx, localPath := range localPaths {
			currentNum := idx + 1
			fileName := filepath.Base(localPath)
			remoteTarget := path.Join(targetRemoteDir, fileName)

			stat, err := os.Stat(localPath)
			if err != nil {
				continue
			}

			if stat.IsDir() {
				prog.SetTitle(fmt.Sprintf("[%d/%d] Uploading folder %s...", currentNum, totalItems, fileName))
				prog.ResetProgress()
				err := sc.UploadDir(localPath, remoteTarget, func(subName string, size int64) {
					prog.SetTitle(fmt.Sprintf("[%d/%d] %s: %s", currentNum, totalItems, fileName, subName))
					prog.ResetProgress()
				}, func(transferred, total int64) {
					prog.UpdateProgress(transferred, total)
				})
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, ev.window) })
					break
				}
			} else {
				prog.SetTitle(fmt.Sprintf("[%d/%d] Uploading file %s...", currentNum, totalItems, fileName))
				prog.ResetProgress()
				err := sc.Upload(localPath, remoteTarget, func(transferred, total int64) {
					prog.UpdateProgress(transferred, total)
				})
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, ev.window) })
					break
				}
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
