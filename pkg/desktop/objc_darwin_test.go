//go:build darwin

package desktop

import (
	"bytes"
	"github.com/go-webgpu/goffi/ffi"
	"image/png"
	"runtime"
	"testing"
)

func TestNativeURLDescriptor(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initObjC(); err != nil {
		t.Fatal(err)
	}
	if _, err := ffi.LoadLibrary("/System/Library/Frameworks/Foundation.framework/Foundation"); err != nil {
		t.Fatal(err)
	}
	pool := send(send(cls("NSAutoreleasePool"), "alloc"), "init")
	defer send(pool, "drain")
	uri := "dym-bridge://callback?code=local-test&state=state-test"
	descriptor := send(cls("NSAppleEventDescriptor"), "descriptorWithString:", str(uri))
	event := send(cls("NSAppleEventDescriptor"), "recordDescriptor")
	send(event, "setParamDescriptor:forKeyword:", descriptor, uint32(0x2d2d2d2d))
	if got := extractURL(event); got != uri {
		t.Fatalf("callback URL = %q", got)
	}
	if got := extractURL(0); got != "" {
		t.Fatalf("empty event = %q", got)
	}
}

func TestTrayImageLoadsAtMenuBarSize(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initObjC(); err != nil {
		t.Fatal(err)
	}
	if _, err := ffi.LoadLibrary("/System/Library/Frameworks/AppKit.framework/AppKit"); err != nil {
		t.Fatal(err)
	}
	pool := send(send(cls("NSAutoreleasePool"), "alloc"), "init")
	defer send(pool, "drain")
	image := newTrayImage()
	if image == 0 {
		t.Fatal("tray icon failed to decode")
	}
	defer send(image, "release")
	if send(image, "isTemplate") == 0 {
		t.Fatal("tray icon must use native monochrome tint")
	}
	if send(image, "isValid") == 0 {
		t.Fatal("invalid tray image")
	}
	size := nativeString(send(send(send(image, "valueForKey:", str("size")), "description"), "UTF8String"))
	if size != "NSSize: {18, 18}" {
		t.Fatalf("tray logical size = %s", size)
	}
}

func TestTrayGlyphIsTransparentAndFillsItsCanvas(t *testing.T) {
	image, err := png.Decode(bytes.NewReader(macTrayIcon))
	if err != nil {
		t.Fatal(err)
	}
	minX, minY, maxX, maxY, count := 36, 36, 0, 0, 0
	for y := 0; y < 36; y++ {
		for x := 0; x < 36; x++ {
			r, g, b, a := image.At(x, y).RGBA()
			if r != g || g != b {
				t.Fatal("tray glyph must be monochrome")
			}
			if a > 32767 {
				count++
				minX = min(minX, x)
				minY = min(minY, y)
				maxX = max(maxX, x)
				maxY = max(maxY, y)
			}
		}
	}
	_, _, _, corner := image.At(0, 0).RGBA()
	if corner != 0 || count < 200 || count > 1000 {
		t.Fatal("expected transparent background and visible bridge glyph", count)
	}
	if maxX-minX < 31 || maxY-minY < 31 {
		t.Fatalf("glyph too small: %d x %d", maxX-minX+1, maxY-minY+1)
	}
}
