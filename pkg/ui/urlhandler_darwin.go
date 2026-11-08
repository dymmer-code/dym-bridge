//go:build fyne && darwin

package ui

/*
#cgo LDFLAGS: -framework Cocoa
#include "urlhandler_darwin.h"
*/
import "C"

import (
	"strings"

	"github.com/dymmer-code/dym-bridge/pkg/callback"
)

// On macOS the system does not start a second process for dym-bridge://...: it sends an Apple event to the one that
// is running. This hands that address to the sign-in that is waiting, the same way the other systems do through the
// argument of a second process.
func init() { C.dymBridgeRegisterURLHandler() }

//export dymBridgeOpenedURL
func dymBridgeOpenedURL(url *C.char) {
	uri := C.GoString(url)
	if !strings.HasPrefix(uri, "dym-bridge://") {
		return
	}
	if path, err := callback.DefaultPath(); err == nil {
		_ = callback.Send(path, uri)
	}
}
