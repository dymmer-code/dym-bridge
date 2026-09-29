package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dymmer-code/dym-bridge/pkg/secret"
)

// ShowEncryptDialog opens a modal dialog allowing the user to encrypt a password or SSH key.
func ShowEncryptDialog(parent fyne.Window) {
	secretEntry := widget.NewMultiLineEntry()
	secretEntry.SetPlaceHolder("Pegue aquí la contraseña o clave privada SSH (OpenSSH / PEM)...")
	secretEntry.Wrapping = fyne.TextWrapWord
	secretEntry.SetMinRowsVisible(4)

	masterEntry := widget.NewPasswordEntry()
	masterEntry.SetPlaceHolder("Clave maestra para cifrar...")

	confirmEntry := widget.NewPasswordEntry()
	confirmEntry.SetPlaceHolder("Confirmar clave maestra...")

	resultEntry := widget.NewEntry()
	resultEntry.SetPlaceHolder("El valor ENC(...) aparecerá aquí tras cifrar...")
	resultEntry.Disable()

	errLabel := widget.NewLabel("")
	errLabel.Importance = widget.DangerImportance
	errLabel.Wrapping = fyne.TextWrapWord

	btnCopy := widget.NewButtonWithIcon("Copiar", theme.ContentCopyIcon(), func() {
		if resultEntry.Text != "" {
			parent.Clipboard().SetContent(resultEntry.Text)
			errLabel.Importance = widget.SuccessImportance
			errLabel.SetText("✔ Copiado al portapapeles. Péguelo en su dym-bridge.ini")
		}
	})
	btnCopy.Disable()

	var d dialog.Dialog

	btnEncrypt := widget.NewButtonWithIcon("Cifrar", theme.ConfirmIcon(), func() {
		errLabel.Importance = widget.DangerImportance
		text := secretEntry.Text
		if text == "" {
			errLabel.SetText("El texto secreto no puede estar vacío.")
			return
		}

		pwd := masterEntry.Text
		if pwd == "" {
			errLabel.SetText("La clave maestra no puede estar vacía.")
			return
		}

		if pwd != confirmEntry.Text {
			errLabel.SetText("Las claves maestras no coinciden.")
			return
		}

		enc, err := secret.Encrypt(text, pwd)
		if err != nil {
			errLabel.SetText("Error al cifrar: " + err.Error())
			return
		}

		resultEntry.Enable()
		resultEntry.SetText(enc)
		resultEntry.Disable()
		btnCopy.Enable()

		errLabel.Importance = widget.SuccessImportance
		errLabel.SetText("✔ Cifrado con éxito (AES-256-GCM). Copie el valor y colóquelo en dym-bridge.ini.")
	})
	btnEncrypt.Importance = widget.HighImportance

	btnClose := widget.NewButtonWithIcon("Cerrar", theme.CancelIcon(), func() {
		if d != nil {
			d.Hide()
		}
	})

	form := container.NewVBox(
		widget.NewLabelWithStyle("Texto a cifrar (contraseña o clave SSH):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		secretEntry,
		widget.NewLabelWithStyle("Clave maestra:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		masterEntry,
		widget.NewLabelWithStyle("Confirmar clave maestra:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		confirmEntry,
		container.NewHBox(btnEncrypt),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Resultado para dym-bridge.ini:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, btnCopy, resultEntry),
		errLabel,
	)

	content := container.NewVBox(
		form,
		widget.NewSeparator(),
		container.NewHBox(btnClose),
	)

	d = dialog.NewCustomWithoutButtons("Cifrar Credenciales (Contraseña o Clave SSH)", content, parent)
	d.Resize(fyne.NewSize(540, 420))
	d.Show()
}
