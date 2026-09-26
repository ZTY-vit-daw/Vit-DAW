package rlm

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// realOldC1ProjectObserveSHA256 pins the extracted content so the fixture
// cannot silently drift. Provenance chain (recorded in the L2-1-RLM-1 receipt):
// source artifact VitApp/Workspace/Artifacts/c1_saved_project_state_diag.json
// (2026-07-31, UTF-8 with BOM, sha256
// 3358de4b7e335aae3b9b131276299be7f058ffc102faae203cd37065739861de);
// extracted result.project_observe (71 real tracks), minified UTF-8, then
// gzipped (level 9) into testdata/real_old_c1_project_observe.json.gz.
const realOldC1ProjectObserveSHA256 = "fbe495a75fa0e4f429206c1ff957b338cd7e93ed777831784886d064b18e0bc3"

func loadRealOldC1ProjectObserve(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "real_old_c1_project_observe.json.gz"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip fixture: %v", err)
	}
	minified, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("gunzip fixture: %v", err)
	}
	if got := sha256Hex(minified); got != realOldC1ProjectObserveSHA256 {
		t.Fatalf("fixture drifted: sha256 = %s, want %s", got, realOldC1ProjectObserveSHA256)
	}
	out := map[string]any{}
	if err := json.Unmarshal(minified, &out); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return out
}

func TestBuiltinDeliveryProfilesAreCompleteAndValid(t *testing.T) {
	profiles := BuiltinDeliveryProfiles()
	if len(profiles) != 5 {
		t.Fatalf("builtin profile count = %d, want 5 (Apple Music/Spotify/AES streaming/EBU R128/GY/T 282-2014)", len(profiles))
	}
	seen := map[string]bool{}
	for _, profile := range profiles {
		if strings.TrimSpace(profile.ProfileID) == "" {
			t.Fatalf("builtin profile with empty id: %+v", profile)
		}
		if seen[profile.ProfileID] {
			t.Fatalf("duplicate builtin profile id %q", profile.ProfileID)
		}
		seen[profile.ProfileID] = true
		if strings.TrimSpace(profile.Source) == "" {
			t.Fatalf("builtin profile %q missing source note", profile.ProfileID)
		}
		if err := ValidateDeliveryProfile(profile); err != nil {
			t.Fatalf("builtin profile %q invalid: %v", profile.ProfileID, err)
		}
		// Standards without a programme-level short-term/momentary ceiling leave
		// those unconstrained: the fields exist, invented values do not.
		if profile.ShortTermMax != nil || profile.MomentaryMax != nil {
			t.Fatalf("builtin profile %q invents a short-term/momentary ceiling not present in its cited standard", profile.ProfileID)
		}
		if lookedUp, ok := LookupDeliveryProfile(profile.ProfileID); !ok || lookedUp.ProfileID != profile.ProfileID {
			t.Fatalf("LookupDeliveryProfile(%q) miss", profile.ProfileID)
		}
	}
	if _, ok := LookupDeliveryProfile("builtin:does_not_exist"); ok {
		t.Fatalf("LookupDeliveryProfile must miss unknown ids")
	}

	// Value anchors straight from the cited public standards.
	anchors := map[string]struct {
		min    *float64
		max    float64
		peakDB float64
	}{
		"builtin:ebu_r128":    {min: bandPtr(-23.5), max: -22.5, peakDB: -1},
		"builtin:aes_td1004":  {min: bandPtr(-20), max: -16, peakDB: -1},
		"builtin:apple_music": {min: nil, max: -16, peakDB: -1},
		"builtin:spotify":     {min: nil, max: -14, peakDB: -1},
		"builtin:gy_282_2014": {min: bandPtr(-26), max: -22, peakDB: -2},
	}
	for id, want := range anchors {
		profile, ok := LookupDeliveryProfile(id)
		if !ok {
			t.Fatalf("missing builtin profile %q", id)
		}
		if profile.Integrated.Max != want.max {
			t.Fatalf("%s integrated band max = %v, want %v", id, profile.Integrated.Max, want.max)
		}
		if !ptrEqual(profile.Integrated.Min, want.min) {
			t.Fatalf("%s integrated band min = %v, want %v", id, profile.Integrated.Min, want.min)
		}
		if profile.TruePeakMaxDBTP != want.peakDB {
			t.Fatalf("%s true peak max = %v, want %v", id, profile.TruePeakMaxDBTP, want.peakDB)
		}
		if profile.ChannelConfiguration != ChannelStereo {
			t.Fatalf("%s channel configuration = %q, want %q", id, profile.ChannelConfiguration, ChannelStereo)
		}
	}
}

func TestValidateDeliveryProfileRejectsIllegalVectors(t *testing.T) {
	valid := func(mutate func(*DeliveryProfile)) DeliveryProfile {
		base, ok := LookupDeliveryProfile("builtin:ebu_r128")
		if !ok {
			t.Fatalf("builtin:ebu_r128 missing")
		}
		next := base
		mutate(&next)
		return next
	}
	cases := []struct {
		name    string
		profile DeliveryProfile
	}{
		{"band min above max", valid(func(p *DeliveryProfile) { p.Integrated.Min = bandPtr(-10) })},
		{"short-term max above momentary max", valid(func(p *DeliveryProfile) {
			short, moment := -10.0, -12.0
			p.ShortTermMax, p.MomentaryMax = &short, &moment
		})},
		{"positive true peak", valid(func(p *DeliveryProfile) { p.TruePeakMaxDBTP = 0.5 })},
		{"unknown channel configuration", valid(func(p *DeliveryProfile) { p.ChannelConfiguration = "quadraphonic" })},
		{"unknown headroom strategy", valid(func(p *DeliveryProfile) { p.HeadroomStrategy = "hard_limited" })},
		{"empty profile id", valid(func(p *DeliveryProfile) { p.ProfileID = " " })},
		{"wrong schema version", valid(func(p *DeliveryProfile) { p.SchemaVersion = "rlm.delivery_profile.v999" })},
		{"NaN integrated max", valid(func(p *DeliveryProfile) { p.Integrated.Max = math.NaN() })},
		{"NaN true peak", valid(func(p *DeliveryProfile) { p.TruePeakMaxDBTP = math.Inf(1) })},
		{"NaN short-term max", valid(func(p *DeliveryProfile) {
			short := math.NaN()
			p.ShortTermMax = &short
		})},
		{"NaN band min", valid(func(p *DeliveryProfile) { p.Integrated.Min = bandPtr(math.NaN()) })},
	}
	for _, tc := range cases {
		if err := ValidateDeliveryProfile(tc.profile); err == nil {
			t.Fatalf("%s: illegal profile accepted: %+v", tc.name, tc.profile)
		}
	}

	// Short-term max below momentary max is physically sound (a 3s window
	// average can never exceed the max 400ms window average) and must pass.
	short, moment := -20.0, -18.0
	monoOnlyShort := valid(func(p *DeliveryProfile) {
		p.ChannelConfiguration = ChannelMono
		p.ShortTermMax, p.MomentaryMax = &short, &moment
	})
	if err := ValidateDeliveryProfile(monoOnlyShort); err != nil {
		t.Fatalf("consistent vector rejected: %v", err)
	}
	// One-sided constraints (only short-term, no momentary) stay legal.
	onlyShort := valid(func(p *DeliveryProfile) { p.ShortTermMax = &short })
	if err := ValidateDeliveryProfile(onlyShort); err != nil {
		t.Fatalf("one-sided constraint rejected: %v", err)
	}
	// Unset headroom strategy means "not specified" and stays legal.
	noHeadroom := valid(func(p *DeliveryProfile) { p.HeadroomStrategy = "" })
	if err := ValidateDeliveryProfile(noHeadroom); err != nil {
		t.Fatalf("unset headroom strategy rejected: %v", err)
	}
}

func TestDeliveryProfileSerializationRoundTrip(t *testing.T) {
	for _, profile := range BuiltinDeliveryProfiles() {
		data, err := MarshalDeliveryProfile(profile)
		if err != nil {
			t.Fatalf("marshal %q: %v", profile.ProfileID, err)
		}
		parsed, err := ParseDeliveryProfile(data)
		if err != nil {
			t.Fatalf("parse %q: %v", profile.ProfileID, err)
		}
		if !reflect.DeepEqual(parsed, profile) {
			t.Fatalf("round trip changed profile %q:\n got %+v\nwant %+v", profile.ProfileID, parsed, profile)
		}
	}

	base, _ := LookupDeliveryProfile("builtin:ebu_r128")
	mutateJSON := func(edit func(doc map[string]any)) []byte {
		data, err := json.Marshal(base)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		doc := map[string]any{}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		edit(doc)
		out, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("marshal edited: %v", err)
		}
		return out
	}
	failClosed := []struct {
		name string
		data []byte
	}{
		{"unknown schema version", mutateJSON(func(doc map[string]any) { doc["schema_version"] = "rlm.delivery_profile.v999" })},
		{"missing schema version", mutateJSON(func(doc map[string]any) { delete(doc, "schema_version") })},
		{"unknown channel enum", mutateJSON(func(doc map[string]any) { doc["channel_configuration"] = "9.1.6" })},
		{"unknown headroom enum", mutateJSON(func(doc map[string]any) { doc["headroom_strategy"] = "limiter_ceiling" })},
		{"band min above max", mutateJSON(func(doc map[string]any) { doc["integrated_lufs"] = map[string]any{"min_lufs": -10, "max_lufs": -20} })},
	}
	for _, tc := range failClosed {
		if _, err := ParseDeliveryProfile(tc.data); err == nil {
			t.Fatalf("%s: parse must fail closed", tc.name)
		}
	}
}

func TestRenderProfileBindingEnforcesOneRenderOneProfile(t *testing.T) {
	binding, err := NewRenderProfileBinding("render_001", "builtin:ebu_r128", "2026-09-27T00:00:00Z")
	if err != nil {
		t.Fatalf("valid binding rejected: %v", err)
	}
	if binding.RenderID != "render_001" || binding.ProfileID != "builtin:ebu_r128" || binding.SchemaVersion != RenderBindingSchemaVersion {
		t.Fatalf("binding fields = %+v", binding)
	}
	if profile, err := binding.ResolveProfile(); err != nil || profile.ProfileID != "builtin:ebu_r128" {
		t.Fatalf("resolve profile = %+v, err = %v", profile, err)
	}
	if _, err := NewRenderProfileBinding("", "builtin:ebu_r128", "2026-09-27T00:00:00Z"); err == nil {
		t.Fatalf("empty render id must be rejected")
	}
	if _, err := NewRenderProfileBinding("render_001", "builtin:unknown_platform", "2026-09-27T00:00:00Z"); err == nil {
		t.Fatalf("unknown profile id must be rejected (fail closed)")
	}

	data, err := MarshalRenderProfileBinding(binding)
	if err != nil {
		t.Fatalf("marshal binding: %v", err)
	}
	parsed, err := ParseRenderProfileBinding(data)
	if err != nil {
		t.Fatalf("parse binding: %v", err)
	}
	if !reflect.DeepEqual(parsed, binding) {
		t.Fatalf("binding round trip changed:\n got %+v\nwant %+v", parsed, binding)
	}
	badDocs := []string{
		`{"schema_version":"rlm.render_profile_binding.v999","render_id":"r","profile_id":"builtin:spotify"}`,
		`{"render_id":"r","profile_id":"builtin:spotify"}`,
		`{"schema_version":"` + RenderBindingSchemaVersion + `","render_id":"","profile_id":"builtin:spotify"}`,
		`{"schema_version":"` + RenderBindingSchemaVersion + `","render_id":"r","profile_id":"builtin:nope"}`,
	}
	for _, doc := range badDocs {
		if _, err := ParseRenderProfileBinding([]byte(doc)); err == nil {
			t.Fatalf("binding parse must fail closed: %s", doc)
		}
	}

	index := NewRenderBindingIndex()
	if err := index.Bind(binding); err != nil {
		t.Fatalf("first bind rejected: %v", err)
	}
	if err := index.Bind(binding); err != nil {
		t.Fatalf("idempotent rebind of the same profile rejected: %v", err)
	}
	other, err := NewRenderProfileBinding("render_001", "builtin:spotify", "2026-09-27T01:00:00Z")
	if err != nil {
		t.Fatalf("valid second binding rejected: %v", err)
	}
	if err := index.Bind(other); err == nil {
		t.Fatalf("one render one profile: rebinding render_001 to a different profile must be rejected")
	}
	if profileID, ok := index.ProfileForRender("render_001"); !ok || profileID != "builtin:ebu_r128" {
		t.Fatalf("ProfileForRender = %q, %v", profileID, ok)
	}
	if _, ok := index.ProfileForRender("render_missing"); ok {
		t.Fatalf("ProfileForRender must miss unbound renders")
	}
}

// Old records (pre profile field) must keep loading with zero behavior change:
// no panic, no profile, projection id and serialized shape unchanged. The
// input is a real artifact from the live workspace, not a synthetic sample.
func TestProjectionFromRealOldRecordKeepsCurrentBehaviorWithoutProfile(t *testing.T) {
	projectObserve := loadRealOldC1ProjectObserve(t)
	proj := Build(Input{
		GeneratedAt:  "2026-09-27T00:00:00Z",
		ProjectState: projectObserve,
	})
	if proj.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %q", proj.SchemaVersion)
	}
	if len(proj.Rows) == 0 {
		t.Fatalf("real 71-track project state produced zero rows")
	}
	switch proj.Status {
	case StatusReady, StatusPartial, StatusMissing:
	default:
		t.Fatalf("unexpected status %q", proj.Status)
	}

	data, err := json.Marshal(proj)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	if bytes.Contains(data, []byte("delivery_profile")) {
		t.Fatalf("projection built without a profile must not serialize a delivery_profile field")
	}
	reloaded := Projection{}
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("old-shape record without profile field must load: %v", err)
	}
	if reloaded.DeliveryProfile != nil {
		t.Fatalf("record without profile field must deserialize to no profile, got %+v", reloaded.DeliveryProfile)
	}
	if reloaded.ProjectionID != proj.ProjectionID || reloaded.Status != proj.Status {
		t.Fatalf("round trip changed projection: %q/%q vs %q/%q", reloaded.ProjectionID, reloaded.Status, proj.ProjectionID, proj.Status)
	}
	if !reflect.DeepEqual(ContextProjectionMap(reloaded), ContextProjectionMap(proj)) {
		t.Fatalf("context projection map changed for old-shape record")
	}

	minimalOldRecord := []byte(`{"schema_version":"` + SchemaVersion + `","rlm_version":"` + Version + `","status":"missing","actionable":false,"summary":{"track_count":0}}`)
	legacy := Projection{}
	if err := json.Unmarshal(minimalOldRecord, &legacy); err != nil {
		t.Fatalf("minimal old record must load: %v", err)
	}
	if legacy.DeliveryProfile != nil || legacy.Status != StatusMissing {
		t.Fatalf("minimal old record semantics changed: %+v", legacy)
	}
}

func ptrEqual(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
