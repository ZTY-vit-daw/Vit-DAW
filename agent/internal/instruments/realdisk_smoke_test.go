package instruments

import (
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/pluginsemantics"
)

// TestRealDiskSmoke is an opt-in diagnostic for real-machine evidence runs
// (SRC-PRESET-AGENT-1 leg A receipt). It skips unless
// VIT_INSTRUMENTS_REAL_SMOKE=1 so the default suite never depends on the
// local Surge XT install or the user's plugin semantic index.
func TestRealDiskSmoke(t *testing.T) {
	if os.Getenv("VIT_INSTRUMENTS_REAL_SMOKE") != "1" {
		t.Skip("set VIT_INSTRUMENTS_REAL_SMOKE=1 to run the real-disk diagnostic")
	}
	root := DefaultFactoryRoot()
	if root == "" {
		t.Skip("no default Surge factory root on this machine")
	}
	index, err := BuildFactoryIndex(root)
	if err != nil {
		t.Fatalf("real factory index: %v", err)
	}
	t.Logf("factory root=%s patches=%d invalid=%d", root, len(index.Patches), index.InvalidCount)
	for _, row := range index.CategoryCounts() {
		t.Logf("category %-14s count=%-4d invalid=%d semantics=%v", row.Category, row.Count, row.Invalid, row.Semantics)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home: %v", err)
	}
	semIdx, err := pluginsemantics.Load(filepath.Join(home, ".vit", "plugin_semantics.json"))
	if err != nil {
		t.Skipf("plugin semantic index unavailable: %v", err)
	}
	installed := FromPluginSemantics(semIdx.Entries)
	t.Logf("semantic index instruments=%d", len(installed))

	queries := []string{"这曲用钢琴", "电钢琴", "贝斯", "鼓", "弦乐", "gm:82", "氛围 pad", "水果沙拉"}
	for _, query := range queries {
		result := Match(query, installed, index)
		if result.Degrade != nil {
			t.Logf("query=%q class=%q DEGRADE[%s] %s", query, result.Class, result.Degrade.Code, result.Degrade.Message)
			continue
		}
		for _, candidate := range result.Candidates {
			t.Logf("query=%q class=%q -> %s (%s) evidence=%s", query, result.Class, candidate.Installed.Name, candidate.Kind, candidate.Evidence)
		}
		for _, hint := range result.PresetHints {
			t.Logf("query=%q hint %s count=%d approximate=%v examples=%v", query, hint.Category, hint.Count, hint.Approximate, hint.Examples)
		}
	}
}
