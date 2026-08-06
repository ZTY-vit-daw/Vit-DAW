package chat

import (
	"testing"

	"vit-daw-agent/internal/harness"
)

func TestAttachCompressorControlRefsRoundTrip(t *testing.T) {
	summary := map[string]any{
		"control_topology": map[string]any{"generation": "c2t1_test"},
		"compressor_stage": map[string]any{
			"stage_key": "compressor",
			"control_paths": []map[string]any{{
				"path_key":        "main",
				"operating_point": []map[string]any{{"param_id": "12", "role": "threshold"}},
			}},
			"output": []map[string]any{{"param_id": "18", "role": "mix"}},
		},
	}
	if err := attachCompressorControlRefs(summary, "1007", "1013"); err != nil {
		t.Fatal(err)
	}
	path := mapRowsValue(mapValue(summary["compressor_stage"])["control_paths"])[0]
	row := mapRowsValue(path["operating_point"])[0]
	ref, err := decodeCompressorControlRef(firstNonEmptyText(row, "control_ref"))
	if err != nil {
		t.Fatal(err)
	}
	if ref.TrackID != "1007" || ref.PluginID != "1013" || ref.TopologyGeneration != "c2t1_test" ||
		ref.PathKey != "main" || ref.Section != "operating_point" || ref.Role != "threshold" || ref.ParamID != "12" {
		t.Fatalf("unexpected control ref: %+v", ref)
	}
	tampered := firstNonEmptyText(row, "control_ref")
	tampered = tampered[:len(tampered)-1] + map[bool]string{true: "0", false: "1"}[tampered[len(tampered)-1] != '0']
	if _, err := decodeCompressorControlRef(tampered); err == nil {
		t.Fatal("tampered compressor control_ref must be rejected")
	}
}

func TestPluginGrabberInspectCompressorInvokeCommand(t *testing.T) {
	cmd, ok := pluginGrabberInspectCompressorInvokeCommand(harnessInvokeRequestForCompressorTest())
	if !ok || firstNonEmptyText(cmd, "track_id") != "1007" || firstNonEmptyText(cmd, "plugin_id") != "1013" {
		t.Fatalf("command = %+v ok=%v", cmd, ok)
	}
}

func harnessInvokeRequestForCompressorTest() harness.InvokeRequest {
	return harness.InvokeRequest{Tool: pluginGrabberInspectCompressorTool, Args: map[string]any{"track_id": "1007", "plugin_id": "1013"}}
}
