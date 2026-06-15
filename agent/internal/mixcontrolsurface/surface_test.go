package mixcontrolsurface

import "testing"

func TestBuildInfersVocalStabilityRoles(t *testing.T) {
	surface := Build(Request{
		MixSessionID: "mix_1",
		Mode:         "auto_mix",
		Goal:         "make the vocal more stable and more forward",
		Target:       Target{Kind: "track", ID: "track_1", Label: "Vocal"},
	})

	roles := rows(surface["required_roles"])
	if len(roles) < 2 {
		t.Fatalf("roles = %#v", roles)
	}
	seen := map[string]bool{}
	for _, role := range roles {
		seen[text(role, "type")] = true
	}
	if !seen["eq"] || !seen["dynamics"] {
		t.Fatalf("expected eq and dynamics roles, got %#v", roles)
	}
}

func TestBuildMarksReadyProfileButNeedsLoad(t *testing.T) {
	profile := tdrNovaProfile()
	surface := Build(Request{
		MixSessionID: "mix_1",
		Mode:         "auto_mix",
		Goal:         "make vocal stable and forward",
		Target:       Target{Kind: "track", ID: "track_1", Label: "Vocal"},
		ProjectProfiles: []map[string]any{
			profile,
		},
	})

	if got := text(surface, "readiness"); got != ReadinessNeedsConfirmation {
		t.Fatalf("readiness = %q surface=%#v", got, surface)
	}
	if got := text(surface, "profile_status"); got != ProfileReady {
		t.Fatalf("profile_status = %q surface=%#v", got, surface)
	}
	if got := text(surface, "instance_status"); got != InstanceNeedsLoad {
		t.Fatalf("instance_status = %q surface=%#v", got, surface)
	}
	selected := rows(surface["selected_chain"])
	if len(selected) == 0 {
		t.Fatalf("selected chain missing: %#v", surface)
	}
	if controls := rows(surface["proposed_controls"]); len(controls) == 0 {
		t.Fatalf("expected proposed controls from profile: %#v", surface)
	}
}

func TestBuildBlocksMissingProfile(t *testing.T) {
	surface := Build(Request{
		MixSessionID: "mix_1",
		Mode:         "co_mix",
		Goal:         "make vocal stable and forward",
		Target:       Target{Kind: "track", ID: "track_1", Label: "Vocal"},
		PluginCandidates: []map[string]any{{
			"name":         "Plain Compressor",
			"primary_type": "dynamics",
			"plugin_path":  "C:/VST/Plain Compressor.vst3",
		}, {
			"name":         "Plain EQ",
			"primary_type": "eq",
			"plugin_path":  "C:/VST/Plain EQ.vst3",
		}},
	})

	if got := text(surface, "readiness"); got != ReadinessBlocked {
		t.Fatalf("readiness = %q surface=%#v", got, surface)
	}
	if got := text(surface, "next_required_action"); got != "learn_plugin_profile" {
		t.Fatalf("next action = %q surface=%#v", got, surface)
	}
	if blockers := rows(surface["blockers"]); len(blockers) != 0 {
		t.Fatalf("blockers should be []string, got rows %#v", blockers)
	}
}

func TestBuildPrefersReadyDynamicEQOverMissingCompressorProfile(t *testing.T) {
	surface := Build(Request{
		MixSessionID: "mix_1",
		Mode:         "auto_mix",
		Goal:         "make vocal stable and forward",
		Target:       Target{Kind: "track", ID: "track_1", Label: "Vocal"},
		PluginCandidates: []map[string]any{{
			"name":         "ZL Compressor",
			"manufacturer": "ZL",
			"primary_type": "dynamics",
			"plugin_path":  "C:/VST/ZL Compressor.vst3",
			"semantic_types": []map[string]any{{
				"type":  "dynamics",
				"score": 900,
			}},
			"search_score": 900,
		}, {
			"name":         "ZL Compressor",
			"manufacturer": "ZL",
			"primary_type": "dynamics",
			"identifier":   "duplicate-zl-compressor",
		}},
		ProjectProfiles: []map[string]any{
			tdrNovaProfile(),
		},
	})

	if got := text(surface, "profile_status"); got != ProfileReady {
		t.Fatalf("profile_status = %q surface=%#v", got, surface)
	}
	selected := rows(surface["selected_chain"])
	var dynamics map[string]any
	for _, row := range selected {
		if text(row, "type") == "dynamics" {
			dynamics = row
			break
		}
	}
	if len(dynamics) == 0 {
		t.Fatalf("dynamics selection missing: %#v", selected)
	}
	plugin := mapValue(dynamics["selected_plugin"])
	if got := text(plugin, "name"); got != "TDR Nova" {
		t.Fatalf("dynamics should prefer ready TDR Nova over missing ZL Compressor, got %q row=%#v", got, dynamics)
	}
	candidates := rows(surface["plugin_candidates"])
	zlCount := 0
	for _, candidate := range candidates {
		if text(candidate, "name") == "ZL Compressor" {
			zlCount++
		}
	}
	if zlCount != 1 {
		t.Fatalf("ZL Compressor should be deduped, count=%d candidates=%#v", zlCount, candidates)
	}
}

func TestBuildBlocksWhenOnlyMissingCompressorProfileExists(t *testing.T) {
	surface := Build(Request{
		MixSessionID: "mix_1",
		Mode:         "auto_mix",
		Goal:         "make vocal stable and forward",
		Target:       Target{Kind: "track", ID: "track_1", Label: "Vocal"},
		PluginCandidates: []map[string]any{{
			"name":         "Plain EQ",
			"manufacturer": "Plain",
			"primary_type": "eq",
			"plugin_path":  "C:/VST/Plain EQ.vst3",
		}, {
			"name":         "ZL Compressor",
			"manufacturer": "ZL",
			"primary_type": "dynamics",
			"plugin_path":  "C:/VST/ZL Compressor.vst3",
		}},
		ProjectProfiles: []map[string]any{
			eqOnlyProfile(),
		},
	})

	if got := text(surface, "readiness"); got != ReadinessBlocked {
		t.Fatalf("readiness = %q surface=%#v", got, surface)
	}
	if got := text(surface, "next_required_action"); got != "learn_plugin_profile" {
		t.Fatalf("next action = %q surface=%#v", got, surface)
	}
}

func TestBuildMarksExistingInstance(t *testing.T) {
	surface := Build(Request{
		MixSessionID: "mix_1",
		Mode:         "auto_mix",
		Goal:         "make vocal stable and forward",
		Target:       Target{Kind: "track", ID: "track_1", Label: "Vocal"},
		ProjectProfiles: []map[string]any{
			tdrNovaProfile(),
		},
		RackPlugins: []map[string]any{{
			"track_id":    "track_1",
			"plugin_id":   "plugin_1",
			"plugin_name": "TDR Nova",
			"plugin_path": "C:/Program Files/Common Files/VST3/TDR Nova.vst3",
		}},
	})

	if got := text(surface, "instance_status"); got != InstanceExisting {
		t.Fatalf("instance_status = %q surface=%#v", got, surface)
	}
	if got := text(surface, "readiness"); got != ReadinessPlanReady {
		t.Fatalf("readiness = %q surface=%#v", got, surface)
	}
}

func tdrNovaProfile() map[string]any {
	return map[string]any{
		"profile_id": "plugin_tdr_nova",
		"class":      "eq",
		"plugin_identity": map[string]any{
			"plugin_name":   "TDR Nova",
			"plugin_path":   "C:/Program Files/Common Files/VST3/TDR Nova.vst3",
			"manufacturer":  "Tokyo Dawn Labs",
			"plugin_format": "VST3",
		},
		"plugin_skill": map[string]any{
			"operations": []map[string]any{{
				"name":         "add presence with band 3",
				"component_id": "band3",
				"params": map[string]any{
					"frequency": map[string]any{"param_id": "28"},
					"gain":      map[string]any{"param_id": "26"},
				},
			}, {
				"name":         "wideband compression",
				"component_id": "wide",
				"params": map[string]any{
					"threshold": map[string]any{"param_id": "58"},
					"ratio":     map[string]any{"param_id": "59"},
				},
			}},
		},
	}
}

func eqOnlyProfile() map[string]any {
	return map[string]any{
		"profile_id": "plugin_plain_eq",
		"class":      "eq",
		"plugin_identity": map[string]any{
			"plugin_name":   "Plain EQ",
			"plugin_path":   "C:/VST/Plain EQ.vst3",
			"manufacturer":  "Plain",
			"plugin_format": "VST3",
		},
		"plugin_skill": map[string]any{
			"operations": []map[string]any{{
				"name":         "shape eq band",
				"component_id": "band1",
				"params": map[string]any{
					"frequency": map[string]any{"param_id": "freq"},
					"gain":      map[string]any{"param_id": "gain"},
				},
			}},
		},
	}
}
