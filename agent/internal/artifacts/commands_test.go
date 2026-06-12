package artifacts

import "testing"

func TestListCommandCanFilterInternalPluginLearningStages(t *testing.T) {
	store := NewStore(t.TempDir())
	_, err := store.Upsert(Artifact{
		ID:     "pl_stage_session_a_ui_reference",
		Kind:   "plugin_learning_stage",
		Source: "plugin_grabber",
		Status: "ready",
		Metadata: map[string]any{
			"artifact_schema":            "vit.plugin_learning_session.v1",
			"plugin_learning_session_id": "session_a",
			"plugin_learning_stage":      "ui_reference",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Upsert(Artifact{
		ID:     "pl_stage_session_b_ui_reference",
		Kind:   "plugin_learning_stage",
		Source: "plugin_grabber",
		Status: "ready",
		Metadata: map[string]any{
			"artifact_schema":            "vit.plugin_learning_session.v1",
			"plugin_learning_session_id": "session_b",
			"plugin_learning_stage":      "ui_reference",
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
		"include_internal":           true,
		"kind":                       "plugin_learning_stage",
		"plugin_learning_session_id": "session_a",
	})
	if err != nil {
		t.Fatal(err)
	}
	items := filtered["artifacts"].([]Summary)
	if len(items) != 1 || items[0].ID != "pl_stage_session_a_ui_reference" {
		t.Fatalf("filtered stage artifacts = %+v", items)
	}
}
