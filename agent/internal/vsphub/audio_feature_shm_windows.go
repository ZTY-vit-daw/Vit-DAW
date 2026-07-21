//go:build windows

package vsphub

import (
	"fmt"
	"syscall"
	"unsafe"
)

const fileMapRead = 0x0004

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procOpenFileMappingW = kernel32.NewProc("OpenFileMappingW")
	procMapViewOfFile    = kernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile  = kernel32.NewProc("UnmapViewOfFile")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
)

func readSharedMemoryFloat32(name string, floatCount int) ([]float32, error) {
	if name == "" || floatCount <= 0 {
		return nil, fmt.Errorf("invalid shared memory request")
	}
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, _, openErr := procOpenFileMappingW.Call(
		uintptr(fileMapRead),
		0,
		uintptr(unsafe.Pointer(namePtr)),
	)
	if handle == 0 {
		return nil, fmt.Errorf("OpenFileMappingW failed for %s: %w", name, openErr)
	}
	defer procCloseHandle.Call(handle)

	byteCount := uintptr(floatCount * 4)
	view, _, mapErr := procMapViewOfFile.Call(
		handle,
		uintptr(fileMapRead),
		0,
		0,
		byteCount,
	)
	if view == 0 {
		return nil, fmt.Errorf("MapViewOfFile failed for %s: %w", name, mapErr)
	}
	defer procUnmapViewOfFile.Call(view)

	source := unsafe.Slice((*float32)(unsafe.Pointer(view)), floatCount)
	out := make([]float32, floatCount)
	copy(out, source)
	return out, nil
}
