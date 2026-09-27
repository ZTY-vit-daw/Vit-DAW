package tim

import (
	"testing"
)

// TIM-KERNEL-HYGIENE-1 red-first tests: AS-SIG P3 (DC offset, signed mean of
// finite samples) and the plugin-list hygiene assertion consuming the kernel
// get_project_state plugin_list_hygiene block (GAPS Item 2 + Item 5).
//
// Red state: before the asserter emits these rows, Build produces no
// signal_hygiene/dc_offset or plugin_hygiene rows and the helpers below fail.

func TestBuildAssertionDCOffsetFailAndPass(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 2,
		"tracks": []any{
			map[string]any{
				"track_id": "t-biased", "track_name": "Biased", "clip_count": 1,
				"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
				"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -6.0, "dc_offset": 0.05},
			},
			map[string]any{
				"track_id": "t-clean", "track_name": "Clean", "clip_count": 1,
				"primary_clip": map[string]any{"clip_id": "c2", "current_source_path": "/a/c.wav"},
				"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -6.0, "dc_offset": 0.001},
			},
		},
	}})
	biased := assertionResultBy(t, proj, "signal_hygiene", "dc_offset", "t-biased")
	if biased.Status != AssertionStatusFail {
		t.Fatalf("dc_offset status = %s, want fail: %#v", biased.Status, biased)
	}
	if biased.Code != "assert_signal_dc_offset" {
		t.Fatalf("dc_offset code = %s, want assert_signal_dc_offset", biased.Code)
	}
	if biased.Value == nil || *biased.Value != 0.05 {
		t.Fatalf("dc_offset value = %v, want 0.05 (absolute)", biased.Value)
	}
	if biased.Threshold == nil || *biased.Threshold != SignalDCOffsetWarnLinear {
		t.Fatalf("dc_offset threshold = %v, want %v (exposed constant)", biased.Threshold, SignalDCOffsetWarnLinear)
	}

	clean := assertionResultBy(t, proj, "signal_hygiene", "dc_offset", "t-clean")
	if clean.Status != AssertionStatusPass {
		t.Fatalf("dc_offset status = %s, want pass: %#v", clean.Status, clean)
	}
	if clean.Threshold == nil || *clean.Threshold != SignalDCOffsetWarnLinear {
		t.Fatalf("pass row must still expose the threshold constant")
	}
}

func TestBuildAssertionDCOffsetNegativeBiasUsesAbsoluteValue(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t-neg", "track_name": "Neg", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			"acoustic":     map[string]any{"status": "ready", "dc_offset": -0.05},
		}},
	}})
	row := assertionResultBy(t, proj, "signal_hygiene", "dc_offset", "t-neg")
	if row.Status != AssertionStatusFail {
		t.Fatalf("negative dc_offset status = %s, want fail (|v| over threshold)", row.Status)
	}
	if row.Value == nil || *row.Value != 0.05 {
		t.Fatalf("value = %v, want absolute 0.05", row.Value)
	}
}

func TestBuildAssertionDCOffsetThresholdBoundary(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t-at-limit", "track_name": "AtLimit", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			"acoustic":     map[string]any{"status": "ready", "dc_offset": SignalDCOffsetWarnLinear},
		}},
	}})
	row := assertionResultBy(t, proj, "signal_hygiene", "dc_offset", "t-at-limit")
	if row.Status != AssertionStatusPass {
		t.Fatalf("dc_offset exactly at the threshold must pass (fail is strictly above): %#v", row)
	}
}

func TestBuildAssertionDCOffsetNotEvaluableWithoutKey(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "t1", "track_name": "NoKey", "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			"acoustic":     map[string]any{"status": "ready", "peak_dbfs": -6.0},
		}},
	}})
	row := assertionResultBy(t, proj, "signal_hygiene", "dc_offset", "t1")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("dc_offset status = %s, want not_evaluable (missing key)", row.Status)
	}
	if row.Code != "assert_signal_dc_offset_not_evaluable" {
		t.Fatalf("code = %s, want assert_signal_dc_offset_not_evaluable", row.Code)
	}
}

func TestBuildAssertionDCOffsetFromEvidenceFallback(t *testing.T) {
	proj := Build(Input{
		ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks": []any{map[string]any{
				"track_id": "t1", "track_name": "Evidence", "clip_count": 1,
				"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
				"acoustic":     map[string]any{"status": "ready"},
			}},
		},
		AcousticEvidenceByTrack: map[string]map[string]any{
			"t1": {"dc_offset": 0.02},
		},
	})
	row := assertionResultBy(t, proj, "signal_hygiene", "dc_offset", "t1")
	if row.Status != AssertionStatusFail {
		t.Fatalf("dc_offset status = %s, want fail via evidence fallback: %#v", row.Status, row)
	}
}

func TestBuildAssertionPluginHygieneStates(t *testing.T) {
	basePackage := map[string]any{"track_count": 0, "tracks": []any{}}

	// Missing block entirely: not_evaluable.
	proj := Build(Input{ProjectPackage: basePackage})
	row := assertionResultBy(t, proj, "plugin_hygiene", "startup_cleanup", "")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("plugin_hygiene without block status = %s, want not_evaluable", row.Status)
	}
	if row.Code != "assert_plugin_hygiene_not_evaluable" {
		t.Fatalf("code = %s, want assert_plugin_hygiene_not_evaluable", row.Code)
	}

	// Cleanup ran, removed nothing: pass with explicit zero.
	proj = Build(Input{
		ProjectPackage:    basePackage,
		PluginListHygiene: map[string]any{"cleanup_ran": true, "removed_total": 0, "types_removed": 0, "blacklist_removed": 0},
	})
	row = assertionResultBy(t, proj, "plugin_hygiene", "startup_cleanup", "")
	if row.Status != AssertionStatusPass {
		t.Fatalf("zero-removal run status = %s, want pass (explicit zero)", row.Status)
	}
	if row.Value == nil || *row.Value != 0 {
		t.Fatalf("zero-removal value = %v, want 0", row.Value)
	}

	// Cleanup ran and removed stale entries: fail (warn) with the count.
	proj = Build(Input{
		ProjectPackage:    basePackage,
		PluginListHygiene: map[string]any{"cleanup_ran": true, "removed_total": 3, "types_removed": 2, "blacklist_removed": 1},
	})
	row = assertionResultBy(t, proj, "plugin_hygiene", "startup_cleanup", "")
	if row.Status != AssertionStatusFail {
		t.Fatalf("stale-removal run status = %s, want fail", row.Status)
	}
	if row.Code != "assert_plugin_list_stale_removed" {
		t.Fatalf("code = %s, want assert_plugin_list_stale_removed", row.Code)
	}
	if row.Value == nil || *row.Value != 3 {
		t.Fatalf("value = %v, want removed_total 3", row.Value)
	}

	// Block present but cleanup never ran (older/odd kernel): not_evaluable.
	proj = Build(Input{
		ProjectPackage:    basePackage,
		PluginListHygiene: map[string]any{"cleanup_ran": false, "removed_total": 0},
	})
	row = assertionResultBy(t, proj, "plugin_hygiene", "startup_cleanup", "")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("never-ran status = %s, want not_evaluable", row.Status)
	}

	// Block present but uninterpretable keys: not_evaluable, never fabricated.
	proj = Build(Input{
		ProjectPackage:    basePackage,
		PluginListHygiene: map[string]any{"cleanup_ran": true},
	})
	row = assertionResultBy(t, proj, "plugin_hygiene", "startup_cleanup", "")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("uninterpretable block status = %s, want not_evaluable", row.Status)
	}
}

func TestBuildAssertionHygieneRowsReconcileLoaded(t *testing.T) {
	// The new checks must be in the fail-closed entity registry so persisted
	// rows survive ReconcileLoadedAssertions.
	rows := []AssertionResult{
		{Asserter: "signal_hygiene", Check: "dc_offset", Status: AssertionStatusFail},
		{Asserter: "plugin_hygiene", Check: "startup_cleanup", Status: AssertionStatusPass},
		{Asserter: "signal_hygiene", Check: "invented_check", Status: AssertionStatusPass},
	}
	kept, limitations := ReconcileLoadedAssertions(rows)
	if len(kept) != 2 {
		t.Fatalf("kept = %d rows, want 2 (new checks registered): %#v", len(kept), kept)
	}
	if len(limitations) == 0 {
		t.Fatalf("invented check must produce a limitation")
	}
}
