//go:build windows

package harness

import (
	"os"
	"path/filepath"
	"strings"
)

// defaultPluginScanPaths returns the Windows VST3 common-folder defaults.
// The VST3 SDK standard location is %CommonProgramFiles%\VST3; the env
// overrides exist because Go cannot read the WOW64-redirected view.
func defaultPluginScanPaths() []string {
	if commonProgramFiles := strings.TrimSpace(os.Getenv("CommonProgramFiles")); commonProgramFiles != "" {
		return []string{filepath.Join(commonProgramFiles, "VST3")}
	}
	if programFiles := strings.TrimSpace(os.Getenv("ProgramFiles")); programFiles != "" {
		return []string{filepath.Join(programFiles, "Common Files", "VST3")}
	}
	return []string{`C:\Program Files\Common Files\VST3`}
}
