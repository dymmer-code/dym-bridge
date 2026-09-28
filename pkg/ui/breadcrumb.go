package ui

import (
	"path"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// BreadcrumbBar renders a clickable breadcrumb path navigation component.
type BreadcrumbBar struct {
	container    *fyne.Container
	scroll       *container.Scroll
	currentPath  string
	onNavigate   func(targetPath string)
}

// NewBreadcrumbBar creates a new breadcrumb navigation bar.
func NewBreadcrumbBar(initialPath string, onNavigate func(targetPath string)) *BreadcrumbBar {
	bb := &BreadcrumbBar{
		container:   container.NewHBox(),
		currentPath: initialPath,
		onNavigate:  onNavigate,
	}
	bb.scroll = container.NewHScroll(bb.container)
	bb.SetPath(initialPath)
	return bb
}

// SetPath updates the breadcrumb buttons to match the given directory path.
func (bb *BreadcrumbBar) SetPath(dirPath string) {
	bb.currentPath = dirPath
	bb.container.Objects = nil

	cleanPath := path.Clean(dirPath)
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	// Root button
	rootBtn := widget.NewButtonWithIcon("/", theme.HomeIcon(), func() {
		if bb.onNavigate != nil {
			bb.onNavigate("/")
		}
	})
	rootBtn.Importance = widget.LowImportance
	bb.container.Add(rootBtn)

	if cleanPath == "/" {
		bb.container.Refresh()
		return
	}

	segments := strings.Split(strings.TrimPrefix(cleanPath, "/"), "/")
	accumPath := ""

	for i, segment := range segments {
		if segment == "" {
			continue
		}
		accumPath = path.Join(accumPath, segment)
		target := "/" + accumPath

		// Separator chevron
		sep := widget.NewLabel(">")
		sep.Importance = widget.LowImportance
		bb.container.Add(sep)

		isLast := (i == len(segments)-1)
		segBtn := widget.NewButton(segment, func(p string) func() {
			return func() {
				if bb.onNavigate != nil {
					bb.onNavigate(p)
				}
			}
		}(target))

		if isLast {
			segBtn.Importance = widget.HighImportance
		} else {
			segBtn.Importance = widget.LowImportance
		}

		bb.container.Add(segBtn)
	}

	bb.container.Refresh()
	bb.scroll.Refresh()
}

// CurrentPath returns the currently displayed directory path.
func (bb *BreadcrumbBar) CurrentPath() string {
	return bb.currentPath
}

// Container returns the Fyne canvas object for embedding in layouts.
func (bb *BreadcrumbBar) Container() fyne.CanvasObject {
	return bb.scroll
}
