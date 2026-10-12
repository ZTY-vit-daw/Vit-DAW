package harness

// automation_curve_test.go — M3-KERNEL-1 agent 面单测：
// ①摄入白名单（裁定 4，DAD 证据层扩型）——automation_curve_summary 事件
//   落盘 mixboard feature snapshot 行（plugin/param 身份），非白名单
//   feature_type 与缺身份事件不摄入（fail-closed）；
// ②Invoke pre-journal 越界 fail-closed——t<0、|c|>1、region 非法、
//   区域外点、缺字段/缺 points、clear_region 缺 region 整单拒绝，
//   合法载荷放行（v 的 valueRange 域在内核侧校验，不在本面）。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func automationIngestTestHarness(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	root := filepath.Join(tmp, "mixboard")
	previous, had := os.LookupEnv("VIT_MIXBOARD_ROOT")
	if err := os.Setenv("VIT_MIXBOARD_ROOT", root); err != nil {
		t.Fatal(err)
	}
	if had {
		t.Cleanup(func() { _ = os.Setenv("VIT_MIXBOARD_ROOT", previous) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("VIT_MIXBOARD_ROOT") })
	}
	return filepath.Join(tmp, "mixboard_feature_snapshot.json")
}

func readAutomationSnapshotSection(t *testing.T, path string) (map[string]any, []any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("feature snapshot file missing: %v", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("feature snapshot not valid JSON: %v", err)
	}
	row, _ := snapshot["automation_curve_summary"].(map[string]any)
	rows, _ := snapshot["automation_curve_summaries"].([]any)
	return row, rows
}

func TestIngestAutomationCurveSummaryPersistsRow(t *testing.T) {
	snapshotPath := automationIngestTestHarness(t)
	h := NewWithSender(nil, nil, nil)

	h.IngestKernelTelemetry(map[string]any{
		"command":     "audio_feature_data_ready",
		"feature_type": "automation_curve_summary",
		"plugin_id":   "plug-1",
		"param_id":    "gain",
		"track_id":    "track-1",
		"point_count": 3,
		"time_base":   "time",
	})

	row, rows := readAutomationSnapshotSection(t, snapshotPath)
	if len(row) == 0 {
		t.Fatal("automation_curve_summary section missing after ingest")
	}
	if row["plugin_id"] != "plug-1" || row["param_id"] != "gain" {
		t.Fatalf("row identity = %v/%v", row["plugin_id"], row["param_id"])
	}
	if row["feature_type"] != "automation_curve_summary" {
		t.Fatalf("row feature_type = %v", row["feature_type"])
	}
	if row["materialized_by"] != "kernel_prepared_automation_telemetry" {
		t.Fatalf("row materialized_by = %v", row["materialized_by"])
	}
	if len(rows) != 1 {
		t.Fatalf("automation_curve_summaries rows = %d, want 1", len(rows))
	}

	// 同 plugin/param 的第二条摘要应合并为同一身份行，不无限增长。
	h.IngestKernelTelemetry(map[string]any{
		"command":     "audio_feature_data_ready",
		"feature_type": "automation_curve_summary",
		"plugin_id":   "plug-1",
		"param_id":    "gain",
		"track_id":    "track-1",
		"point_count": 2,
	})
	_, rows = readAutomationSnapshotSection(t, snapshotPath)
	if len(rows) != 1 {
		t.Fatalf("same-identity rows = %d, want merged 1", len(rows))
	}

	// 不同 param 是不同身份行。
	h.IngestKernelTelemetry(map[string]any{
		"command":     "audio_feature_data_ready",
		"feature_type": "automation_curve_summary",
		"plugin_id":   "plug-1",
		"param_id":    "pan",
		"track_id":    "track-1",
		"point_count": 1,
	})
	_, rows = readAutomationSnapshotSection(t, snapshotPath)
	if len(rows) != 2 {
		t.Fatalf("rows after second param = %d, want 2", len(rows))
	}
}

func TestIngestAutomationCurveWhitelistFailClosed(t *testing.T) {
	snapshotPath := automationIngestTestHarness(t)
	h := NewWithSender(nil, nil, nil)

	// 非白名单 feature_type：不摄入。
	h.IngestKernelTelemetry(map[string]any{
		"command":     "audio_feature_data_ready",
		"feature_type": "automation_curve_raw",
		"plugin_id":   "plug-1",
		"param_id":    "gain",
	})
	if _, err := os.Stat(snapshotPath); !os.IsNotExist(err) {
		t.Fatal("non-whitelisted feature_type must not create a snapshot file")
	}

	// 缺 param_id 身份：不摄入。
	h.IngestKernelTelemetry(map[string]any{
		"command":     "audio_feature_data_ready",
		"feature_type": "automation_curve_summary",
		"plugin_id":   "plug-1",
	})
	if _, err := os.Stat(snapshotPath); !os.IsNotExist(err) {
		t.Fatal("identity-less automation event must not create a snapshot file")
	}

	// isAutomationCurveSummaryFeature 白名单本体。
	for featureType, want := range map[string]bool{
		"automation_curve_summary":  true,
		"AUTOMATION_CURVE_SUMMARY":  true,
		" automation_curve_summary ": true,
		"automation_curve_raw":      false,
		"band_energy_summary":       false,
		"":                          false,
	} {
		if got := isAutomationCurveSummaryFeature(featureType); got != want {
			t.Fatalf("isAutomationCurveSummaryFeature(%q) = %v, want %v", featureType, got, want)
		}
	}
}

func TestValidateAutomationSetPointsFailClosed(t *testing.T) {
	h := NewWithSender(nil, nil, nil)

	valid := map[string]any{
		"plugin_id": "plug-1",
		"param_id":  "gain",
		"points": []any{
			map[string]any{"t": 0.0, "v": -6.0, "c": 0.0},
			map[string]any{"t": 2.5, "v": -3.0, "c": 0.4},
			map[string]any{"t": 5.0, "v": -12.0},
		},
	}
	if err := h.validateAutomationWrite("automation.set_points", cloneAnyMap(valid)); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	withRegion := cloneAnyMap(valid)
	withRegion["region"] = map[string]any{"start": 0.0, "end": 6.0}
	if err := h.validateAutomationWrite("automation.set_points", withRegion); err != nil {
		t.Fatalf("valid region payload rejected: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(cmd map[string]any)
		wantErr string
	}{
		{"t below zero", func(cmd map[string]any) {
			cmd["points"] = []any{map[string]any{"t": -0.1, "v": -6.0}}
		}, "t < 0"},
		{"curve c above one", func(cmd map[string]any) {
			cmd["points"] = []any{map[string]any{"t": 1.0, "v": -6.0, "c": 1.5}}
		}, "outside [-1, 1]"},
		{"curve c below minus one", func(cmd map[string]any) {
			cmd["points"] = []any{map[string]any{"t": 1.0, "v": -6.0, "c": -1.5}}
		}, "outside [-1, 1]"},
		{"non numeric t", func(cmd map[string]any) {
			cmd["points"] = []any{map[string]any{"t": "soon", "v": -6.0}}
		}, "requires numeric t and v"},
		{"missing v", func(cmd map[string]any) {
			cmd["points"] = []any{map[string]any{"t": 1.0}}
		}, "requires numeric t and v"},
		{"point not an object", func(cmd map[string]any) {
			cmd["points"] = []any{"point"}
		}, "must be an object"},
		{"empty points", func(cmd map[string]any) {
			cmd["points"] = []any{}
		}, "non-empty points array"},
		{"missing points", func(cmd map[string]any) {
			delete(cmd, "points")
		}, "non-empty points array"},
		{"region end before start", func(cmd map[string]any) {
			cmd["region"] = map[string]any{"start": 5.0, "end": 1.0}
		}, "end must be greater than region start"},
		{"region start negative", func(cmd map[string]any) {
			cmd["region"] = map[string]any{"start": -1.0, "end": 5.0}
		}, "start must be >= 0"},
		{"region missing end", func(cmd map[string]any) {
			cmd["region"] = map[string]any{"start": 1.0}
		}, "numeric start and end"},
		{"point outside region", func(cmd map[string]any) {
			cmd["region"] = map[string]any{"start": 0.0, "end": 3.0}
		}, "outside the declared replacement region"},
		{"missing plugin_id", func(cmd map[string]any) {
			delete(cmd, "plugin_id")
		}, "plugin_id and param_id"},
		{"missing param_id", func(cmd map[string]any) {
			delete(cmd, "param_id")
		}, "plugin_id and param_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cloneAnyMap(valid)
			tc.mutate(cmd)
			err := h.validateAutomationWrite("automation.set_points", cmd)
			if err == nil {
				t.Fatal("out-of-domain payload must be rejected")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}

	// 别名形态归一后同规则。
	if err := h.validateAutomationWrite("automation_set_points", cloneAnyMap(valid)); err != nil {
		t.Fatalf("alias form of valid payload rejected: %v", err)
	}
	invalidAlias := cloneAnyMap(valid)
	invalidAlias["points"] = []any{map[string]any{"t": -1.0, "v": 0.0}}
	if err := h.validateAutomationWrite("automation_set_points", invalidAlias); err == nil {
		t.Fatal("alias form of out-of-domain payload must be rejected")
	}
}

func TestValidateAutomationClearRegionFailClosed(t *testing.T) {
	h := NewWithSender(nil, nil, nil)

	valid := map[string]any{
		"plugin_id": "plug-1",
		"param_id":  "gain",
		"region":    map[string]any{"start": 1.0, "end": 5.0},
	}
	if err := h.validateAutomationWrite("automation.clear_region", cloneAnyMap(valid)); err != nil {
		t.Fatalf("valid clear_region rejected: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(cmd map[string]any)
		wantErr string
	}{
		{"missing region", func(cmd map[string]any) {
			delete(cmd, "region")
		}, "requires a region"},
		{"region not an object", func(cmd map[string]any) {
			cmd["region"] = "0..5"
		}, "must be an object"},
		{"region end equals start", func(cmd map[string]any) {
			cmd["region"] = map[string]any{"start": 5.0, "end": 5.0}
		}, "end must be greater than region start"},
		{"region start negative", func(cmd map[string]any) {
			cmd["region"] = map[string]any{"start": -0.5, "end": 5.0}
		}, "start must be >= 0"},
		{"missing plugin_id", func(cmd map[string]any) {
			delete(cmd, "plugin_id")
		}, "plugin_id and param_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cloneAnyMap(valid)
			tc.mutate(cmd)
			err := h.validateAutomationWrite("automation.clear_region", cmd)
			if err == nil {
				t.Fatal("out-of-domain clear_region must be rejected")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}

	if name := automationWriteCommandName("automation.get_curve"); name != "" {
		t.Fatal("read command must not be treated as a write command")
	}
}
