package plugingrabber

import (
	"strings"
	"testing"
)

func TestAutoLearnTDRNovaBuildsEQBandsAndRuntimeOperation(t *testing.T) {
	digest := tdrNovaDigest()
	patch, summary, ok := BuildAutoLearnProfilePatch(digest)
	if !ok {
		t.Fatal("expected deterministic TDR Nova auto learn patch")
	}
	if patch.Class != "eq" {
		t.Fatalf("class = %q", patch.Class)
	}
	if len(patch.Groups) < 4 {
		t.Fatalf("groups = %+v", patch.Groups)
	}
	b1 := profileGroupByID(patch.Groups, "b1")
	if len(b1) == 0 {
		t.Fatalf("b1 group missing: %+v", patch.Groups)
	}
	params := mapValue(b1["params"])
	for _, slot := range []string{"frequency", "gain", "q", "enable", "dyn_enable", "type"} {
		if len(mapValue(params[slot])) == 0 {
			t.Fatalf("b1 missing %s mapping: %+v", slot, params)
		}
	}
	if !containsVirtualControlName(patch.VirtualControls, "eq.cut_region") {
		t.Fatalf("eq.cut_region missing: %+v", patch.VirtualControls)
	}
	if summary["strategy"] != autoLearnSourceEQBandPattern {
		t.Fatalf("strategy = %+v", summary["strategy"])
	}
	frequency := mapValue(params["frequency"])
	if confirmed, _ := frequency["confirmed"].(bool); confirmed {
		t.Fatalf("auto learn name/pattern match should not be confirmed: %+v", frequency)
	}
	if frequency["status"] != PluginSkillStatusActive {
		t.Fatalf("auto learn runtime mapping status = %+v", frequency["status"])
	}
	frequencyDomain := mapValue(frequency["display_domain"])
	if frequencyDomain["unit"] != "Hz" || frequencyDomain["status"] != displayDomainStatusInferred {
		t.Fatalf("frequency display domain = %+v", frequencyDomain)
	}
	dynEnable := mapValue(params["dyn_enable"])
	dynEnableDomain := mapValue(dynEnable["display_domain"])
	if dynEnable["param_id"] != "B1 Dyn" || dynEnableDomain["unit"] != "toggle" || dynEnableDomain["min"] != 0.0 || dynEnableDomain["max"] != 1.0 {
		t.Fatalf("dynamic enable mapping = %+v", dynEnable)
	}
	shape := mapValue(params["type"])
	if shape["param_id"] != "B1 Type" {
		t.Fatalf("band shape mapping = %+v", shape)
	}
	domainSummary, ok := summary["display_domain_summary"].(map[string]int)
	if !ok || domainSummary[displayDomainStatusInferred] == 0 {
		t.Fatalf("display domain summary = %+v", summary["display_domain_summary"])
	}
	cmd, validation, err := BuildPluginSkillUpsertCommand(LearningTarget{TrackID: "track_1", PluginID: "nova_1"}, patch, digest)
	if err != nil {
		t.Fatalf("BuildPluginSkillUpsertCommand: %v", err)
	}
	if cmd["schema_version"] != PluginSkillSchemaVersion {
		t.Fatalf("schema version = %+v", cmd["schema_version"])
	}
	if validation.Coverage["B1 Frequency"] != "used_in_component" {
		t.Fatalf("coverage = %+v", validation.Coverage)
	}
	if validation.CoverageSummary["used_in_component"] == 0 {
		t.Fatalf("coverage summary should retain mapped counts: %+v", validation.CoverageSummary)
	}
	for _, warning := range validation.Warnings {
		if strings.Contains(warning, "unknown_needs_review") {
			t.Fatalf("unknown coverage should not be exposed as a warning: %+v", validation.Warnings)
		}
	}
}

func TestPluginSkillValidationSummarizesUnknownCoverageWithoutWarnings(t *testing.T) {
	digest := ParameterDigest{
		PluginName:     "Simple EQ",
		ParameterCount: 3,
		Parameters: []ParameterInfo{
			{ID: "B1 Frequency", Name: "B1 Frequency", HostControllable: true},
			{ID: "B1 Gain", Name: "B1 Gain", HostControllable: true},
			{ID: "Dry Wet", Name: "Dry Wet", HostControllable: true},
		},
	}
	patch := ProfilePatch{
		Class: "eq",
		Groups: []map[string]any{{
			"id":    "b1",
			"role":  "eq_band",
			"label": "B1",
			"params": map[string]any{
				"frequency": map[string]any{"param_id": "B1 Frequency", "label": "B1 Frequency", "source": "test", "confidence": 0.95, "confirmed": true},
				"gain":      map[string]any{"param_id": "B1 Gain", "label": "B1 Gain", "source": "test", "confidence": 0.95, "confirmed": true},
			},
		}},
		VirtualControls: []map[string]any{{"name": "eq.set_region", "resolver": "choose_nearest_or_free_band"}},
	}
	_, validation, err := BuildPluginSkillUpsertCommand(LearningTarget{TrackID: "track_1", PluginID: "eq_1"}, patch, digest)
	if err != nil {
		t.Fatalf("BuildPluginSkillUpsertCommand: %v", err)
	}
	if validation.CoverageSummary["unknown_needs_review"] != 1 {
		t.Fatalf("coverage summary = %+v", validation.CoverageSummary)
	}
	for _, warning := range validation.Warnings {
		if strings.Contains(warning, "unknown_needs_review") {
			t.Fatalf("unknown coverage should not be exposed as a warning: %+v", validation.Warnings)
		}
	}
}

func TestPluginSkillSanitizesUnverifiedNumericSafetyNotes(t *testing.T) {
	digest := ParameterDigest{
		PluginName:     "Delay",
		ParameterCount: 1,
		Parameters: []ParameterInfo{
			{ID: "delay", Name: "Delay_Ms", HostControllable: true},
		},
	}
	patch := ProfilePatch{
		QuickControlIDs: []string{"delay"},
		Aliases:         map[string]string{"delay": "Delay"},
		Safety: map[string]any{
			"notes":              "Delay time up to 300 ms or synced.",
			"max_gain_change_db": 6,
		},
		VirtualControls: []map[string]any{{
			"name":         "Echo Length",
			"component_id": "delay",
			"params": map[string]any{
				"time": map[string]any{"param_id": "delay", "label": "Delay"},
			},
		}},
	}
	cmd, _, err := BuildPluginSkillUpsertCommand(LearningTarget{TrackID: "track_1", PluginID: "delay_1"}, patch, digest)
	if err != nil {
		t.Fatalf("BuildPluginSkillUpsertCommand: %v", err)
	}
	safety := mapValue(cmd["safety"])
	if strings.Contains(firstNonEmptyText(safety, "notes"), "300") {
		t.Fatalf("unsafe safety notes were not sanitized: %+v", safety)
	}
	if safety["max_gain_change_db"] == nil {
		t.Fatalf("structured safety field should remain: %+v", safety)
	}
	skill, ok := cmd["plugin_skill"].(PluginSkillDocument)
	if !ok {
		t.Fatalf("plugin_skill = %T", cmd["plugin_skill"])
	}
	if strings.Contains(firstNonEmptyText(skill.Safety, "notes"), "300") {
		t.Fatalf("plugin skill safety notes were not sanitized: %+v", skill.Safety)
	}
}

func TestPluginSkillValidationRejectsEQMissingFrequency(t *testing.T) {
	digest := ParameterDigest{
		PluginName:     "Broken EQ",
		ParameterCount: 2,
		Parameters: []ParameterInfo{
			{ID: "B1 Gain", Name: "B1 Gain", HostControllable: true},
			{ID: "B1 Q", Name: "B1 Q", HostControllable: true},
		},
	}
	patch := ProfilePatch{
		Class: "eq",
		Groups: []map[string]any{{
			"id":    "b1",
			"role":  "eq_band",
			"label": "B1",
			"params": map[string]any{
				"gain": map[string]any{"param_id": "B1 Gain", "label": "B1 Gain", "confidence": 0.95, "confirmed": true},
				"q":    map[string]any{"param_id": "B1 Q", "label": "B1 Q", "confidence": 0.95, "confirmed": true},
			},
		}},
		VirtualControls: []map[string]any{{"name": "eq.cut_region", "resolver": "choose_nearest_or_free_band"}},
	}
	_, _, err := BuildPluginSkillUpsertCommand(LearningTarget{TrackID: "track_1", PluginID: "eq_1"}, patch, digest)
	if err == nil || !strings.Contains(err.Error(), "missing frequency") {
		t.Fatalf("expected missing frequency error, got %v", err)
	}
}

func TestTeachModeDiffCapturesMultipleParameterChanges(t *testing.T) {
	before := PluginParameterSnapshot{Parameters: []PluginParameterSnapshotValue{
		{ParamID: "p1", Name: "Drive", NormalizedValue: 0.1},
		{ParamID: "p2", Name: "Tone", NormalizedValue: 0.2},
		{ParamID: "p3", Name: "Mix", NormalizedValue: 0.3},
	}}
	after := PluginParameterSnapshot{Parameters: []PluginParameterSnapshotValue{
		{ParamID: "p1", Name: "Drive", NormalizedValue: 0.4},
		{ParamID: "p2", Name: "Tone", NormalizedValue: 0.8},
		{ParamID: "p3", Name: "Mix", NormalizedValue: 0.3},
	}}
	changes := DiffParameterSnapshots(before, after)
	if len(changes) != 2 {
		t.Fatalf("changes = %+v", changes)
	}
	if changes[0].ParamID != "p1" || changes[1].ParamID != "p2" {
		t.Fatalf("unexpected changes: %+v", changes)
	}
}

func TestTeachModeProfilePatchCreatesUserDemonstratedMappings(t *testing.T) {
	digest := ParameterDigest{
		PluginName:     "Demo Plugin",
		ParameterCount: 2,
		Parameters: []ParameterInfo{
			{ID: "drive", Name: "Drive", HostControllable: true},
			{ID: "tone", Name: "Tone", HostControllable: true},
		},
	}
	row := TeachModeRowRecord{
		RowID:             "row_1",
		Description:       "make it brighter",
		DisplayDomainText: "-20~20 dB",
		State:             TeachModeRowCaptured,
		Changes: []PluginParameterChange{
			{ParamID: "drive", Name: "Drive", BeforeNormalized: 0.2, AfterNormalized: 0.4, Delta: 0.2},
			{ParamID: "tone", Name: "Tone", BeforeNormalized: 0.3, AfterNormalized: 0.7, Delta: 0.4},
		},
	}
	patch, summary, err := BuildTeachModeProfilePatch(digest, []TeachModeRowRecord{row})
	if err != nil {
		t.Fatalf("BuildTeachModeProfilePatch: %v", err)
	}
	if summary["row_count"] != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	skill := BuildPluginSkillDocument(digest, patch)
	if len(skill.Components) != 1 || len(skill.Components[0].Params) != 2 {
		t.Fatalf("skill components = %+v", skill.Components)
	}
	for _, mapping := range skill.Components[0].Params {
		if mapping.Source != teachModeSourceUserDemonstrated || !mapping.Locked || !mapping.Confirmed {
			t.Fatalf("mapping not demonstrated/locked: %+v", mapping)
		}
		if mapping.DisplayDomain == nil || mapping.DisplayDomain.Unit != "dB" || mapping.DisplayDomain.Status != displayDomainStatusConfirmed {
			t.Fatalf("display domain missing from demonstrated mapping: %+v", mapping)
		}
	}
	if rows, ok := skill.Preferences.Explicit["teach_mode_rows"].([]TeachModeRowRecord); !ok || len(rows) != 1 {
		t.Fatalf("teach rows missing from preferences: %+v", skill.Preferences)
	}
}

func TestTeachModeDisplayDomainParsesInlineUnitRange(t *testing.T) {
	domain := parseDisplayDomainText("-20db——+20db", "teach_mode_user_input", true)
	if domain == nil {
		t.Fatal("domain is nil")
	}
	if domain.Unit != "dB" || domain.Status != displayDomainStatusConfirmed {
		t.Fatalf("domain unit/status = %+v", domain)
	}
	if domain.Min == nil || domain.Max == nil || *domain.Min != -20 || *domain.Max != 20 {
		t.Fatalf("domain range = %+v", domain)
	}
}

func TestDisplayDomainParsesConfirmedEQQRange(t *testing.T) {
	domain := parseDisplayDomainText("0.1~10 Q", "auto_learn_user_review", true)
	if domain == nil {
		t.Fatal("domain is nil")
	}
	if domain.Unit != "Q" || domain.Status != displayDomainStatusConfirmed {
		t.Fatalf("domain unit/status = %+v", domain)
	}
	if domain.Min == nil || domain.Max == nil || *domain.Min != 0.1 || *domain.Max != 10 {
		t.Fatalf("domain range = %+v", domain)
	}
}

func TestDisplayDomainParsesConfirmedEnumFilterShape(t *testing.T) {
	text := "enum: Bell / Low Shelf / Low Cut / High Shelf / High Cut / Notch / Band Pass / Tilt Shelf"
	domain := parseDisplayDomainText(text, "auto_learn_user_review", true)
	if domain == nil {
		t.Fatal("domain is nil")
	}
	if domain.Unit != "enum" || domain.Status != displayDomainStatusConfirmed {
		t.Fatalf("domain unit/status = %+v, want unit=enum status=confirmed (discrete_labels already resolve every step)", domain)
	}
}

func TestAutoLearnPrefersObservedQCurveOverGenericFallback(t *testing.T) {
	minValue, maxValue := 0.1, 6.0
	domain := inferredDisplayDomainForSlot("q", ParameterInfo{
		ID: "B1 Q", Name: "B1 Q",
		DisplayDomainCandidate: &PluginDisplayDomain{
			Text: "0.1~6", Min: &minValue, Max: &maxValue, Scale: "log",
			Status: displayDomainStatusInferred, Source: "display_probe_inferred", Confidence: 0.76,
		},
	})
	if domain == nil || domain.Unit != "Q" || domain.Min == nil || domain.Max == nil || *domain.Min != minValue || *domain.Max != maxValue || domain.Scale != "log" {
		t.Fatalf("observed Q curve was not retained: %+v", domain)
	}
}

func TestAutoLearnPreservesUserDemonstratedMapping(t *testing.T) {
	digest := tdrNovaDigest()
	digest.Parameters = append(digest.Parameters, ParameterInfo{ID: "User B2 Gain", Name: "User B2 Gain", HostControllable: true})
	digest.PluginSkill = map[string]any{
		"components": []any{map[string]any{
			"id":    "b2",
			"role":  "eq_band",
			"label": "B2",
			"params": map[string]any{
				"gain": map[string]any{
					"param_id":   "User B2 Gain",
					"label":      "Demonstrated B2 Gain",
					"source":     teachModeSourceUserDemonstrated,
					"confidence": 1.0,
					"confirmed":  true,
					"locked":     true,
				},
			},
		}},
	}
	patch, _, ok := BuildAutoLearnProfilePatch(digest)
	if !ok {
		t.Fatal("expected auto learn patch")
	}
	b2 := profileGroupByID(patch.Groups, "b2")
	params := mapValue(b2["params"])
	gain := mapValue(params["gain"])
	if gain["param_id"] != "User B2 Gain" || gain["source"] != teachModeSourceUserDemonstrated {
		t.Fatalf("user demonstrated gain was not preserved: %+v", gain)
	}
}

func tdrNovaDigest() ParameterDigest {
	params := []ParameterInfo{}
	for band := 1; band <= 4; band++ {
		prefix := "B" + string(rune('0'+band))
		params = append(params,
			ParameterInfo{ID: prefix + " Frequency", Name: prefix + " Frequency", RawName: prefix + " Freq", HostControllable: true},
			ParameterInfo{ID: prefix + " Gain", Name: prefix + " Gain", HostControllable: true},
			ParameterInfo{ID: prefix + " Q", Name: prefix + " Q", HostControllable: true},
			ParameterInfo{ID: prefix + " Enable", Name: prefix + " Enable", HostControllable: true, IsBoolean: true},
			ParameterInfo{ID: prefix + " Dyn", Name: prefix + " Dyn", HostControllable: true, IsBoolean: true},
			ParameterInfo{ID: prefix + " Type", Name: prefix + " Type", HostControllable: true, ValueText: "Bell"},
		)
	}
	return ParameterDigest{
		TrackID:        "track_1",
		PluginID:       "nova_1",
		PluginName:     "TDR Nova",
		TemplateRole:   "eq",
		ParameterCount: len(params),
		PluginIdentity: map[string]any{
			"plugin_name":  "TDR Nova",
			"manufacturer": "Tokyo Dawn Labs",
			"profile_key":  "plugin_tdr_nova",
		},
		Parameters: params,
	}
}

func profileGroupByID(groups []map[string]any, id string) map[string]any {
	for _, group := range groups {
		if firstNonEmptyText(group, "id") == id {
			return group
		}
	}
	return nil
}
