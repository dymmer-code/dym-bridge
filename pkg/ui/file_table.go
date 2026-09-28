package ui

import (
	"path"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
)

// FileItemRow represents the visual template for a single row in the file list.
type FileItemRow struct {
	Container *fyne.Container
	Icon      *widget.Icon
	NameLabel *widget.Label
	SizeLabel *widget.Label
	DateLabel *widget.Label
}

// FileListView manages the virtualized list of files and folders.
type FileListView struct {
	list        *widget.List
	items       []sftp.Item
	selectedIdx int
	mu          sync.RWMutex

	lastTapID   int
	lastTapTime time.Time

	onOpenFolder func(folderPath string)
	onFileAction func(item sftp.Item)
	onSelect     func(item *sftp.Item)
}

// NewFileListView creates a new file list view.
func NewFileListView(onOpenFolder func(path string), onFileAction func(item sftp.Item), onSelect func(item *sftp.Item)) *FileListView {
	flv := &FileListView{
		selectedIdx:  -1,
		lastTapID:    -1,
		onOpenFolder: onOpenFolder,
		onFileAction: onFileAction,
		onSelect:     onSelect,
	}

	flv.list = widget.NewList(
		func() int {
			flv.mu.RLock()
			defer flv.mu.RUnlock()
			return len(flv.items)
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.FileIcon())
			nameLabel := widget.NewLabel("sample_file.txt")
			nameLabel.Truncation = fyne.TextTruncateEllipsis

			sizeLabel := widget.NewLabel("100 KB")
			sizeLabel.Alignment = fyne.TextAlignTrailing
			sizeLabel.TextStyle = fyne.TextStyle{Monospace: true}

			dateLabel := widget.NewLabel("2026-09-28 00:00")
			dateLabel.Alignment = fyne.TextAlignTrailing
			dateLabel.Importance = widget.LowImportance

			// Layout: Icon + Name (expanding) on left, Size + Date on right
			leftBox := container.NewBorder(nil, nil, icon, nil, nameLabel)
			rightBox := container.NewHBox(
				container.NewGridWrap(fyne.NewSize(90, 24), sizeLabel),
				container.NewGridWrap(fyne.NewSize(140, 24), dateLabel),
			)

			return container.NewBorder(nil, nil, nil, rightBox, leftBox)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			flv.mu.RLock()
			if id < 0 || id >= len(flv.items) {
				flv.mu.RUnlock()
				return
			}
			item := flv.items[id]
			flv.mu.RUnlock()

			border := obj.(*fyne.Container)
			leftBox := border.Objects[0].(*fyne.Container)
			icon := leftBox.Objects[1].(*widget.Icon)
			nameLabel := leftBox.Objects[0].(*widget.Label)

			rightBox := border.Objects[1].(*fyne.Container)
			sizeGrid := rightBox.Objects[0].(*fyne.Container)
			sizeLabel := sizeGrid.Objects[0].(*widget.Label)
			dateGrid := rightBox.Objects[1].(*fyne.Container)
			dateLabel := dateGrid.Objects[0].(*widget.Label)

			// Icon & Typography
			if item.IsDir {
				icon.SetResource(theme.FolderIcon())
				nameLabel.TextStyle = fyne.TextStyle{Bold: true}
				sizeLabel.SetText("—")
			} else {
				icon.SetResource(getFileTypeIcon(item.Name))
				nameLabel.TextStyle = fyne.TextStyle{Bold: false}
				sizeLabel.SetText(sftp.FormatFileSize(item.Size))
			}

			nameLabel.SetText(item.Name)
			dateLabel.SetText(item.ModTime.Format("2006-01-02 15:04"))
		},
	)

	flv.list.OnSelected = func(id widget.ListItemID) {
		flv.mu.Lock()
		if id < 0 || id >= len(flv.items) {
			flv.mu.Unlock()
			return
		}
		item := flv.items[id]
		flv.selectedIdx = id

		now := time.Now()
		isDoubleTap := (flv.lastTapID == id && now.Sub(flv.lastTapTime) < 450*time.Millisecond)
		flv.lastTapID = id
		flv.lastTapTime = now
		flv.mu.Unlock()

		if flv.onSelect != nil {
			flv.onSelect(&item)
		}

		if isDoubleTap {
			if item.IsDir {
				if flv.onOpenFolder != nil {
					flv.onOpenFolder(item.Path)
				}
			} else {
				if flv.onFileAction != nil {
					flv.onFileAction(item)
				}
			}
		}
	}

	return flv
}

// SetItems updates the file items and refreshes the view.
func (flv *FileListView) SetItems(items []sftp.Item) {
	flv.mu.Lock()
	flv.items = items
	flv.selectedIdx = -1
	flv.lastTapID = -1
	flv.mu.Unlock()

	flv.list.UnselectAll()
	flv.list.Refresh()
}

// SelectedItem returns the currently selected item, or nil.
func (flv *FileListView) SelectedItem() *sftp.Item {
	flv.mu.RLock()
	defer flv.mu.RUnlock()
	if flv.selectedIdx >= 0 && flv.selectedIdx < len(flv.items) {
		item := flv.items[flv.selectedIdx]
		return &item
	}
	return nil
}

// Widget returns the underlying Fyne widget.
func (flv *FileListView) Widget() fyne.CanvasObject {
	return flv.list
}

func getFileTypeIcon(filename string) fyne.Resource {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico":
		return theme.FileImageIcon()
	case ".zip", ".tar", ".gz", ".bz2", ".7z", ".rar":
		return theme.StorageIcon()
	case ".txt", ".md", ".json", ".xml", ".log":
		return theme.FileTextIcon()
	case ".php", ".js", ".css", ".html", ".go", ".py", ".sql":
		return theme.DocumentCreateIcon()
	default:
		return theme.FileIcon()
	}
}
