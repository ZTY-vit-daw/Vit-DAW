package vpsforge

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func writeAgentGuide(root string, manifest Manifest) error {
	guide := `# Vit VPS Forge Workspace

This directory is an isolated, staging-only VPS authoring workspace.

## Authority boundary

- Do not write, replace or delete the canonical VPS Library.
- Do not issue or claim a Verified Credential from this workspace.
- Automatic plugin enumeration is observed evidence, not semantic authority.
- Unknown functions remain unknown until a user or conformance evidence resolves them.
- Installation into Vit is a separate explicit gate after conformance.

## Reproducible workflow

1. Read authoring_manifest.json, evidence_ledger.json and fxm_measurement_plan.json.
2. Connect a real plugin-host adapter and capture surface_snapshot.json.
3. Run preflight to capture plugin_identity.json, parameter_probe_results.json, state_roundtrip_results.json and fxm_default_baseline.json in this staging directory.
4. Register every parameter as observed; do not infer executable meaning from its label.
5. Treat candidate_task_badges.json and inferred_parameter_groups.json as agent-inferred review material only. They are never Credentials or routes.
6. Review human_evidence_gaps.json and witness_rounds.json, then ask the user to witness the smallest useful semantic slice.
7. Archive the user's raw GUI report with the vpsforge witness-record command; it is user-confirmed evidence, never semantic conformance.
8. Draft semantic capabilities, mappings, constraints and rollback requirements.
9. Record the FXM projection and its external evidence references.
10. Run write/readback, boundary, state-retention, resource-isolation, behavior and rollback conformance.
11. Present the VPS Draft/Diff and unknowns to the user.
12. Only a separate Vit installation flow may issue/update a Credential and Catalog entry.

## Agent interoperability

Codex, Claude, OpenCode, Hermes and other agents should use the JSON artifacts and the
loopback HTTP protocol described in agent_protocol.json. Agent-specific skills may wrap
this protocol, but must not duplicate or weaken the authority rules above.
`
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(guide), 0o600); err != nil {
		return err
	}
	protocol := map[string]any{
		"schema_version": "vit.vpsforge.authoring.v1",
		"workspace_id":   manifest.WorkspaceID,
		"transport":      "loopback_http_or_cli",
		"artifacts":      manifest.Artifacts,
		"commands": []map[string]any{
			{"name": "workspace.status", "http": "GET /v1/workspace", "cli": "vpsforge status -workspace <path>", "mutates": false},
			{"name": "host.snapshot", "http": "GET /v1/plugin/snapshot", "cli": "vpsforge host -listen 127.0.0.1:<port>", "authority": "observed_only"},
			{"name": "preflight.run", "cli": "vpsforge preflight -workspace <path> -host http://127.0.0.1:<port>", "authority": "observed_and_agent_inferred_only"},
			{"name": "stereo_placement.probe", "cli": "vpsforge stereo-placement-probe -workspace <path> -host http://127.0.0.1:<port>", "authority": "observed_audio_metrics_plus_concurrent_live_human_listening_pending", "constraints": "Pro-Q 3 reference experiment; staging_only; verified rollback; offline renders do not replace live witness; no Credential issuance"},
			{"name": "pro_q_3.core_eq.probe", "cli": "vpsforge pro-q-3-eq-core-probe -workspace <path> -host http://127.0.0.1:<port>", "authority": "observed_static_response_measurements_only", "constraints": "Pro-Q 3 reference experiment; every write requires fresh readback, state roundtrip and rollback; no executable semantic mapping, Credential or route"},
			{"name": "surface.ingest", "http": "POST /v1/surface", "cli": "vpsforge ingest-surface -workspace <path> -input <json>", "authority": "observed_only"},
			{"name": "host.probe", "http": "POST /v1/probe", "cli": "vpsforge probe -workspace <path> -host <url>", "authority": "observed_only"},
			{"name": "witness.record", "cli": "vpsforge witness-record -workspace <path> -round <id> -statement <raw-user-statement> [-context <prompt-context>] [-screenshot <image>]", "authority": "user_confirmed_only", "constraints": "staging_only; a text-only confirmation is allowed; no semantic conformance or Credential issuance"},
			{"name": "fxm.measure", "http": "POST /v1/fxm", "cli": "vpsforge measure -workspace <path> -input <json>", "authority": "derived_observation_only"},
			{"name": "workspace.validate", "cli": "vpsforge validate -workspace <path>", "mutates": false},
		},
		"forbidden": []string{"canonical_library_write", "credential_issue", "catalog_update", "spal_route_enable", "automatic_semantic_authority"},
	}
	data, err := json.MarshalIndent(protocol, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "agent_protocol.json"), append(data, '\n'), 0o600)
}
