//go:build darwin

package vsphub

import (
	"encoding/binary"
	"math"
	"os"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"unsafe"
)

var shmTestSeq atomic.Int64

// shmCreateForTest creates a POSIX shm segment of exactly size bytes and
// returns a read-write mapping of it. The segment is unlinked and the mapping
// released on test cleanup, so nothing leaks into the machine's shm namespace.
func shmCreateForTest(t *testing.T, name string, size int) []byte {
	t.Helper()
	posix := posixShmName(name)
	namePtr, err := syscall.BytePtrFromString(posix)
	if err != nil {
		t.Fatalf("shm_open name %s: %v", posix, err)
	}
	fd, _, errno := syscall.Syscall(syscall.SYS_SHM_OPEN,
		uintptr(unsafe.Pointer(namePtr)), uintptr(syscall.O_CREAT|syscall.O_RDWR), 0o600)
	if errno != 0 {
		t.Fatalf("shm_open create %s: %v", posix, errno)
	}
	t.Cleanup(func() {
		syscall.Syscall(syscall.SYS_SHM_UNLINK, uintptr(unsafe.Pointer(namePtr)), 0, 0)
	})
	if err := syscall.Ftruncate(int(fd), int64(size)); err != nil {
		syscall.Close(int(fd))
		t.Fatalf("ftruncate %s to %d bytes: %v", posix, size, err)
	}
	view, err := syscall.Mmap(int(fd), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	syscall.Close(int(fd))
	if err != nil {
		t.Fatalf("mmap %s: %v", posix, err)
	}
	t.Cleanup(func() { syscall.Munmap(view) })
	return view
}

// shmTestName builds a unique segment name within macOS's hard 31-char
// shm_open limit (leading slash included), so tags must stay short.
func shmTestName(t *testing.T, tag string) string {
	t.Helper()
	return "vshm_" + tag +
		"_" + strconv.Itoa(os.Getpid()) + "_" + strconv.Itoa(int(shmTestSeq.Add(1)))
}

// TestReadSharedMemoryFloat32RoundTrip writes known float bit patterns into a
// real POSIX shm segment and reads them back through the materializer reader
// using the unprefixed name the kernel publishes.
func TestReadSharedMemoryFloat32RoundTrip(t *testing.T) {
	name := shmTestName(t, "roundtrip")
	want := []float32{0, 1, -1.5, math.MaxFloat32, float32(math.SmallestNonzeroFloat64), 1e-10, -1e10, 0.25}
	view := shmCreateForTest(t, name, len(want)*4)
	for i, v := range want {
		binary.LittleEndian.PutUint32(view[i*4:], math.Float32bits(v))
	}
	got, err := readSharedMemoryFloat32(name, len(want))
	if err != nil {
		t.Fatalf("readSharedMemoryFloat32(%s, %d) failed: %v", name, len(want), err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d floats, want %d", len(got), len(want))
	}
	for i := range want {
		if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
			t.Fatalf("float %d: got %v (bits %#x), want %v (bits %#x)",
				i, got[i], math.Float32bits(got[i]), want[i], math.Float32bits(want[i]))
		}
	}
}

// TestReadSharedMemoryFloat32MissingSegment is the fail-closed path: a
// segment that was never created must surface an error, never zero-filled
// data (the caller counts these as shmReadMiss).
func TestReadSharedMemoryFloat32MissingSegment(t *testing.T) {
	name := shmTestName(t, "missing")
	got, err := readSharedMemoryFloat32(name, 4)
	if err == nil {
		t.Fatalf("expected fail-closed error for missing segment %s", name)
	}
	if got != nil {
		t.Fatalf("expected nil data on missing segment, got %v", got)
	}
}

// TestReadSharedMemoryFloat32RejectsRangeBeyondSegment mirrors the Windows
// MapViewOfFile behaviour when the requested range exceeds the segment: an
// explicit error instead of an over-mapping that would fault on access.
// macOS rounds segment storage up to 16 KiB granularity, so the mismatch is
// driven past that boundary.
func TestReadSharedMemoryFloat32RejectsRangeBeyondSegment(t *testing.T) {
	name := shmTestName(t, "under")
	shmCreateForTest(t, name, 8)                                   // logical 2 floats; macOS rounds storage to 16 KiB
	if _, err := readSharedMemoryFloat32(name, 4200); err == nil { // 16800 bytes > 16 KiB
		t.Fatal("expected error when requesting a range beyond the segment object")
	}
}

func TestReadSharedMemoryFloat32InvalidRequest(t *testing.T) {
	cases := []struct {
		name       string
		floatCount int
	}{
		{"", 4},
		{shmTestName(t, "zero"), 0},
		{shmTestName(t, "neg"), -1},
	}
	for i, c := range cases {
		if got, err := readSharedMemoryFloat32(c.name, c.floatCount); err == nil || got != nil {
			t.Fatalf("case %d (%q, %d): expected validation error, got data=%v err=%v",
				i, c.name, c.floatCount, got, err)
		}
	}
}
