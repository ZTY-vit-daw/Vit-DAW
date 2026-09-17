//go:build !windows && !darwin

package harness

import "fmt"

// readSharedFloat32Array reads the kernel's audio-feature shared-memory
// segments. Windows opens Win32 named file mappings (shm_windows.go); darwin
// opens POSIX shm_open objects (shm_darwin.go). Other platforms degrade to an
// explicit unsupported error instead of a silent empty result.
func readSharedFloat32Array(memoryName string, floatCount int) ([]float32, error) {
	return nil, fmt.Errorf("shared memory feature segments are unsupported on this platform")
}
