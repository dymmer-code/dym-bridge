//go:build fyne

package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/systray"
	"github.com/dymmer-code/dym-bridge/pkg/tunnel"
)

// TrayManager manages the system tray icon, tooltip and contextual menu.
type TrayManager struct {
	app         fyne.App
	window      fyne.Window
	tunnelMgr   *tunnel.Manager
	iconBytes   []byte
	iconRes     fyne.Resource
	onOpenAdmin func()
	onReconnect func()
}

// NewTrayManager initializes and configures the system tray integration.
func NewTrayManager(app fyne.App, window fyne.Window, tm *tunnel.Manager, onOpenAdmin, onReconnect func()) *TrayManager {
	rawPNG := CreateTrayTemplateIcon(32)
	baseRes := fyne.NewStaticResource("dym_bridge_tray.png", rawPNG)
	themedRes := GetTrayIconResource()

	tray := &TrayManager{
		app:         app,
		window:      window,
		tunnelMgr:   tm,
		iconBytes:   baseRes.Content(),
		iconRes:     themedRes,
		onOpenAdmin: onOpenAdmin,
		onReconnect: onReconnect,
	}

	if desk, ok := app.(desktop.App); ok {
		desk.SetSystemTrayIcon(tray.iconRes)
	}

	tray.Refresh()

	// Ensure template icon and title sync as soon as native Cocoa/Win32 event loop is ready
	go func() {
		for _, delay := range []time.Duration{50 * time.Millisecond, 150 * time.Millisecond, 300 * time.Millisecond, 600 * time.Millisecond} {
			time.Sleep(delay)
			fyne.Do(func() {
				tray.Refresh()
			})
		}
	}()

	return tray
}

// Refresh updates the tray title, tooltip and context menu entries.
func (t *TrayManager) Refresh() {
	desk, ok := t.app.(desktop.App)
	if !ok {
		return
	}

	systray.SetTemplateIcon(t.iconBytes, t.iconBytes)

	state := t.tunnelMgr.State()
	cfg := t.tunnelMgr.Config()

	switch state {
	case tunnel.StateConnected:
		systray.SetTitle(" Dym Bridge ●")
		systray.SetTooltip(fmt.Sprintf("Dym Bridge — Active tunnel (%s)", t.tunnelMgr.AdminURL()))
	case tunnel.StateConnecting:
		systray.SetTitle(" Dym Bridge ◌")
		systray.SetTooltip("Dym Bridge — Connecting...")
	case tunnel.StateError:
		systray.SetTitle(" Dym Bridge ✕")
		systray.SetTooltip("Dym Bridge — Connection error")
	default:
		systray.SetTitle(" Dym Bridge ○")
		systray.SetTooltip("Dym Bridge — Disconnected")
	}

	var menuItems []*fyne.MenuItem

	// 1. Open Web Admin item
	openAdminLabel := "Open Web Admin"
	if state == tunnel.StateConnected {
		openAdminLabel = fmt.Sprintf("● %s", openAdminLabel)
	}
	openAdminItem := fyne.NewMenuItem(openAdminLabel, func() {
		if t.onOpenAdmin != nil {
			t.onOpenAdmin()
		}
	})
	openAdminItem.Disabled = (state != tunnel.StateConnected)
	menuItems = append(menuItems, openAdminItem)

	// 2. Open File Explorer item
	openExplorerItem := fyne.NewMenuItem("File Explorer...", func() {
		t.window.Show()
		t.window.RequestFocus()
	})
	menuItems = append(menuItems, openExplorerItem)

	menuItems = append(menuItems, fyne.NewMenuItemSeparator())

	// 3. Connect / Reconnect item
	if state == tunnel.StateConnected {
		menuItems = append(menuItems, fyne.NewMenuItem("Disconnect Tunnel", func() {
			t.tunnelMgr.Disconnect()
		}))
	} else {
		menuItems = append(menuItems, fyne.NewMenuItem("Reconnect Tunnel", func() {
			if t.onReconnect != nil {
				t.onReconnect()
			}
		}))
	}

	// 4. Host info display
	if cfg != nil {
		infoItem := fyne.NewMenuItem(fmt.Sprintf("Server: %s", cfg.SSH.Host), nil)
		infoItem.Disabled = true
		menuItems = append(menuItems, infoItem)
	}

	menuItems = append(menuItems, fyne.NewMenuItemSeparator())

	// 5. Quit application item
	quitItem := fyne.NewMenuItem("Quit Dym Bridge", func() {
		t.app.Quit()
	})
	quitItem.IsQuit = true
	menuItems = append(menuItems, quitItem)

	menu := fyne.NewMenu("Dym Bridge", menuItems...)
	desk.SetSystemTrayMenu(menu)
}
