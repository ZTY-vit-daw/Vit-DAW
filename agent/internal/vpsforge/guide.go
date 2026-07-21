package vpsforge

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func writeAgentGuide(root string, manifest Manifest) error {
	guide := `# Vit VPS Forge Workspace

This directory is an isolated authoring workspace for observing a plug-in surface and preparing a minimal VPS file.

## Authority boundary

- Host enumeration is observed evidence, not semantic authority.
- Do not mutate the canonical Vit project or install a runtime profile from this workspace.
- Use vpsforge draft to create a user-reviewable <plugin>.vps.json file.
- Use vpsforge verify for write, fresh readback, display comparison, rollback, and verified stamping.
- Runtime execution uses the governed B4 plug-in control path; raw parameter tools are not an authoring shortcut.

## Reproducible workflow

1. Initialize the workspace and capture the real plug-in surface with probe or ingest-surface.
2. Review parameter labels, display text, units, bounds, and the installation fingerprint.
3. Run vpsforge draft against surface_snapshot.json and edit only semantic slot names and intended bindings.
4. Run vpsforge verify with the native worker. Verification must restore every written value and fail closed on mismatch.
5. Place only the verified VPS file in the user VPS directory. Do not install staging workspaces or generated evidence bundles.

## Agent interoperability

Codex and other authoring agents should use the JSON artifacts and loopback HTTP protocol in agent_protocol.json. Wrappers must preserve the same B4 and verification boundaries.
`
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(guide), 0o600); err != nil {
		return err
	}
	protocol := map[string]any{
		"schema_version": "vit.vpsforge.authoring.v2",
		"workspace_id":   manifest.WorkspaceID,
		"transport":      "loopback_http_or_cli",
		"artifacts":      manifest.Artifacts,
		"commands": []map[string]any{
			{"name": "workspace.status", "http": "GET /v1/workspace", "cli": "vpsforge status -workspace <path>", "mutates": false},
			{"name": "surface.ingest", "http": "POST /v1/surface", "cli": "vpsforge ingest-surface -workspace <path> -input <json>", "authority": "observed_only"},
			{"name": "host.probe", "http": "POST /v1/probe", "cli": "vpsforge probe -workspace <path> -host <url>", "authority": "observed_only"},
			{"name": "fxm.measure", "http": "POST /v1/fxm", "cli": "vpsforge measure -workspace <path> -input <json>", "authority": "derived_observation_only"},
			{"name": "vps.draft", "cli": "vpsforge draft -surface <surface_snapshot.json> -output <plugin.vps.json>", "authority": "draft_only"},
			{"name": "vps.verify", "cli": "vpsforge verify -file <plugin.vps.json> -worker <worker.exe>", "authority": "verified_write_readback_rollback"},
			{"name": "workspace.validate", "cli": "vpsforge validate -workspace <path>", "mutates": false},
		},
		"forbidden": []string{"project_runtime_write", "raw_parameter_tool_exposure", "automatic_semantic_authority", "staging_runtime_install"},
	}
	data, err := json.MarshalIndent(protocol, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "agent_protocol.json"), append(data, '\n'), 0o600)
}
