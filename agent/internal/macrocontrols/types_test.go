package macrocontrols

import "testing"

func TestNormalizeControlSynthesizesTrackVolumeBinding(t *testing.T) {
	macro := NormalizeControl(map[string]any{
		"macro_id": "mix_macro_session_track_volume",
		"name":     "Track volume",
		"track_id": "1007",
		"control":  "track.volume",
		"value":    -6.0,
		"min":      -60.0,
		"max":      12.0,
		"unit":     "dB",
	})

	if macro["control"] != "track.volume" {
		t.Fatalf("control = %v", macro["control"])
	}
	if macro["value"] != -6.0 || macro["min"] != -60.0 || macro["max"] != 12.0 {
		t.Fatalf("range/value lost: %#v", macro)
	}
	bindings, ok := macro["bindings"].([]map[string]any)
	if !ok || len(bindings) != 1 {
		t.Fatalf("bindings = %#v", macro["bindings"])
	}
	if bindings[0]["param_id"] != "track.volume" || bindings[0]["control"] != "track.volume" {
		t.Fatalf("binding = %#v", bindings[0])
	}
}

func TestNormalizeControlPreservesBindingControl(t *testing.T) {
	macro := NormalizeControl(map[string]any{
		"macro_id": "macro_1",
		"track_id": "1007",
		"bindings": []map[string]any{{
			"control":  "track.volume",
			"track_id": "1007",
			"param_id": "track.volume",
		}},
	})
	bindings := macro["bindings"].([]map[string]any)
	if bindings[0]["control"] != "track.volume" {
		t.Fatalf("binding control lost: %#v", bindings[0])
	}
}
