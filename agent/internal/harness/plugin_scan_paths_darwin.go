//go:build darwin

package harness

import (
	"os"
	"path/filepath"
)

// defaultPluginScanPaths returns the macOS VST3 standard locations:
// per-user ~/Library/Audio/Plug-Ins/VST3 and system-wide
// /Library/Audio/Plug-Ins/VST3 (VST3 SDK Interface Documentation).
// VST_ENABLE_USER_PLUGIN_DIR=0 changes the user dir only, which we do not
// model; callers may pass explicit paths to override defaults entirely.
func defaultPluginScanPaths() []string {
	home, err := os.UserHomeDir()
	paths := make([]string, 0, 2)
	if err == nil && home != "" {
		paths = append(paths, filepath.Join(home, "Library", "Audio", "Plug-Ins", "VST3"))
	}
	return append(paths, "/Library/Audio/Plug-Ins/VST3")
}
