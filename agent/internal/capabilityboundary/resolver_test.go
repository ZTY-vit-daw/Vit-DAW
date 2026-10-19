package capabilityboundary

import (
	"reflect"
	"testing"
)

func admittedCandidate(family string, rank int, identifier string) Candidate {
	return Candidate{
		Family:        family,
		FamilyRank:    rank,
		Identifier:    identifier,
		PluginPath:    "C:/Plugins/" + identifier + ".vst3",
		PluginName:    identifier,
		SubjectKey:    "subject:" + identifier,
		AttestationID: "att_" + identifier,
		CoverageAxes:  []string{"threshold", "ratio"},
		CoverageLevel: "full",
	}
}

func loadedInstance(pluginID, family string, params ...ParamRef) InstanceRef {
	return InstanceRef{TrackID: "1017", TrackName: "guitar", PluginID: pluginID, PluginName: pluginID, Family: family, Params: params}
}

func kinds(actions []NextAction) []string {
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		out = append(out, action.Kind)
	}
	return out
}

// noWriteKinds is the closed v1 kind set. Every kind is a non-writing
// remediation option: none performs or proposes a parameter write.
var noWriteKinds = map[string]bool{
	KindAlternateProcessor: true,
	KindNativeProcessor:    true,
	KindManualGuidance:     true,
	KindOpenPluginUI:       true,
	KindCertificationEntry: true,
}

func assertNoApproximateWrite(t *testing.T, boundary Boundary) {
	t.Helper()
	if boundary.MutationPerformed {
		t.Fatalf("boundary reports mutation_performed=true")
	}
	for _, action := range boundary.NextActions {
		if !noWriteKinds[action.Kind] {
			t.Fatalf("action %q is outside the closed non-writing kind set", action.Kind)
		}
		if action.RequiresConfirmation && action.Kind != KindAlternateProcessor && action.Kind != KindCertificationEntry {
			t.Fatalf("action %q unexpectedly requires confirmation", action.Kind)
		}
	}
	for rank, action := range boundary.NextActions {
		if action.Rank != rank+1 {
			t.Fatalf("action %s rank %d out of order in position %d", action.Key, action.Rank, rank)
		}
	}
}

func TestResolveNoPCABlockAdmitsSameFamilyAlternateAndCertificationEntry(t *testing.T) {
	boundary := Resolve(Input{
		Code:      "free_state_admission_gate_failed",
		Class:     ClassFreeState,
		Family:    "broadband_compressor",
		Reason:    "pca_load_gate: missing non-forgeable exact selection authorization",
		BlockedBy: []string{"pca_admission"},
		Candidates: []Candidate{
			admittedCandidate("broadband_compressor", 0, "CompB"),
			admittedCandidate("limiter", 1, "LimA"),
			admittedCandidate("broadband_compressor", 0, "CompA"),
		},
		Instances:     []InstanceRef{loadedInstance("fx_1", "broadband_compressor")},
		Native:        &NativeOption{Family: "broadband_compressor", Label: "DAW 原生路径"},
		Certification: &CertificationEntry{Family: "broadband_compressor", Endpoint: "/agent/processor-certification/candidates?family=broadband_compressor", ConsentToken: "temporary_track_apply_readback_restore"},
	})

	got := kinds(boundary.NextActions)
	want := []string{
		KindAlternateProcessor, // CompA (same family, identifier order)
		KindAlternateProcessor, // CompB (same family)
		KindAlternateProcessor, // LimA (adjacent rank 1)
		KindNativeProcessor,
		KindManualGuidance,
		KindOpenPluginUI,
		KindCertificationEntry,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("action kinds = %v, want %v", got, want)
	}
	if boundary.NextActions[0].Target["identifier"] != "CompA" || boundary.NextActions[2].Target["identifier"] != "LimA" {
		t.Fatalf("same-family alternates must precede adjacent-family: %+v", boundary.NextActions[:3])
	}
	if boundary.NextActions[0].Evidence["attestation_id"] != "att_CompA" {
		t.Fatalf("alternate must carry attestation evidence, got %+v", boundary.NextActions[0].Evidence)
	}
	if !boundary.NextActions[0].RequiresConfirmation || !boundary.NextActions[len(boundary.NextActions)-1].RequiresConfirmation {
		t.Fatalf("alternate_processor and certification_entry must require confirmation")
	}
	assertNoApproximateWrite(t, boundary)
}

func TestResolveNoAlternatesFallsBackToNativeAndManualGuidance(t *testing.T) {
	boundary := Resolve(Input{
		Code:       "no_pca_admitted_dynamic_plugin_candidate",
		Class:      ClassDynamicControl,
		Family:     "reverb",
		Reason:     "no exact PCA-admitted reverb candidate",
		Candidates: []Candidate{admittedCandidate("static_eq", 9, "NotAdjacentEQ")},
		Instances:  []InstanceRef{loadedInstance("fx_2", "delay", ParamRef{Role: "wet", ParamID: "mix", Range: "0..1", CurrentValue: "0.35", Expected: "调高湿量"})},
		Native:     &NativeOption{Family: "reverb"},
	})

	got := kinds(boundary.NextActions)
	want := []string{KindNativeProcessor, KindManualGuidance, KindOpenPluginUI}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("action kinds = %v, want %v", got, want)
	}
	manual := boundary.NextActions[1]
	if len(manual.ManualParams) != 1 || manual.ManualParams[0].ParamID != "mix" || manual.ManualParams[0].CurrentValue != "0.35" {
		t.Fatalf("manual guidance must carry live context pack rows, got %+v", manual.ManualParams)
	}
	for _, reason := range []string{ReasonNoSameFamilyCandidate, ReasonNoAdjacentCandidate} {
		found := false
		for _, unavailable := range boundary.UnavailableReasons {
			if unavailable == reason {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing unavailable reason %q in %v", reason, boundary.UnavailableReasons)
		}
	}
	// The non-adjacent static_eq candidate must not leak into the list.
	for _, action := range boundary.NextActions {
		if action.Kind == KindAlternateProcessor {
			t.Fatalf("non-adjacent candidate leaked into alternates: %+v", action)
		}
	}
	assertNoApproximateWrite(t, boundary)
}

func TestResolveReadOnlyFamilyEmitsNoWriteActions(t *testing.T) {
	boundary := Resolve(Input{
		Code:       "free_state_admission_gate_failed",
		Class:      ClassFreeState,
		Family:     "spectral_dynamics",
		Reason:     "inspect-only family has no controller vocabulary",
		Candidates: []Candidate{admittedCandidate("spectral_dynamics", 0, "SpectralX")},
		Native:     &NativeOption{Family: "spectral_dynamics"},
		Instances:  []InstanceRef{loadedInstance("fx_3", "spectral_dynamics")},
	})

	for _, action := range boundary.NextActions {
		if action.Kind == KindAlternateProcessor || action.Kind == KindNativeProcessor {
			t.Fatalf("read-only family must not offer substitution actions: %+v", action)
		}
	}
	if len(boundary.NextActions) == 0 {
		t.Fatalf("manual guidance for the loaded instance must remain")
	}
	foundReadOnlyReason := false
	for _, reason := range boundary.UnavailableReasons {
		if reason == ReasonReadOnlyFamily {
			foundReadOnlyReason = true
		}
	}
	if !foundReadOnlyReason {
		t.Fatalf("read-only reason missing: %v", boundary.UnavailableReasons)
	}
	assertNoApproximateWrite(t, boundary)
}

func TestResolveZeroCandidatesReturnsEmptyListWithReasons(t *testing.T) {
	boundary := Resolve(Input{
		Code:   "no_pca_coverage_for_project_treatment",
		Class:  ClassDynamicControl,
		Family: "de_esser",
		Reason: "no C2 target has a locally PCA-covered exact processor candidate",
	})

	if len(boundary.NextActions) != 0 {
		t.Fatalf("zero-fact input must yield an empty next_actions, got %+v", boundary.NextActions)
	}
	if boundary.MutationPerformed {
		t.Fatalf("mutation_performed must stay false")
	}
	for _, reason := range []string{ReasonNoSameFamilyCandidate, ReasonNoAdjacentCandidate, ReasonNativeUnavailable, ReasonNoManualInstance, ReasonCertificationUnsupported} {
		found := false
		for _, unavailable := range boundary.UnavailableReasons {
			if unavailable == reason {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing unavailable reason %q in %v", reason, boundary.UnavailableReasons)
		}
	}
}

func TestResolveUnresolvedFamilyDisclosesInsteadOfFabricating(t *testing.T) {
	boundary := Resolve(Input{Code: "capability_blocked", Class: ClassFreeState, Reason: "model declared boundary without a family"})
	if len(boundary.NextActions) != 0 {
		t.Fatalf("unresolved family must not fabricate alternates, got %+v", boundary.NextActions)
	}
	found := false
	for _, reason := range boundary.UnavailableReasons {
		if reason == ReasonFamilyUnresolved {
			found = true
		}
	}
	if !found {
		t.Fatalf("family_unresolved reason missing: %v", boundary.UnavailableReasons)
	}
}

func TestResolveCapsAlternatesDeterministically(t *testing.T) {
	candidates := []Candidate{}
	for _, identifier := range []string{"E", "D", "C", "B", "A", "F", "G"} {
		candidates = append(candidates, admittedCandidate("broadband_compressor", 0, identifier))
	}
	boundary := Resolve(Input{Code: "x", Class: ClassFreeState, Family: "broadband_compressor", Reason: "r", Candidates: candidates})
	count := 0
	for _, action := range boundary.NextActions {
		if action.Kind == KindAlternateProcessor {
			count++
		}
	}
	if count != MaxAlternateActions {
		t.Fatalf("alternates capped at %d, got %d", MaxAlternateActions, count)
	}
	// The stable sort picks A..E by identifier before the cap.
	if boundary.NextActions[0].Target["identifier"] != "A" || boundary.NextActions[MaxAlternateActions-1].Target["identifier"] != "E" {
		t.Fatalf("cap must keep the first five after the stable sort, got %v then %v", boundary.NextActions[0].Target["identifier"], boundary.NextActions[MaxAlternateActions-1].Target["identifier"])
	}
	assertNoApproximateWrite(t, boundary)
}

// TestResolveIsDeterministic pins the zero-LLM property operationally: the
// resolver is a pure function, so identical inputs must render identical
// payloads on every call.
func TestResolveIsDeterministic(t *testing.T) {
	input := Input{
		Code:          "free_state_admission_gate_failed",
		Class:         ClassFreeState,
		Family:        "limiter",
		Reason:        "gate refusal",
		EvidenceRefs:  []string{"obs_1", "obs_1", "obs_2"},
		BlockedBy:     []string{"G6_target_evidence"},
		Candidates:    []Candidate{admittedCandidate("limiter", 0, "L1"), admittedCandidate("broadband_compressor", 1, "C1")},
		Instances:     []InstanceRef{loadedInstance("fx_1", "limiter")},
		Native:        &NativeOption{Family: "limiter"},
		Certification: &CertificationEntry{Family: "limiter", Endpoint: "/e", ConsentToken: "t"},
	}
	first := Resolve(input)
	second := Resolve(input)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("resolver is not deterministic:\n%+v\n%+v", first, second)
	}
	if len(first.EvidenceRefs) != 2 {
		t.Fatalf("evidence refs must dedupe, got %v", first.EvidenceRefs)
	}
	assertNoApproximateWrite(t, first)
}
