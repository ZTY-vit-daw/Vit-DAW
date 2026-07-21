package kernel

import "testing"

func TestLegacyStateFromVSPPayloadPreservesProjectWorkspaceIdentity(t *testing.T) {
	manifest := map[string]any{"schema_version": "vit_analysis_manifest.v1", "status": "ready"}
	state := legacyStateFromVSPPayload(map[string]any{
		"project": map[string]any{
			"project_path": "D:/Projects/song.vit", "project_uuid": "vitproj_song",
			"parent_project_uuid": "vitproj_parent", "analysis_manifest": manifest,
		},
		"tracks": []any{},
	})
	if state["project_path"] != "D:/Projects/song.vit" || state["project_uuid"] != "vitproj_song" || state["parent_project_uuid"] != "vitproj_parent" {
		t.Fatalf("workspace identity missing from VSP state: %+v", state)
	}
	if embedded := mapFromAny(state["analysis_manifest"]); embedded["status"] != "ready" {
		t.Fatalf("analysis manifest missing from VSP state: %+v", state)
	}
}

func TestVSPEnvelopePreservesStableExecutionIDs(t *testing.T) {
	client := &Client{}
	envelope := client.vspEnvelope("session-1", "command", "command.request", "vsp.command.request.v1", map[string]any{"command": "legacy.command"}, "action-1", "execution-1")
	if envelope["request_id"] != "action-1" || envelope["transaction_id"] != "execution-1" {
		t.Fatalf("stable execution IDs were not preserved: %#v", envelope)
	}
}

func TestBoolMapFromAnyReadsNegotiatedCASFeature(t *testing.T) {
	flags := boolMapFromAny(map[string]any{
		"command.idempotency":       true,
		"command.base_revision_cas": "true",
		"asset.range_read":          false,
	})
	if !flags["command.idempotency"] || !flags["command.base_revision_cas"] || flags["asset.range_read"] {
		t.Fatalf("feature flags = %#v", flags)
	}
}
