package chat

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
)

func TestDeEsserInspectCommandRecognition(t *testing.T) {
	cmd, ok := pluginGrabberInspectDeEsserInvokeCommand(harness.InvokeRequest{
		Tool: pluginGrabberInspectDeEsserTool,
		Args: map[string]any{"track_id": "1007", "plugin_id": "1013"},
	})
	if !ok || firstNonEmptyText(cmd, "cmd") != pluginGrabberInspectDeEsserCommand ||
		firstNonEmptyText(cmd, "track_id") != "1007" || firstNonEmptyText(cmd, "plugin_id") != "1013" {
		t.Fatalf("cmd=%+v ok=%v", cmd, ok)
	}
}

func TestAttachDeEsserControlRefsBindsTopologyAndTarget(t *testing.T) {
	summary := map[string]any{
		"control_topology": map[string]any{"generation": "d1t1_test"},
		"de_esser_stages": []map[string]any{{
			"stage_key":             "de_esser_main",
			"operating_point":       []map[string]any{{"param_id": "threshold", "role": "threshold"}},
			"frequency_selectivity": []map[string]any{{"param_id": "frequency", "role": "focus_frequency"}},
			"timing":                []map[string]any{}, "mode": []map[string]any{}, "output": []map[string]any{},
		}},
	}
	if err := attachDeEsserControlRefs(summary, "1007", "1013"); err != nil {
		t.Fatal(err)
	}
	stage := mapRowsValue(summary["de_esser_stages"])[0]
	for _, section := range []string{"operating_point", "frequency_selectivity"} {
		for _, row := range mapRowsValue(stage[section]) {
			encoded := firstNonEmptyText(row, "control_ref")
			if !strings.HasPrefix(encoded, "d1cr1_") {
				t.Fatalf("control_ref=%q", encoded)
			}
			framing := strings.Split(strings.TrimPrefix(encoded, "d1cr1_"), ".")
			if len(framing) != 2 {
				t.Fatalf("invalid framing: %q", encoded)
			}
			payload, err := base64.RawURLEncoding.DecodeString(framing[0])
			if err != nil {
				t.Fatal(err)
			}
			var ref deEsserControlReference
			if err := json.Unmarshal(payload, &ref); err != nil {
				t.Fatal(err)
			}
			if ref.Kind != deEsserControlRefKind || ref.TrackID != "1007" || ref.PluginID != "1013" ||
				ref.TopologyGeneration != "d1t1_test" || ref.StageKey != "de_esser_main" ||
				ref.Section != section || ref.Role != firstNonEmptyText(row, "role") || ref.ParamID != firstNonEmptyText(row, "param_id") {
				t.Fatalf("ref=%+v row=%+v", ref, row)
			}
		}
	}
}
