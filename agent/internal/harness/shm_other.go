//go:build !windows

package harness

import "fmt"

// readSharedFloat32Array reads the kernel's Win32 named shared-memory
// segments (waveform/spectrogram bakers), which only exist on the Windows
// build. Other platforms degrade to an explicit unsupported error instead of
// a compile-time dependency on kernel32.
func readSharedFloat32Array(memoryName string, floatCount int) ([]float32, error) {
	return nil, fmt.Errorf("shared memory feature segments are unsupported on this platform")
}
