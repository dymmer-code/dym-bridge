//go:build darwin

package desktop

import (
	"github.com/go-webgpu/goffi/ffi"
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
