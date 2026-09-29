package harness

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/rlm"
)

func injectReadyRender(h *Harness, jobID string) {
	h.IngestKernelTelemetry(map[string]any{
		"topic": "render", "subtopic": "render_done", "job_id": jobID, "status": "ok", "file_path": "/tmp/render.wav",
	})
}

func TestBindRenderProfileRequiresKnownReadyRender(t *testing.T) {
	h := NewWithSender(nil, nil, nil)

	_, err := h.BindRenderProfile("job-unknown", "builtin:apple_music")
	if err == nil || !strings.Contains(err.Error(), "unknown render") {
		t.Fatalf("unknown render must be rejected, got err=%v", err)
	}

	h.IngestKernelTelemetry(map[string]any{
		"topic": "render", "subtopic": "render_failed", "job_id": "job-failed", "status": "error", "message": "render failed",
	})
	_, err = h.BindRenderProfile("job-failed", "builtin:apple_music")
	if err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("failed render must be rejected as not ready, got err=%v", err)
	}

	injectReadyRender(h, "job-ready")
	if _, err := h.BindRenderProfile("job-ready", "builtin:apple_music"); err != nil {
		t.Fatalf("ready render binding rejected: %v", err)
	}
}

func TestBindRenderProfileRejectsUnknownDeliveryProfile(t *testing.T) {
	h := NewWithSender(nil, nil, nil)
	injectReadyRender(h, "job-ready")

	_, err := h.BindRenderProfile("job-ready", "builtin:nope")
	if err == nil || !strings.Contains(err.Error(), "unknown delivery profile") {
		t.Fatalf("unknown delivery profile must be rejected, got err=%v", err)
	}

	_, err = h.BindRenderProfile("job-ready", "")
	if err == nil || !strings.Contains(err.Error(), "unknown delivery profile") {
		t.Fatalf("empty profile_id must be rejected fail-closed, got err=%v", err)
	}
}

func TestBindRenderProfileEnforcesOneRenderOneProfile(t *testing.T) {
	h := NewWithSender(nil, nil, nil)
	injectReadyRender(h, "job-one")

	if _, err := h.BindRenderProfile("job-one", "builtin:apple_music"); err != nil {
		t.Fatalf("initial bind failed: %v", err)
	}
	if _, err := h.BindRenderProfile("job-one", "builtin:apple_music"); err != nil {
		t.Fatalf("idempotent rebind must pass, got %v", err)
	}
	_, err := h.BindRenderProfile("job-one", "builtin:spotify")
	if err == nil || !strings.Contains(err.Error(), "one render one profile") {
		t.Fatalf("rebinding to a different profile must be rejected, got err=%v", err)
	}
}

func TestListRenderProfileBindingsDisclosesTargetBandSemantics(t *testing.T) {
	h := NewWithSender(nil, nil, nil)
	injectReadyRender(h, "job-list")
	if _, err := h.BindRenderProfile("job-list", "builtin:gy_282_2014"); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	result, err := h.ListRenderProfileBindings()
	if err != nil {
		t.Fatal(err)
	}
	bindings, ok := result["bindings"].([]rlm.RenderBindingDisclosure)
	if !ok || len(bindings) != 1 {
		t.Fatalf("bindings disclosure missing: %#v", result["bindings"])
	}
	row := bindings[0]
	if row.RenderID != "job-list" || row.ProfileID != "builtin:gy_282_2014" || row.Status != rlm.RenderBindingStatusBound {
		t.Fatalf("disclosure row: %+v", row)
	}
	if row.Profile == nil || row.Profile.TruePeakMaxDBTP != -2 {
		t.Fatalf("gy profile vector missing: %+v", row.Profile)
	}
	if !strings.Contains(strings.ToLower(row.AssertionSemantics), "target band") {
		t.Fatalf("assertion semantics must state target band semantics, got %q", row.AssertionSemantics)
	}
	available, ok := result["available_profiles"].([]rlm.DeliveryProfile)
	if !ok || len(available) != len(rlm.BuiltinDeliveryProfiles()) {
		t.Fatalf("available built-in profiles missing: %#v", result["available_profiles"])
	}
}

func TestRenderProfileBindingsSnapshotRestoreRoundTrip(t *testing.T) {
	h := NewWithSender(nil, nil, nil)
	injectReadyRender(h, "job-rt-1")
	injectReadyRender(h, "job-rt-2")
	if _, err := h.BindRenderProfile("job-rt-1", "builtin:apple_music"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.BindRenderProfile("job-rt-2", "builtin:ebu_r128"); err != nil {
		t.Fatal(err)
	}
	snapshot := h.RenderProfileBindingsSnapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot len=%d want 2", len(snapshot))
	}

	restored := NewWithSender(nil, nil, nil)
	if dropped := restored.RestoreRenderProfileBindings(snapshot); dropped != 0 {
		t.Fatalf("restore dropped=%d want 0", dropped)
	}
	if again := restored.RenderProfileBindingsSnapshot(); len(again) != 2 {
		t.Fatalf("restored snapshot len=%d want 2", len(again))
	}
}

func TestRestoreRenderProfileBindingsDropsInvalidEntries(t *testing.T) {
	valid, err := rlm.NewRenderProfileBinding("job-valid", "builtin:spotify", "")
	if err != nil {
		t.Fatal(err)
	}
	ghost := rlm.RenderProfileBinding{
		SchemaVersion: rlm.RenderBindingSchemaVersion,
		RenderID:      "job-ghost",
		ProfileID:     "builtin:retired",
	}
	h := NewWithSender(nil, nil, nil)
	dropped := h.RestoreRenderProfileBindings([]rlm.RenderProfileBinding{valid, ghost})
	if dropped != 1 {
		t.Fatalf("dropped=%d want 1", dropped)
	}
	remaining := h.RenderProfileBindingsSnapshot()
	if len(remaining) != 1 || remaining[0].RenderID != "job-valid" {
		t.Fatalf("remaining bindings: %+v", remaining)
	}
}

func TestInvokeLocalDispatchesRenderProfileTools(t *testing.T) {
	h := NewWithSender(nil, nil, nil)
	cmd, spec, err := h.resolveToolCall("render.profile.bind", map[string]any{"render_id": "job-x", "profile_id": "builtin:apple_music"})
	if err != nil {
		t.Fatalf("resolveToolCall: %v", err)
	}
	if spec.CommandName != "render_profile_bind" {
		t.Fatalf("resolved command=%q", spec.CommandName)
	}
	if _, handled := h.invokeLocal(context.Background(), spec, cmd, nil, ""); !handled {
		t.Fatal("render_profile_bind must be handled locally")
	}

	cmd, spec, err = h.resolveToolCall("render.profile.list", map[string]any{})
	if err != nil {
		t.Fatalf("resolveToolCall list: %v", err)
	}
	if spec.CommandName != "render_profile_list" {
		t.Fatalf("resolved list command=%q", spec.CommandName)
	}
	if _, handled := h.invokeLocal(context.Background(), spec, cmd, nil, ""); !handled {
		t.Fatal("render_profile_list must be handled locally")
	}
}
