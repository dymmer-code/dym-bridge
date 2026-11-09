//go:build darwin

package desktop

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"github.com/dymmer-code/dym-bridge/pkg/callback"
	"github.com/go-webgpu/goffi/ffi"
	"strings"
)

//go:embed tray-macos.png
var macTrayIcon []byte

// newTrayImage returns an owned 18-point image with a 2x Retina representation.
func newTrayImage() uintptr {
	data := send(send(cls("NSData"), "alloc"), "initWithBase64EncodedString:options:", str(base64.StdEncoding.EncodeToString(macTrayIcon)), uintptr(0))
	defer send(data, "release")
	return send(send(cls("NSImage"), "alloc"), "initWithData:", data)
}

// Install runs on the Cocoa thread after the window exists. The retained handler
// survives for the application lifetime; Apple events never replace its delegate.
func Install(a Actions) (func(), bool, error) {
	if err := initObjC(); err != nil {
		return nil, false, err
	}
	methods := []struct {
		selector string
		fn       any
	}{
		{selector: "openBridge:", fn: func(_ uintptr, _ uintptr, _ uintptr) { a.Open() }},
		{selector: "connectBridge:", fn: func(_ uintptr, _ uintptr, _ uintptr) { a.Connect() }},
		{selector: "disconnectBridge:", fn: func(_ uintptr, _ uintptr, _ uintptr) { a.Disconnect() }},
		{selector: "sitesBridge:", fn: func(_ uintptr, _ uintptr, _ uintptr) { a.Sites() }},
		{selector: "quitBridge:", fn: func(_ uintptr, _ uintptr, _ uintptr) { a.Quit() }},
		{selector: "handleURL:reply:", fn: func(_ uintptr, _ uintptr, event, reply uintptr) {
			uri := extractURL(event)
			if strings.HasPrefix(uri, "dym-bridge://") {
				go func() {
					if p, err := callback.DefaultPath(); err == nil {
						_ = callback.Send(p, uri)
					}
				}()
			}
		}},
	}
	handlerClass := objcCall("objc_allocateClassPair", cls("NSObject"), "DymBridgeDesktopHandler", uintptr(0))
	if handlerClass == 0 {
		return nil, false, fmt.Errorf("Could not register desktop handler")
	}
	for _, m := range methods {
		encoding := "v@:@"
		if m.selector == "handleURL:reply:" {
			encoding = "v@:@@"
		}
		if objcCall("class_addMethod", handlerClass, sel(m.selector), ffi.NewCallback(m.fn), encoding) == 0 {
			return nil, false, fmt.Errorf("Could not register %s", m.selector)
		}
	}
	objcCall("objc_registerClassPair", handlerClass)
	handler := send(send(handlerClass, "alloc"), "init")
	events := send(cls("NSAppleEventManager"), "sharedAppleEventManager")
	send(events, "setEventHandler:andSelector:forEventClass:andEventID:", handler, sel("handleURL:reply:"), uint32(0x4755524c), uint32(0x4755524c))
	image := newTrayImage()
	if image == 0 {
		send(events, "removeEventHandlerForEventClass:andEventID:", uint32(0x4755524c), uint32(0x4755524c))
		send(handler, "release")
		return nil, false, fmt.Errorf("Could not load tray icon")
	}
	defer send(image, "release")
	bar := send(cls("NSStatusBar"), "systemStatusBar")
	item := send(bar, "statusItemWithLength:", float64(-2))
	send(item, "retain")
	button := send(item, "button")
	send(button, "setImage:", image)
	send(button, "setImagePosition:", uintptr(1))
	send(button, "setAccessibilityLabel:", str("Dym Bridge"))
	send(button, "setToolTip:", str("Dym Bridge — secure files and sites"))
	menu := send(send(cls("NSMenu"), "alloc"), "initWithTitle:", str("Dym Bridge"))
	for _, entry := range []struct{ title, selector string }{{"Open Dym Bridge", "openBridge:"}, {"Sites…", "sitesBridge:"}, {"Connect / Reconnect", "connectBridge:"}, {"Disconnect", "disconnectBridge:"}, {"Quit Dym Bridge", "quitBridge:"}} {
		child := send(send(cls("NSMenuItem"), "alloc"), "initWithTitle:action:keyEquivalent:", str(entry.title), sel(entry.selector), str(""))
		send(child, "setTarget:", handler)
		send(menu, "addItem:", child)
		send(child, "release")
	}
	send(item, "setMenu:", menu)
	send(menu, "release")
	return func() {
		send(events, "removeEventHandlerForEventClass:andEventID:", uint32(0x4755524c), uint32(0x4755524c))
		send(bar, "removeStatusItem:", item)
		send(item, "release")
		send(handler, "release")
	}, true, nil
}

func extractURL(event uintptr) string {
	value := send(send(event, "paramDescriptorForKeyword:", uint32(0x2d2d2d2d)), "stringValue")
	return nativeString(send(value, "UTF8String"))
}
