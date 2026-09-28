package ui

import (
	"fmt"
	"path"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	maxVisibleSegments = 3
	maxSegmentChars    = 13
)

// BreadcrumbBar renders a compact, responsive breadcrumb navigation bar.
type BreadcrumbBar struct {
	window       fyne.Window
	rowBox       *fyne.Container
	scroll       *container.Scroll
	pathLabel    *widget.Label
	wrapper      *fyne.Container
	currentPath  string
	onNavigate   func(targetPath string)
}

// NewBreadcrumbBar creates a new responsive breadcrumb navigation bar.
func NewBreadcrumbBar(window fyne.Window, initialPath string, onNavigate func(targetPath string)) *BreadcrumbBar {
	bb := &BreadcrumbBar{
		window:      window,
		rowBox:      container.NewHBox(),
		currentPath: initialPath,
		onNavigate:  onNavigate,
	}

	bb.pathLabel = widget.NewLabel(initialPath)
	bb.pathLabel.Truncation = fyne.TextTruncateEllipsis
	bb.pathLabel.Importance = widget.LowImportance

	bb.scroll = container.NewHScroll(bb.rowBox)

	// Stack breadcrumb buttons on top, full path label with ellipsis underneath
	bb.wrapper = container.NewVBox(
		bb.scroll,
		bb.pathLabel,
	)

	bb.SetPath(initialPath)
	return bb
}

// SetPath updates the breadcrumb buttons, limiting segment count and label length.
func (bb *BreadcrumbBar) SetPath(dirPath string) {
	bb.currentPath = dirPath
	bb.rowBox.Objects = nil

	cleanPath := path.Clean(dirPath)
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	bb.pathLabel.SetText(fmt.Sprintf("Ruta: %s", cleanPath))

	// Root button
	rootBtn := widget.NewButtonWithIcon("/", theme.HomeIcon(), func() {
		if bb.onNavigate != nil {
			bb.onNavigate("/")
		}
	})
	rootBtn.Importance = widget.LowImportance
	bb.rowBox.Add(rootBtn)

	if cleanPath == "/" {
		bb.rowBox.Refresh()
		bb.scroll.Refresh()
		return
	}

	rawSegments := strings.Split(strings.TrimPrefix(cleanPath, "/"), "/")
	var segments []string
	for _, s := range rawSegments {
		if s != "" {
			segments = append(segments, s)
		}
	}

	totalSegments := len(segments)

	if totalSegments <= maxVisibleSegments {
		// All segments fit within the limit
		accum := ""
		for i, seg := range segments {
			accum = path.Join(accum, seg)
			bb.addSegmentButton(seg, "/"+accum, i == totalSegments-1)
		}
	} else {
		// Too many segments: collapse leading segments into a "[ ... ]" dropdown menu
		hiddenCount := totalSegments - (maxVisibleSegments - 1)
		var hiddenMenuItems []*fyne.MenuItem
		accum := ""

		for i := 0; i < hiddenCount; i++ {
			accum = path.Join(accum, segments[i])
			targetPath := "/" + accum
			label := segments[i]
			item := fyne.NewMenuItem(label, func() {
				if bb.onNavigate != nil {
					bb.onNavigate(targetPath)
				}
			})
			hiddenMenuItems = append(hiddenMenuItems, item)
		}

		// Separator
		sep := widget.NewLabel(">")
		sep.Importance = widget.LowImportance
		bb.rowBox.Add(sep)

		// Ellipsis button
		ellipsisBtn := widget.NewButton("...", func() {
			if bb.window != nil && len(hiddenMenuItems) > 0 {
				menu := fyne.NewMenu("Carpetas intermedias", hiddenMenuItems...)
				popup := widget.NewPopUpMenu(menu, bb.window.Canvas())
				popup.Show()
			}
		})
		ellipsisBtn.Importance = widget.LowImportance
		bb.rowBox.Add(ellipsisBtn)

		// Show only the remaining trailing segments
		for i := hiddenCount; i < totalSegments; i++ {
			accum = path.Join(accum, segments[i])
			bb.addSegmentButton(segments[i], "/"+accum, i == totalSegments-1)
		}
	}

	bb.rowBox.Refresh()
	bb.scroll.Refresh()
}

func (bb *BreadcrumbBar) addSegmentButton(name, targetPath string, isLast bool) {
	sep := widget.NewLabel(">")
	sep.Importance = widget.LowImportance
	bb.rowBox.Add(sep)

	displayLabel := truncateSegment(name, maxSegmentChars)
	btn := widget.NewButton(displayLabel, func() {
		if bb.onNavigate != nil {
			bb.onNavigate(targetPath)
		}
	})

	if isLast {
		btn.Importance = widget.HighImportance
	} else {
		btn.Importance = widget.LowImportance
	}

	bb.rowBox.Add(btn)
}

func truncateSegment(str string, maxChars int) string {
	runes := []rune(str)
	if len(runes) <= maxChars {
		return str
	}
	return string(runes[:maxChars-1]) + "…"
}

// CurrentPath returns the currently displayed directory path.
func (bb *BreadcrumbBar) CurrentPath() string {
	return bb.currentPath
}

// Container returns the Fyne canvas object for embedding in layouts.
func (bb *BreadcrumbBar) Container() fyne.CanvasObject {
	return bb.wrapper
}
