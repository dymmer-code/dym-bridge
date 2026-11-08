//go:build !darwin && !windows && !linux

package desktop

// Other desktops retain the native application menu; without a tray, closing
// the window must terminate instead of leaving an inaccessible process.
func Install(a Actions) (func(), bool, error) { return func() {}, false, nil }
