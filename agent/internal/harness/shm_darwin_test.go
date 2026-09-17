//go:build darwin

package harness

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

// TestReadSharedFloat32ArrayRoundTrip writes known float bit patterns into a
// real POSIX shm segment and reads them back through the production reader
// using the unprefixed name the kernel publishes.
func TestReadSharedFloat32ArrayRoundTrip(t *testing.T) {
	name := shmTestName(t, "roundtrip")
	want := []float32{0, 1, -1.5, math.MaxFloat32, float32(math.SmallestNonzeroFloat64), 1e-10, -1e10, 0.25}
	view := shmCreateForTest(t, name, len(want)*4)
	for i, v := range want {
		binary.LittleEndian.PutUint32(view[i*4:], math.Float32bits(v))
	}
	got, err := readSharedFloat32Array(name, len(want))
	if err != nil {
		t.Fatalf("readSharedFloat32Array(%s, %d) failed: %v", name, len(want), err)
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

// TestReadSharedFloat32ArrayAcceptsPrefixedName verifies the name
// normalisation: a caller passing the already-normalised "/name" form reads
// the same segment the kernel publishes unprefixed.
func TestReadSharedFloat32ArrayAcceptsPrefixedName(t *testing.T) {
	name := shmTestName(t, "prefixed")
	view := shmCreateForTest(t, name, 4)
	binary.LittleEndian.PutUint32(view, math.Float32bits(7.5))
	got, err := readSharedFloat32Array("/"+name, 1)
	if err != nil {
		t.Fatalf("readSharedFloat32Array with leading slash failed: %v", err)
	}
	if math.Float32bits(got[0]) != math.Float32bits(7.5) {
		t.Fatalf("got %v, want 7.5", got[0])
	}
}

// TestReadSharedFloat32ArrayTrimsName mirrors the Windows sibling's
// TrimSpace behaviour: surrounding whitespace must not break the lookup.
func TestReadSharedFloat32ArrayTrimsName(t *testing.T) {
	name := shmTestName(t, "trim")
	view := shmCreateForTest(t, name, 4)
	binary.LittleEndian.PutUint32(view, math.Float32bits(-2.5))
	got, err := readSharedFloat32Array("  "+name+"\t", 1)
	if err != nil {
		t.Fatalf("readSharedFloat32Array with padded name failed: %v", err)
	}
	if math.Float32bits(got[0]) != math.Float32bits(-2.5) {
		t.Fatalf("got %v, want -2.5", got[0])
	}
}

// TestReadSharedFloat32ArrayMissingSegment is the fail-closed path: a segment
// that was never created must surface an error, never zero-filled data.
func TestReadSharedFloat32ArrayMissingSegment(t *testing.T) {
	name := shmTestName(t, "missing")
	got, err := readSharedFloat32Array(name, 4)
	if err == nil {
		t.Fatalf("expected fail-closed error for missing segment %s", name)
	}
	if got != nil {
		t.Fatalf("expected nil data on missing segment, got %v", got)
	}
}

// TestReadSharedFloat32ArrayRejectsRangeBeyondSegment mirrors the Windows
// MapViewOfFile behaviour when the requested range exceeds the segment: an
// explicit error instead of an over-mapping that would fault on access.
// macOS rounds segment storage up to 16 KiB granularity, so the mismatch is
// driven past that boundary: a segment whose logical size is 2 floats cannot
// serve a request larger than the rounded object.
func TestReadSharedFloat32ArrayRejectsRangeBeyondSegment(t *testing.T) {
	name := shmTestName(t, "under")
	shmCreateForTest(t, name, 8)                                  // logical 2 floats; macOS rounds storage to 16 KiB
	if _, err := readSharedFloat32Array(name, 4200); err == nil { // 16800 bytes > 16 KiB
		t.Fatal("expected error when requesting a range beyond the segment object")
	}
}

// TestReadSharedFloat32ArraySubGranularityMismatchIsInvisible documents a
// macOS platform fact: shm storage rounds up to 16 KiB, so a request that
// exceeds the producer's logical size but stays within the rounded object
// succeeds and reads zero-fill past the logical end. Windows rejects this via
// MapViewOfFile; on darwin the logical-size guard belongs to the callers
// (float_count/stride consistency checks in harness.go), not the reader.
func TestReadSharedFloat32ArraySubGranularityMismatchIsInvisible(t *testing.T) {
	name := shmTestName(t, "sub16k")
	shmCreateForTest(t, name, 8) // logical 2 floats only
	got, err := readSharedFloat32Array(name, 3)
	if err != nil {
		t.Fatalf("expected the sub-16KiB over-read to succeed on darwin, got: %v", err)
	}
	if math.Float32bits(got[2]) != 0 {
		t.Fatalf("expected zero-fill past the logical end, got %v (bits %#x)", got[2], math.Float32bits(got[2]))
	}
}

func TestReadSharedFloat32ArrayInvalidRequest(t *testing.T) {
	cases := []struct {
		name       string
		floatCount int
	}{
		{"", 4},
		{"   ", 4},
		{shmTestName(t, "zero"), 0},
		{shmTestName(t, "neg"), -1},
	}
	for i, c := range cases {
		if got, err := readSharedFloat32Array(c.name, c.floatCount); err == nil || got != nil {
			t.Fatalf("case %d (%q, %d): expected validation error, got data=%v err=%v",
				i, c.name, c.floatCount, got, err)
		}
	}
}
