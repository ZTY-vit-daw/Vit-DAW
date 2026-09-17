//go:build darwin

package harness

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

// readSharedFloat32Array is the darwin counterpart of shm_windows.go: the
// kernel publishes its audio-feature segments as POSIX shm_open objects on
// macOS instead of Win32 named file mappings. Behaviour mirrors the Windows
// implementation: explicit validation error, fail-closed when the segment is
// missing, fail-closed when it is smaller than the requested range, and a
// little-endian decode from offset 0 that does not depend on host alignment.
func readSharedFloat32Array(memoryName string, floatCount int) ([]float32, error) {
	memoryName = strings.TrimSpace(memoryName)
	if memoryName == "" || floatCount <= 0 {
		return nil, fmt.Errorf("shared memory name and float_count are required")
	}
	name := posixShmName(memoryName)
	namePtr, err := syscall.BytePtrFromString(name)
	if err != nil {
		return nil, err
	}
	// Read-only descriptor so the reader can never mutate a kernel segment.
	fd, _, errno := syscall.Syscall(syscall.SYS_SHM_OPEN,
		uintptr(unsafe.Pointer(namePtr)), uintptr(syscall.O_RDONLY), 0)
	if errno != 0 {
		return nil, errno
	}
	defer syscall.Close(int(fd))

	byteCount := floatCount * 4
	var st syscall.Stat_t
	if err := syscall.Fstat(int(fd), &st); err != nil {
		return nil, fmt.Errorf("fstat shared memory %s failed: %w", name, err)
	}
	// MapViewOfFile fails when the requested range exceeds the segment; POSIX
	// mmap would map beyond the object and SIGBUS on access, so the size check
	// must precede the mapping to keep the fail-closed semantics. darwin
	// rounds shm storage up to 16 KiB granularity and fstat reports the
	// rounded extent: logical sizes below that are not observable here and
	// stay guarded by the callers' float_count/stride consistency checks.
	if st.Size < int64(byteCount) {
		return nil, fmt.Errorf("shared memory %s too small: have %d bytes, need %d", name, st.Size, byteCount)
	}
	view, err := syscall.Mmap(int(fd), 0, byteCount, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap shared memory %s failed: %w", name, err)
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
// shm_open convention. The kernel publishes unprefixed names
// ("Vit_AudioFeature_*", exactly what the Windows side opens via
// OpenFileMappingA); the same published name must resolve on both platforms.
func posixShmName(name string) string {
	return "/" + strings.TrimPrefix(name, "/")
}
