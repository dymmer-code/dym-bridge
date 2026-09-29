package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/dymmer-code/dym-bridge/pkg/sftp"
)

// TransferDialog displays progress for file upload/download operations.
type TransferDialog struct {
	dialog      dialog.Dialog
	titleLabel  *widget.Label
	statusLabel *widget.Label
	progressBar *widget.ProgressBar
}

// ShowTransferDialog creates and displays a transfer progress dialog.
func ShowTransferDialog(title, fileName string, parent fyne.Window) *TransferDialog {
	titleText := title
	if fileName != "" {
		titleText = fmt.Sprintf("%s: %s", title, fileName)
	}
	titleLabel := widget.NewLabel(titleText)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	titleLabel.Truncation = fyne.TextTruncateEllipsis

	statusLabel := widget.NewLabel("Starting transfer...")
	statusLabel.Truncation = fyne.TextTruncateEllipsis
	progressBar := widget.NewProgressBar()
	progressBar.SetValue(0)

	content := container.NewVBox(
		titleLabel,
		progressBar,
		statusLabel,
	)

	d := dialog.NewCustomWithoutButtons(title, content, parent)
	d.Resize(fyne.NewSize(380, 140))
	d.Show()

	return &TransferDialog{
		dialog:      d,
		titleLabel:  titleLabel,
		statusLabel: statusLabel,
		progressBar: progressBar,
	}
}

// SetTitle updates the dialog title/header text.
func (td *TransferDialog) SetTitle(title string) {
	fyne.Do(func() {
		td.titleLabel.SetText(title)
	})
}

// SetStatus updates the status label text.
func (td *TransferDialog) SetStatus(status string) {
	fyne.Do(func() {
		td.statusLabel.SetText(status)
	})
}

// ResetProgress resets the progress bar value to 0.
func (td *TransferDialog) ResetProgress() {
	fyne.Do(func() {
		td.progressBar.SetValue(0)
	})
}

// UpdateProgress updates the progress bar and status text.
func (td *TransferDialog) UpdateProgress(transferred, total int64) {
	fyne.Do(func() {
		if total > 0 {
			fraction := float64(transferred) / float64(total)
			td.progressBar.SetValue(fraction)
			td.statusLabel.SetText(fmt.Sprintf("%s / %s (%.0f%%)",
				sftp.FormatFileSize(transferred),
				sftp.FormatFileSize(total),
				fraction*100,
			))
		} else {
			td.statusLabel.SetText(sftp.FormatFileSize(transferred))
		}
	})
}

// Close dismisses the transfer dialog.
func (td *TransferDialog) Close() {
	fyne.Do(func() {
		td.dialog.Hide()
	})
}
