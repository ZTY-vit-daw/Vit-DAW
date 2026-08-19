package processorintent

import "testing"

func TestDecodeResolvedIntentRequiresModelFields(t *testing.T) {
	raw := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"de_esser","intent":"reduce_sibilance","required_coverage":["threshold","frequency_focus","range"],"scope":"current_track","control_mode":"semantic_loop","confidence":0.94,"evidence_refs":["obs-1"]}`
	intent, err := Decode(raw)
	if err != nil || intent.Family != FamilyDeEsser || len(intent.RequiredCoverage) != 3 {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
}

func TestDecodeUnresolvedRequiresStructuredRejection(t *testing.T) {
	raw := `{"schema_version":"semantic_processor_intent.v1","status":"unresolved","control_mode":"unresolved","confidence":0.2,"rejection":{"code":"family_ambiguous","reason":"evidence supports more than one family"}}`
	if _, err := Decode(raw); err != nil {
		t.Fatal(err)
	}
}

func TestIntentDoesNotFillMissingFieldsOrAcceptUnknownFields(t *testing.T) {
	missing := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"limiter","intent":"protect peaks","scope":"current_track","control_mode":"semantic_loop","confidence":0.9}`
	if _, err := Decode(missing); err == nil {
		t.Fatal("missing required coverage was silently defaulted")
	}
	unknown := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"limiter","intent":"protect peaks","required_coverage":["output_ceiling"],"scope":"current_track","control_mode":"semantic_loop","confidence":0.9,"vendor":"forbidden"}`
	if _, err := Decode(unknown); err == nil {
		t.Fatal("unknown vendor field was accepted")
	}
}

func TestIntentRejectsDuplicateCoverageAndResolvedRejection(t *testing.T) {
	duplicate := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"limiter","intent":"protect peaks","required_coverage":["output_ceiling","output_ceiling"],"scope":"current_track","control_mode":"semantic_loop","confidence":0.9}`
	if _, err := Decode(duplicate); err == nil {
		t.Fatal("duplicate coverage was accepted")
	}
	withRejection := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"limiter","intent":"protect peaks","required_coverage":["output_ceiling"],"scope":"current_track","control_mode":"semantic_loop","confidence":0.9,"rejection":{"code":"x","reason":"y"}}`
	if _, err := Decode(withRejection); err == nil {
		t.Fatal("resolved intent retained rejection")
	}
}

func TestDecodeRejectsUnknownEmbeddedIntentField(t *testing.T) {
	raw := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"limiter","intent":"protect peaks","required_coverage":["output_ceiling"],"scope":"current_track","control_mode":"semantic_loop","confidence":0.9,"unexpected":"must fail"}`
	if _, err := Decode(raw); err == nil {
		t.Fatal("unknown embedded intent field was accepted")
	}
}

func TestInspectOnlyControlModeIsBoundToBoundaryFamilies(t *testing.T) {
	valid := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"spectral_dynamics","intent":"inspect spectral behavior","required_coverage":["inspect_only"],"scope":"current_track","control_mode":"inspect_only","confidence":0.8}`
	if _, err := Decode(valid); err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"de_esser", "limiter"} {
		raw := `{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"` + family + `","intent":"inspect","required_coverage":["inspect_only"],"scope":"current_track","control_mode":"inspect_only","confidence":0.8}`
		if _, err := Decode(raw); err == nil {
			t.Fatalf("inspect_only was accepted for executable family %s", family)
		}
	}
}
