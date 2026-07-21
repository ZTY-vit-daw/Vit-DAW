//go:build windows

package orchestration

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const errorSharingViolation syscall.Errno = 32

func acquireStoreFileLock(storePath string) (func(), error) {
	lockPath := storePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create orchestration lock directory: %w", err)
	}
	path, err := syscall.UTF16PtrFromString(lockPath)
	if err != nil {
		return nil, fmt.Errorf("encode orchestration lock path: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		handle, openErr := syscall.CreateFile(
			path,
			syscall.GENERIC_READ|syscall.GENERIC_WRITE,
			0, // no sharing: the kernel releases authority when this handle closes
			nil,
			syscall.OPEN_ALWAYS,
			syscall.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if openErr == nil {
			return func() { _ = syscall.CloseHandle(handle) }, nil
		}
		if openErr != errorSharingViolation && openErr != syscall.ERROR_ACCESS_DENIED {
			return nil, fmt.Errorf("acquire orchestration store lock: %w", openErr)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("acquire orchestration store lock timed out: %w", openErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
