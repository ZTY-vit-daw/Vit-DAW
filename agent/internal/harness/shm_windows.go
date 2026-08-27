//go:build windows

package harness

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modkernel32       = syscall.NewLazyDLL("kernel32.dll")
	procOpenFileMapA  = modkernel32.NewProc("OpenFileMappingA")
	procMapViewOfFile = modkernel32.NewProc("MapViewOfFile")
	procUnmapView     = modkernel32.NewProc("UnmapViewOfFile")
	procCloseHandle   = modkernel32.NewProc("CloseHandle")
)

const fileMapRead = 0x0004

func readSharedFloat32Array(memoryName string, floatCount int) ([]float32, error) {
	memoryName = strings.TrimSpace(memoryName)
	if memoryName == "" || floatCount <= 0 {
		return nil, fmt.Errorf("shared memory name and float_count are required")
	}
	namePtr, err := syscall.BytePtrFromString(memoryName)
	if err != nil {
		return nil, err
	}
	handle, _, err := procOpenFileMapA.Call(uintptr(fileMapRead), uintptr(0), uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		return nil, firstSyscallError(err, "OpenFileMappingA failed")
	}
	defer procCloseHandle.Call(handle)
	byteCount := uintptr(floatCount * 4)
	view, _, err := procMapViewOfFile.Call(handle, uintptr(fileMapRead), 0, 0, byteCount)
	if view == 0 {
		return nil, firstSyscallError(err, "MapViewOfFile failed")
	}
	defer procUnmapView.Call(view)
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(view)), int(byteCount))
	out := make([]float32, floatCount)
	for i := range out {
		bits := binary.LittleEndian.Uint32(bytes[i*4 : i*4+4])
		out[i] = math.Float32frombits(bits)
	}
	return out, nil
}

func firstSyscallError(err error, fallback string) error {
	if err != nil && !errors.Is(err, syscall.Errno(0)) {
		return err
	}
	return fmt.Errorf("%s", fallback)
}
