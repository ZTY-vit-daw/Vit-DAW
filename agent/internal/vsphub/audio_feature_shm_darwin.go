//go:build darwin

package vsphub

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

// readSharedMemoryFloat32 is the darwin counterpart of
// audio_feature_shm_windows.go: the kernel publishes audio-feature telemetry
// segments as POSIX shm_open objects on macOS instead of Win32 named file
// mappings. Validation, fail-closed open, undersized-segment rejection and
// offset-0 little-endian decode mirror the Windows sibling.
func readSharedMemoryFloat32(name string, floatCount int) ([]float32, error) {
	if name == "" || floatCount <= 0 {
		return nil, fmt.Errorf("invalid shared memory request")
	}
	posix := posixShmName(name)
	namePtr, err := syscall.BytePtrFromString(posix)
	if err != nil {
		return nil, err
	}
	// Read-only descriptor so the materializer can never mutate a kernel segment.
	fd, _, errno := syscall.Syscall(syscall.SYS_SHM_OPEN,
		uintptr(unsafe.Pointer(namePtr)), uintptr(syscall.O_RDONLY), 0)
	if errno != 0 {
		return nil, errno
	}
	defer syscall.Close(int(fd))

	byteCount := floatCount * 4
	var st syscall.Stat_t
	if err := syscall.Fstat(int(fd), &st); err != nil {
		return nil, fmt.Errorf("fstat shared memory %s failed: %w", posix, err)
	}
	// MapViewOfFile fails when the requested range exceeds the segment; POSIX
	// mmap would map beyond the object and SIGBUS on access, so the size check
	// must precede the mapping to keep the fail-closed semantics. darwin
	// rounds shm storage up to 16 KiB granularity and fstat reports the
	// rounded extent: logical sizes below that are not observable here and
	// stay guarded by the caller's float_count consistency check.
	if st.Size < int64(byteCount) {
		return nil, fmt.Errorf("shared memory %s too small: have %d bytes, need %d", posix, st.Size, byteCount)
	}
	view, err := syscall.Mmap(int(fd), 0, byteCount, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap shared memory %s failed: %w", posix, err)
	}
	defer syscall.Munmap(view)

	out := make([]float32, floatCount)
	for i := range out {
		bits := binary.LittleEndian.Uint32(view[i*4 : i*4+4])
		out[i] = math.Float32frombits(bits)
	}
	return out, nil
}

// posixShmName normalises the kernel-published segment name to the POSIX
// shm_open convention (leading slash); the kernel publishes unprefixed
// "Vit_AudioFeature_*" names shared with the Win32 reader.
func posixShmName(name string) string {
	return "/" + strings.TrimPrefix(name, "/")
}
