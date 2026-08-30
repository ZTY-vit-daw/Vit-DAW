package mixboard

import (
	"reflect"
	"testing"
)

func TestObservationBindingExposesLineageWithoutPackages(t *testing.T) {
	obs := ObservationPacket{
		ProjectUUID: "project-1", ObservationID: "obs-1", MixSessionID: "mix-1", Status: "ready", CreatedAt: "2026-08-05T00:00:00Z",
		TargetRef: TargetRef{Kind: "track", ID: "1007"}, EvidenceRefs: []string{"evidence://one"},
		ProjectPackage: map[string]any{"project_epoch": "epoch-1", "project_revision": "42", "project_state_hash": "hash-42"},
	}
	binding := observationBinding(obs)
	project := mapValue(binding["project_binding"])
	if binding["observation_id"] != "obs-1" || project["project_uuid"] != "project-1" || project["project_revision"] != "42" {
		t.Fatalf("binding = %+v", binding)
	}
	for _, forbidden := range []string{"mix_package", "deep_package", "global_summary"} {
		if binding[forbidden] != nil {
			t.Fatalf("binding leaked %s: %+v", forbidden, binding)
		}
	}
}

func TestObservationBindingEvidenceRefsPrependBareObservationID(t *testing.T) {
	obs := ObservationPacket{
		ObservationID: "obs-1", MixSessionID: "mix-1", Status: "ready",
		EvidenceRefs: []string{"evidence://one"},
	}
	binding := observationBinding(obs)
	refs, _ := binding["evidence_refs"].([]string)
	if !reflect.DeepEqual(refs, []string{"obs-1", "evidence://one"}) {
		t.Fatalf("evidence-on refs = %#v, want [obs-1 evidence://one]", refs)
	}
}

func TestObservationBindingEvidenceRefsBareOnlyWhenEvidenceOff(t *testing.T) {
	obs := ObservationPacket{
		ObservationID: "obs-1", MixSessionID: "mix-1", Status: "ready",
		EvidenceRefs: nil,
	}
	binding := observationBinding(obs)
	refs, _ := binding["evidence_refs"].([]string)
	if !reflect.DeepEqual(refs, []string{"obs-1"}) {
		t.Fatalf("evidence-off refs = %#v, want [obs-1]", refs)
	}
}

func TestObservationBindingEvidenceRefsDeduplicatesBareObservationID(t *testing.T) {
	obs := ObservationPacket{
		ObservationID: "obs-1", MixSessionID: "mix-1", Status: "ready",
		EvidenceRefs: []string{"obs-1", "evidence://one", "obs-1"},
	}
	binding := observationBinding(obs)
	refs, _ := binding["evidence_refs"].([]string)
	if !reflect.DeepEqual(refs, []string{"obs-1", "evidence://one"}) {
		t.Fatalf("dedup refs = %#v, want [obs-1 evidence://one]", refs)
	}
}

func TestObservationBindingEvidenceRefsKeepOriginalWhenObservationIDEmpty(t *testing.T) {
	obs := ObservationPacket{
		MixSessionID: "mix-1", Status: "ready",
		EvidenceRefs: []string{"evidence://one"},
	}
	binding := observationBinding(obs)
	refs, _ := binding["evidence_refs"].([]string)
	if !reflect.DeepEqual(refs, []string{"evidence://one"}) {
		t.Fatalf("empty obsID refs = %#v, want [evidence://one]", refs)
	}
}
