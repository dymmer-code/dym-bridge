package ui

import (
	"path"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
)

// fileRowWidget implements fyne.Tappable and fyne.DoubleTappable so clicking toggles
// selection and double-clicks open folders or transfer files.
type fileRowWidget struct {
	widget.BaseWidget
	flv       *FileListView
	itemIndex int
	itemPath  string

	check     *widget.Check
	icon      *widget.Icon
	nameLabel *widget.Label
	sizeLabel *widget.Label
	dateLabel *widget.Label
	container *fyne.Container
}

func newFileRowWidget(flv *FileListView) *fileRowWidget {
	w := &fileRowWidget{flv: flv, itemIndex: -1}
	w.ExtendBaseWidget(w)

	w.check = widget.NewCheck("", func(checked bool) {
		if w.flv != nil && w.itemPath != "" {
			w.flv.SetPathSelected(w.itemPath, checked)
		}
	})

	w.icon = widget.NewIcon(theme.FileIcon())

	w.nameLabel = widget.NewLabel("")
	w.nameLabel.Truncation = fyne.TextTruncateEllipsis
	w.nameLabel.Wrapping = fyne.TextTruncate

	w.sizeLabel = widget.NewLabel("")
	w.sizeLabel.Alignment = fyne.TextAlignTrailing
	w.sizeLabel.TextStyle = fyne.TextStyle{Monospace: true}
	w.sizeLabel.Importance = widget.LowImportance

	w.dateLabel = widget.NewLabel("")
	w.dateLabel.Alignment = fyne.TextAlignTrailing
	w.dateLabel.Importance = widget.LowImportance

	rightBox := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(52, 20), w.sizeLabel),
		container.NewGridWrap(fyne.NewSize(72, 20), w.dateLabel),
	)
	leftIconBox := container.NewHBox(w.check, w.icon)
	leftBox := container.NewBorder(nil, nil, leftIconBox, nil, w.nameLabel)
	w.container = container.NewBorder(nil, nil, nil, rightBox, leftBox)

	return w
}

func (w *fileRowWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.container)
}

func (w *fileRowWidget) Tapped(_ *fyne.PointEvent) {
	if w.flv != nil && w.itemPath != "" {
		w.flv.TogglePathSelected(w.itemPath)
	}
}

func (w *fileRowWidget) DoubleTapped(_ *fyne.PointEvent) {
	if w.flv == nil || w.itemIndex < 0 {
		return
	}
	w.flv.mu.RLock()
	if w.itemIndex >= len(w.flv.items) {
		w.flv.mu.RUnlock()
		return
	}
	item := w.flv.items[w.itemIndex]
	w.flv.mu.RUnlock()

	if item.IsDir {
		if w.flv.onOpenFolder != nil {
			w.flv.onOpenFolder(item.Path)
		}
	} else {
		if w.flv.onFileAction != nil {
			w.flv.onFileAction(item)
		}
	}
}

// FileListView renders a dense file table with multi-selection and double-click support.
type FileListView struct {
	list          *widget.List
	items         []sftp.Item
	selectedPaths map[string]bool
	mu            sync.RWMutex

	onOpenFolder       func(folderPath string)
	onFileAction       func(item sftp.Item)
	onSelectionChanged func(items []sftp.Item)
}

// NewFileListView creates a multi-selectable file list view.
func NewFileListView(onOpenFolder func(path string), onFileAction func(item sftp.Item), onSelectionChanged func(items []sftp.Item)) *FileListView {
	flv := &FileListView{
		selectedPaths:      make(map[string]bool),
		onOpenFolder:       onOpenFolder,
		onFileAction:       onFileAction,
		onSelectionChanged: onSelectionChanged,
	}

	flv.list = widget.NewList(
		func() int {
			flv.mu.RLock()
			defer flv.mu.RUnlock()
			return len(flv.items)
		},
		func() fyne.CanvasObject {
			return newFileRowWidget(flv)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row := obj.(*fileRowWidget)
			row.itemIndex = id

			flv.mu.RLock()
			if id < 0 || id >= len(flv.items) {
				flv.mu.RUnlock()
				return
			}
			item := flv.items[id]
			isSelected := flv.selectedPaths[item.Path]
			flv.mu.RUnlock()

			row.itemPath = item.Path

			// Temporarily disable OnChanged to avoid firing during row recycling
			row.check.OnChanged = nil
			row.check.SetChecked(isSelected)
			row.check.OnChanged = func(checked bool) {
				if row.flv != nil && row.itemPath != "" {
					row.flv.SetPathSelected(row.itemPath, checked)
				}
			}

			if item.IsDir {
				row.icon.SetResource(theme.FolderIcon())
				row.nameLabel.TextStyle = fyne.TextStyle{Bold: true}
				row.sizeLabel.SetText("—")
			} else {
				row.icon.SetResource(getFileTypeIcon(item.Name))
				row.nameLabel.TextStyle = fyne.TextStyle{Bold: false}
				row.sizeLabel.SetText(sftp.FormatFileSize(item.Size))
			}

			row.nameLabel.SetText(item.Name)
			row.dateLabel.SetText(item.ModTime.Format("02/01 15:04"))
		},
	)

	return flv
}

// SetItems updates the file items and clears any obsolete selections.
func (flv *FileListView) SetItems(items []sftp.Item) {
	flv.mu.Lock()
	flv.items = items
	flv.selectedPaths = make(map[string]bool)
	flv.mu.Unlock()

	flv.list.Refresh()
	flv.notifySelection()
}

// SetPathSelected sets the selection state for a specific item path.
func (flv *FileListView) SetPathSelected(p string, selected bool) {
	flv.mu.Lock()
	if selected {
		flv.selectedPaths[p] = true
	} else {
		delete(flv.selectedPaths, p)
	}
	flv.mu.Unlock()

	flv.list.Refresh()
	flv.notifySelection()
}

// TogglePathSelected toggles the selection state of an item path.
func (flv *FileListView) TogglePathSelected(p string) {
	flv.mu.Lock()
	if flv.selectedPaths[p] {
		delete(flv.selectedPaths, p)
	} else {
		flv.selectedPaths[p] = true
	}
	flv.mu.Unlock()

	flv.list.Refresh()
	flv.notifySelection()
}

// SelectAll selects all items in the current view.
func (flv *FileListView) SelectAll() {
	flv.mu.Lock()
	for _, it := range flv.items {
		flv.selectedPaths[it.Path] = true
	}
	flv.mu.Unlock()

	flv.list.Refresh()
	flv.notifySelection()
}

// ClearSelection deselects all items.
func (flv *FileListView) ClearSelection() {
	flv.mu.Lock()
	flv.selectedPaths = make(map[string]bool)
	flv.mu.Unlock()

	flv.list.Refresh()
	flv.notifySelection()
}

// ToggleSelectAll toggles between selecting all items and selecting none.
func (flv *FileListView) ToggleSelectAll() {
	flv.mu.RLock()
	total := len(flv.items)
	selected := len(flv.selectedPaths)
	flv.mu.RUnlock()

	if total > 0 && selected == total {
		flv.ClearSelection()
	} else {
		flv.SelectAll()
	}
}

// SelectedItems returns all currently selected items in listing order.
func (flv *FileListView) SelectedItems() []sftp.Item {
	flv.mu.RLock()
	defer flv.mu.RUnlock()

	var selected []sftp.Item
	for _, it := range flv.items {
		if flv.selectedPaths[it.Path] {
			selected = append(selected, it)
		}
	}
	return selected
}

// SelectedCount returns the number of currently selected items.
func (flv *FileListView) SelectedCount() int {
	flv.mu.RLock()
	defer flv.mu.RUnlock()
	return len(flv.selectedPaths)
}

func (flv *FileListView) notifySelection() {
	if flv.onSelectionChanged != nil {
		selected := flv.SelectedItems()
		flv.onSelectionChanged(selected)
	}
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
