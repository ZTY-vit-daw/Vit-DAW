//go:build darwin || windows

package harness

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPluginScanPathsPlatformDefaults(t *testing.T) {
	paths := defaultPluginScanPaths()
	if len(paths) == 0 {
		t.Fatalf("defaultPluginScanPaths returned no paths")
	}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" || !filepath.IsAbs(p) {
			t.Errorf("default scan path %q is not absolute", p)
		}
	}
}
