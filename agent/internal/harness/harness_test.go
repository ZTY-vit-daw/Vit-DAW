package harness

import (
	"context"
	"testing"
)

func TestInvokeConfirmCommandDoesNotNeedKernelBeforeApproval(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Command: map[string]any{"cmd": "delete_track", "track_id": "1007"},
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if resp.Status != "needs_confirmation" {
		t.Fatalf("status = %q, want needs_confirmation", resp.Status)
	}
	if resp.AgentActionID == "" || resp.Preview == "" {
		t.Fatalf("missing action id or preview: %+v", resp)
	}
}

func TestResolveRejectsUnknownCommand(t *testing.T) {
	h := New(nil, nil, nil)
	_, _, err := h.resolveCommand(InvokeRequest{
		Tool: "daw.invoke",
		Args: map[string]any{"cmd": "totally_not_registered"},
	})
	if err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestResolveToolBuildsKernelCommand(t *testing.T) {
	h := New(nil, nil, nil)
	cmd, spec, err := h.resolveCommand(InvokeRequest{
		Tool: "track.mute",
		Args: map[string]any{"track_id": "1007", "mute": true},
	})
	if err != nil {
		t.Fatalf("resolveCommand: %v", err)
	}
	if spec.CommandName != "set_mute" || cmd["cmd"] != "set_mute" || cmd["track_id"] != "1007" {
		t.Fatalf("cmd = %+v spec = %+v", cmd, spec)
	}
}
