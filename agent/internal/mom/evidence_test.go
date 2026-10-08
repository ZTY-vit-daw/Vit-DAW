package mom

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

var evidenceTestObservationID = "obs_20260921T120000_ab12cd34"

func evidenceTestInput() Input {
	return Input{ObservationID: evidenceTestObservationID, TargetRef: map[string]any{"id": "T1"}}
}

// REFSCHEMA-M2: the three C-class constructors emit vit://mom refs that parse
// to RefStateParsed with kind=mom, the legacy data key family in scope_kind,
// the data key remainder in scope_value, observation_id in the snapshot
// segment (identity family, M1 ruling), explicit t=all window and "-" hash.
func TestMomConstructorsEmitParsedMomRefs(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		scopeKind  string
		scopeValue string
	}{
		{"track_read_key", trackReadKey(evidenceTestInput(), "fast.levels"), "mix.read", "track.T1.fast.levels"},
		{"acoustic_feature_ref", acousticFeatureRef(evidenceTestInput(), "l3_deep", "band_energy_summary"), "acoustic_package_status", "l3_deep.band_energy_summary"},
		{"observation_ref", observationRef(evidenceTestInput()), "observation", evidenceTestObservationID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := agentprotocol.ParseRef(tc.raw)
			if err != nil {
				t.Fatalf("ParseRef(%q) error: %v", tc.raw, err)
			}
			if parsed.State != agentprotocol.RefStateParsed {
				t.Fatalf("ParseRef(%q) state = %v, want parsed", tc.raw, parsed.State)
			}
			ref := parsed.Ref
			if ref.Kind != "mom" {
				t.Fatalf("kind = %q, want mom", ref.Kind)
			}
			if ref.ScopeKind != tc.scopeKind || ref.ScopeValue != tc.scopeValue {
				t.Fatalf("scope = %q:%q, want %q:%q", ref.ScopeKind, ref.ScopeValue, tc.scopeKind, tc.scopeValue)
			}
			if ref.Snapshot != evidenceTestObservationID {
				t.Fatalf("snapshot = %q, want observation id %q", ref.Snapshot, evidenceTestObservationID)
			}
			if ref.Window == nil || !ref.Window.AllTime {
				t.Fatalf("window = %+v, want t=all", ref.Window)
			}
			if ref.Hash != "-" {
				t.Fatalf("hash = %q, want explicit un-CASed \"-\"", ref.Hash)
			}
		})
	}
}

// INV2 canonical closed loop: parse(format(r)) == r for every migrated
// constructor output.
func TestMomConstructorRefsCanonicalRoundTrip(t *testing.T) {
	refs := []string{
		trackReadKey(evidenceTestInput(), "slow.band_energy.summary"),
		acousticFeatureRef(evidenceTestInput(), "l1_static", "peak_rms_summary"),
		observationRef(evidenceTestInput()),
	}
	for _, raw := range refs {
		parsed, err := agentprotocol.ParseRef(raw)
		if err != nil || parsed.State != agentprotocol.RefStateParsed {
			t.Fatalf("ParseRef(%q) = %v, %v", raw, parsed.State, err)
		}
		if reformatted, err := agentprotocol.FormatRef(*parsed.Ref); err != nil || reformatted != raw {
			t.Fatalf("canonical round trip broke %q -> %q (err %v)", raw, reformatted, err)
		}
	}
}

// Golden literals: the exact wire form is part of the M2 contract.
func TestMomConstructorGoldenForms(t *testing.T) {
	wantTrack := "vit://mom/mix.read:track.T1.fast.levels/t=all@" + evidenceTestObservationID + "#-"
	if got := trackReadKey(evidenceTestInput(), "fast.levels"); got != wantTrack {
		t.Fatalf("trackReadKey = %q, want %q", got, wantTrack)
	}
	wantAcoustic := "vit://mom/acoustic_package_status:l3_deep.band_energy_summary/t=all@" + evidenceTestObservationID + "#-"
	if got := acousticFeatureRef(evidenceTestInput(), "l3_deep", "band_energy_summary"); got != wantAcoustic {
		t.Fatalf("acousticFeatureRef = %q, want %q", got, wantAcoustic)
	}
	wantObservation := "vit://mom/observation:" + evidenceTestObservationID + "/t=all@" + evidenceTestObservationID + "#-"
	if got := observationRef(evidenceTestInput()); got != wantObservation {
		t.Fatalf("observationRef = %q, want %q", got, wantObservation)
	}
}

// Observation-scoped refs without their identity are not emitted: the vit://mom
// grammar cannot carry them without fabricating a snapshot segment.
func TestMomConstructorsRequireObservationIdentity(t *testing.T) {
	empty := Input{TargetRef: map[string]any{"id": "T1"}}
	if got := trackReadKey(empty, "fast.levels"); got != "" {
		t.Fatalf("trackReadKey without observation = %q, want empty", got)
	}
	if got := acousticFeatureRef(empty, "l3_deep", "band_energy_summary"); got != "" {
		t.Fatalf("acousticFeatureRef without observation = %q, want empty", got)
	}
	if got := observationRef(empty); got != "" {
		t.Fatalf("observationRef without observation = %q, want empty", got)
	}
}

// Legacy compat regression (M2 红线：旧落盘 refs 必须可解析). Old persisted
// mom refs keep their pre-M2 three-state parse behavior: the two registered
// families still translate as legacy entries, and mix.read: stays tolerant
// opaque passthrough (registry never had an entry for it).
func TestLegacyMomRefsRemainParseable(t *testing.T) {
	agentprotocol.ResetOpaqueWarnState()
	defer agentprotocol.ResetOpaqueWarnState()

	acpParsed, err := agentprotocol.ParseRef("acoustic_package_status:l3_deep.band_energy_summary")
	if err != nil {
		t.Fatalf("legacy acoustic_package_status parse error: %v", err)
	}
	if acpParsed.State != agentprotocol.RefStateLegacy {
		t.Fatalf("legacy acoustic_package_status state = %v, want legacy", acpParsed.State)
	}
	if acpParsed.Legacy.TargetKind != "acp" || acpParsed.Legacy.Slot != agentprotocol.RefSlotScope || acpParsed.Legacy.Value != "l3_deep.band_energy_summary" {
		t.Fatalf("legacy acoustic translation = %+v", acpParsed.Legacy)
	}

	obsParsed, err := agentprotocol.ParseRef("observation:" + evidenceTestObservationID)
	if err != nil {
		t.Fatalf("legacy observation parse error: %v", err)
	}
	if obsParsed.State != agentprotocol.RefStateLegacy {
		t.Fatalf("legacy observation state = %v, want legacy", obsParsed.State)
	}
	if obsParsed.Legacy.Slot != agentprotocol.RefSlotSnapshot || obsParsed.Legacy.Value != evidenceTestObservationID {
		t.Fatalf("legacy observation translation = %+v", obsParsed.Legacy)
	}

	mixParsed, err := agentprotocol.ParseRef("mix.read:track.T1.fast.levels")
	if err != nil {
		t.Fatalf("legacy mix.read parse error: %v", err)
	}
	if mixParsed.State != agentprotocol.RefStateOpaque || mixParsed.Raw != "mix.read:track.T1.fast.levels" {
		t.Fatalf("legacy mix.read = state %v raw %q, want opaque passthrough", mixParsed.State, mixParsed.Raw)
	}
}

// Reserved characters inside a data key survive via the grammar's own
// percent-escaping, and the canonical form stays closed under reformatting.
func TestMomRefEscapesReservedCharsInScopeValue(t *testing.T) {
	raw := momEvidenceRef("mix.read", "track.we:ird/id.suffix", "obs_x1")
	if raw == "" {
		t.Fatal("momEvidenceRef returned empty for a reservable key")
	}
	parsed, err := agentprotocol.ParseRef(raw)
	if err != nil || parsed.State != agentprotocol.RefStateParsed {
		t.Fatalf("ParseRef(%q) = %v, %v", raw, parsed.State, err)
	}
	if parsed.Ref.ScopeValue != "track.we:ird/id.suffix" {
		t.Fatalf("scope_value = %q, want original key preserved", parsed.Ref.ScopeValue)
	}
	if reformatted, err := agentprotocol.FormatRef(*parsed.Ref); err != nil || reformatted != raw {
		t.Fatalf("canonical round trip broke %q -> %q (err %v)", raw, reformatted, err)
	}
}

// frequencyProjectCutRef's observation fallback carries the identity in the
// vit://mom form; the no-identity-at-all sentinel stays the legacy literal
// ("unbound" is not an observation id, so no snapshot can be fabricated).
func TestFrequencyProjectCutRefObservationFallback(t *testing.T) {
	ref, weak := frequencyProjectCutRef(Input{ObservationID: "obs_x2"})
	if !weak {
		t.Fatalf("observation fallback must report weak authority")
	}
	parsed, err := agentprotocol.ParseRef(ref)
	if err != nil || parsed.State != agentprotocol.RefStateParsed {
		t.Fatalf("ParseRef(%q) = %v, %v", ref, parsed.State, err)
	}
	if parsed.Ref.ScopeKind != "observation" || parsed.Ref.Snapshot != "obs_x2" {
		t.Fatalf("fallback ref segments = %+v", parsed.Ref)
	}

	sentinel, weak := frequencyProjectCutRef(Input{})
	if !weak || sentinel != "observation:unbound" {
		t.Fatalf("unbound sentinel = (%q, %v), want legacy sentinel", sentinel, weak)
	}
}

// End-to-end: a built projection section carries only parsed vit:// refs from
// the migrated families (the out-of-scope mix.derive: literal is not
// vit-prefixed and stays untouched by this assertion).
func TestProjectMixProfileEvidenceRefsParseAsVitMom(t *testing.T) {
	profile := buildProjectMixProfile(Input{ObservationID: evidenceTestObservationID})
	if len(profile.EvidenceRefs) == 0 {
		t.Fatal("project mix profile emitted no evidence refs")
	}
	sawMixRead := false
	for _, ref := range profile.EvidenceRefs {
		if !strings.HasPrefix(ref, agentprotocol.RefSchemePrefix) {
			continue
		}
		parsed, err := agentprotocol.ParseRef(ref)
		if err != nil {
			t.Fatalf("ParseRef(%q) error: %v", ref, err)
		}
		if parsed.State != agentprotocol.RefStateParsed {
			t.Fatalf("evidence ref %q state = %v, want parsed", ref, parsed.State)
		}
		if parsed.Ref.Kind != "mom" {
			t.Fatalf("evidence ref %q kind = %q, want mom", ref, parsed.Ref.Kind)
		}
		if strings.HasPrefix(ref, "vit://mom/mix.read:project.tracks.summary/") {
			sawMixRead = true
		}
	}
	if !sawMixRead {
		t.Fatalf("missing migrated mix.read project ref in %#v", profile.EvidenceRefs)
	}
}
