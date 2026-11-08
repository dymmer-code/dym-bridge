// Package desktop integrates native URL callbacks and the resident tray without CGo.
package desktop

type Actions struct {
	Open       func()
	Connect    func()
	Disconnect func()
	Sites      func()
	Quit       func()
}
