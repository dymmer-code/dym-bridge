//go:build !windows

package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var lockFile *os.File

// AcquireInstanceLock ensures only one instance of dym-bridge is running per user session.
func AcquireInstanceLock() (func(), error) {
	tempDir := os.TempDir()
	lockPath := filepath.Join(tempDir, "dym-bridge.lock")

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return func() {}, fmt.Errorf("no se pudo abrir el archivo de bloqueo: %w", err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		var pid int
		_, _ = fmt.Fscanf(file, "%d", &pid)
		_ = file.Close()
		if pid > 0 {
			return func() {}, fmt.Errorf("dym-bridge ya está ejecutándose (PID: %d)", pid)
		}
		return func() {}, errors.New("dym-bridge ya está ejecutándose")
	}

	_ = file.Truncate(0)
	_, _ = file.Seek(0, 0)
	_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
	_ = file.Sync()

	lockFile = file
	release := func() {
		if lockFile != nil {
			_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
			_ = lockFile.Close()
			_ = os.Remove(lockPath)
			lockFile = nil
		}
	}

	return release, nil
}
