package artifacts

import "testing"

func TestListCommandCanFilterGenericInternalArtifacts(t *testing.T) {
	store := NewStore(t.TempDir())
	_, err := store.Upsert(Artifact{
		ID:     "internal_observation_a",
		Kind:   "observation_evidence",
		Source: "pluginprobe",
		Status: "ready",
		Metadata: map[string]any{
			"artifact_schema": "vit.observation_evidence.v1",
			"internal":        true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Upsert(Artifact{
		ID:     "internal_observation_b",
		Kind:   "observation_evidence",
		Source: "pluginprobe",
		Status: "ready",
		Metadata: map[string]any{
			"artifact_schema": "vit.observation_evidence.v1",
			"internal":        true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	visible, err := ListCommand(store, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(visible["artifacts"].([]Summary)); got != 0 {
		t.Fatalf("default visible artifact count = %d", got)
	}

	filtered, err := ListCommand(store, map[string]any{
		"include_internal": true,
		"kind":             "observation_evidence",
		"source":           "pluginprobe",
	})
	if err != nil {
		t.Fatal(err)
	}
	items := filtered["artifacts"].([]Summary)
	if len(items) != 2 {
		t.Fatalf("filtered internal artifacts = %+v", items)
	}
}
