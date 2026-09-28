package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// DymBridgeTheme provides a clean, native-feeling theme with high contrast and polish.
type DymBridgeTheme struct{ fyne.Theme }

func NewDymBridgeTheme() fyne.Theme {
	return &DymBridgeTheme{theme.DefaultTheme()}
}

func (t *DymBridgeTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant == theme.VariantDark
	pair := func(light, night uint32) color.Color {
		v := light
		if dark {
			v = night
		}
		return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
	}

	switch name {
	case theme.ColorNameBackground:
		return pair(0xF8F9FA, 0x181C22)
	case theme.ColorNameForeground:
		return pair(0x212529, 0xE9ECEF)
	case theme.ColorNamePrimary:
		// Elegant Dymmer Blue / Cyan
		return pair(0x0D6EFD, 0x3D8BFD)
	case theme.ColorNameButton:
		return pair(0xEAEEF3, 0x272E38)
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return pair(0xFFFFFF, 0x212730)
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return pair(0xDDE2E5, 0x36404D)
	case theme.ColorNameForegroundOnPrimary:
		return pair(0xFFFFFF, 0xFFFFFF)
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return pair(0x6C757D, 0xADB5BD)
	case theme.ColorNameSelection:
		return pair(0xCFE2FF, 0x224068)
	case theme.ColorNameHover:
		return pair(0xE2E6EA, 0x2D3540)
	case theme.ColorNameFocus:
		return pair(0x9EC5FE, 0x3867A6)
	}

	return t.Theme.Color(name, variant)
}

func (t *DymBridgeTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 15
	}
	return t.Theme.Size(name)
}
