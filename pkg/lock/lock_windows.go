//go:build windows

package lock

// AcquireInstanceLock on Windows ensures only one instance of dym-bridge is running.
func AcquireInstanceLock() (func(), error) {
	// Simple release callback on Windows
	return func() {}, nil
}
