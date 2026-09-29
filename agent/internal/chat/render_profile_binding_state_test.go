package chat

import (
	"encoding/json"
	"testing"
)

func bindTestRender(server *Server, t *testing.T) {
	t.Helper()
	server.harness.IngestKernelTelemetry(map[string]any{
		"topic": "render", "subtopic": "render_done", "job_id": "job-persist-1", "status": "ok", "file_path": "/tmp/persist.wav",
	})
	if _, err := server.harness.BindRenderProfile("job-persist-1", "builtin:apple_music"); err != nil {
		t.Fatalf("bind failed: %v", err)
	}
}

func TestRenderProfileBindingsPersistAcrossProjectRuntimeRestart(t *testing.T) {
	server := New(nil, nil, nil)
	bindTestRender(server, t)

	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()

	// Simulate the on-disk round trip (history.WriteAgentRuntimeState stores
	// the marshaled form; restore decodes it back).
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	decoded := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.RenderProfileBindings) != 1 {
		t.Fatalf("decoded bindings len=%d want 1", len(decoded.RenderProfileBindings))
	}
	if decoded.RenderProfileBindings[0].ProfileID != "builtin:apple_music" {
		t.Fatalf("decoded binding: %+v", decoded.RenderProfileBindings[0])
	}

	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(decoded)
	restarted.mu.Unlock()

	snapshot := restarted.harness.RenderProfileBindingsSnapshot()
	if len(snapshot) != 1 || snapshot[0].RenderID != "job-persist-1" || snapshot[0].ProfileID != "builtin:apple_music" {
		t.Fatalf("binding did not survive restart: %+v", snapshot)
	}
}

func TestRenderProfileBindingsAbsentFieldRestoresEmpty(t *testing.T) {
	// Old records carry no render_profile_bindings field: restore must leave
	// the harness with zero bindings and not fail (AGENTS.md §11 default).
	legacy := projectAgentRuntimeState{}
	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(legacy)
	restarted.mu.Unlock()
	if snapshot := restarted.harness.RenderProfileBindingsSnapshot(); len(snapshot) != 0 {
		t.Fatalf("legacy restore must yield zero bindings, got %+v", snapshot)
	}
}
