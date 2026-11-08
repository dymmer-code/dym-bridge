//go:build darwin

package desktop

import (
	"fmt"
	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
	"runtime"
	"unsafe"
)

var objcLib unsafe.Pointer

func initObjC() error {
	var err error
	objcLib, err = ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")
	return err
}

// objcCall handles the scalar signatures used by the URL receiver and status
// menu. No structs, variadics, or ownership-transferring Go pointers cross FFI.
func objcCall(symbol string, args ...any) uintptr {
	fn, err := ffi.GetSymbol(objcLib, symbol)
	if err != nil {
		panic(err)
	}
	values := make([]uint64, len(args))
	argv := make([]unsafe.Pointer, len(args))
	kinds := make([]*types.TypeDescriptor, len(args))
	keep := make([][]byte, 0)
	for i, arg := range args {
		kinds[i] = types.UInt64TypeDescriptor
		switch v := arg.(type) {
		case uintptr:
			values[i] = uint64(v)
		case uint32:
			values[i] = uint64(v)
			kinds[i] = types.UInt32TypeDescriptor
		case float64:
			values[i] = *(*uint64)(unsafe.Pointer(&v))
			kinds[i] = types.DoubleTypeDescriptor
		case string:
			bytes := append([]byte(v), 0)
			keep = append(keep, bytes)
			values[i] = uint64(uintptr(unsafe.Pointer(&bytes[0])))
			kinds[i] = types.PointerTypeDescriptor
		default:
			panic(fmt.Sprintf("unsupported Objective-C argument %T", arg))
		}
		argv[i] = unsafe.Pointer(&values[i])
	}
	cif := new(types.CallInterface)
	if err = ffi.PrepareCallInterface(cif, types.DefaultCall, types.UInt64TypeDescriptor, kinds); err != nil {
		panic(err)
	}
	var result uintptr
	if _, err = ffi.CallFunction(cif, fn, unsafe.Pointer(&result), argv); err != nil {
		panic(err)
	}
	runtime.KeepAlive(keep)
	return result
}
func sel(s string) uintptr { return objcCall("sel_registerName", s) }
func send(id uintptr, selector string, args ...any) uintptr {
	return objcCall("objc_msgSend", append([]any{id, sel(selector)}, args...)...)
}
func cls(s string) uintptr { return objcCall("objc_getClass", s) }
func str(s string) uintptr { return send(cls("NSString"), "stringWithUTF8String:", s) }
func nativeString(value uintptr) string {
	if value == 0 {
		return ""
	}
	ptr := *(*unsafe.Pointer)(unsafe.Pointer(&value))
	bytes := make([]byte, 0, 256)
	for i := 0; i < 16384; i++ {
		c := *(*byte)(unsafe.Add(ptr, i))
		if c == 0 {
			return string(bytes)
		}
		bytes = append(bytes, c)
	}
	return ""
}
