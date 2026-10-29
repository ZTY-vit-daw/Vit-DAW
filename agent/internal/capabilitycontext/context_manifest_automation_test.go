package capabilitycontext

import (
	"strings"
	"testing"
)

// M3-C4-AUTO-DISCLOSE-1 披露契约锁定：automation 不再是四视图排除项，
// 且每视图 Guidance 披露 automation.* 工具面与轨级 Volume & Pan 寻址。
// 该面是显式披露行为变更，不得静默回退。
func TestAutomationNotExcludedAndDisclosedInStaticMixManifests(t *testing.T) {
	manifests := []struct {
		name        string
		manifest    ContextManifest
		wantTrackFn bool // 断言轨级 Volume & Pan 寻址在 Guidance 中
	}{
		{name: "B2 static_balance", manifest: StaticMixStaticBalanceContextManifest(), wantTrackFn: true},
		{name: "B3 pan_layout", manifest: StaticMixPanLayoutContextManifest(), wantTrackFn: true},
		{name: "B4 low_end_relation", manifest: StaticMixLowEndRelationContextManifest()},
		{name: "C1 frequency_cleanup", manifest: FineMixFrequencyCleanupContextManifest()},
	}
	for _, item := range manifests {
		for _, excluded := range item.manifest.Excluded {
			if strings.EqualFold(strings.TrimSpace(excluded), "automation") {
				t.Fatalf("%s: Excluded still contains %q; automation disclosure must not regress", item.name, excluded)
			}
		}
		var guidanceHit bool
		for _, line := range item.manifest.Guidance {
			if strings.Contains(line, "automation.set_points") && strings.Contains(line, "automation.get_curve") && strings.Contains(line, "automation.clear_region") {
				guidanceHit = true
				if item.wantTrackFn && !strings.Contains(line, "Volume & Pan") {
					t.Fatalf("%s: automation guidance line lacks track-level Volume & Pan addressing: %s", item.name, line)
				}
				break
			}
		}
		if !guidanceHit {
			t.Fatalf("%s: no Guidance line discloses the automation.* tool surface", item.name)
		}
	}
}
