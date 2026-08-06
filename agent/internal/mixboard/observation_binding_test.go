package mixboard

import "testing"

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
