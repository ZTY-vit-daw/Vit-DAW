//go:build !windows && !darwin

package vsphub

import "fmt"

// readSharedMemoryFloat32 has real implementations on windows (Win32 named
// file mappings) and darwin (POSIX shm_open); other platforms keep the
// explicit unsupported error.

func readSharedMemoryFloat32(name string, floatCount int) ([]float32, error) {
	return nil, fmt.Errorf("shared memory materializer unsupported on this platform for %s (%d floats)", name, floatCount)
}
