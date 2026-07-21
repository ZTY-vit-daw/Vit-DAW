//go:build !windows

package vsphub

import "fmt"

func readSharedMemoryFloat32(name string, floatCount int) ([]float32, error) {
	return nil, fmt.Errorf("shared memory materializer unsupported on this platform for %s (%d floats)", name, floatCount)
}
