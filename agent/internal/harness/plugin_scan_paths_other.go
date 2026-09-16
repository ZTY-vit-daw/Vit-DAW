//go:build !windows && !darwin

package harness

// defaultPluginScanPaths has no platform default on this OS; returning nil
// lets the kernel-side scanner apply its own defaults instead of silently
// scanning nothing.
func defaultPluginScanPaths() []string {
	return nil
}
